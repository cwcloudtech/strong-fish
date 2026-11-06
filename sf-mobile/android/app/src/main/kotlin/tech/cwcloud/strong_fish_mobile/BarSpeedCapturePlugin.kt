package tech.cwcloud.strong_fish_mobile

import android.Manifest
import android.app.Activity
import android.bluetooth.BluetoothDevice
import android.bluetooth.BluetoothGatt
import android.bluetooth.BluetoothGattCallback
import android.bluetooth.BluetoothGattCharacteristic
import android.bluetooth.BluetoothGattDescriptor
import android.bluetooth.BluetoothManager
import android.bluetooth.BluetoothProfile
import android.bluetooth.le.ScanCallback
import android.bluetooth.le.ScanResult
import android.bluetooth.le.ScanSettings
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import android.os.Handler
import android.os.Looper
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.EventChannel
import io.flutter.plugin.common.MethodChannel
import java.util.UUID

/**
 * Native bar-speed capture over BLE, behind the sf/bar_speed_capture channels
 * the Dart [BarSpeedCapture] codes against: scan for nearby sensors, connect to
 * one, and stream its motion - the same shape a brand's own app has.
 *
 * PROVISIONAL — the channel wiring and GATT lifecycle are written to be correct,
 * but the sensor specifics (the WitMotion service/characteristic UUIDs and the
 * 0x55 frame layout) MUST be validated against the real device. It cannot be
 * exercised in CI. Requires BLUETOOTH_SCAN and BLUETOOTH_CONNECT (manifest +
 * runtime grant) on Android 12+.
 *
 * The scan is deliberately unfiltered: a WitMotion-class sensor often does not
 * advertise its service UUID, so a scan filtered on it finds nothing and the
 * device "will not pair". We list everything nearby and let the member pick, the
 * way their own app does. On connect we look for the WitMotion stream: notify on
 * 0xFFE4 under service 0xFFE5, 11-byte frames led by 0x55 then a type (0x51
 * acceleration ±16 g / 32768, 0x59 quaternion).
 */
class BarSpeedCapturePlugin(private val activity: Activity, messenger: BinaryMessenger) {
    private val appContext = activity.applicationContext
    private val main = Handler(Looper.getMainLooper())

    private var devicesSink: EventChannel.EventSink? = null
    private var stateSink: EventChannel.EventSink? = null
    private var samplesSink: EventChannel.EventSink? = null

    private var gatt: BluetoothGatt? = null
    private var writeChar: BluetoothGattCharacteristic? = null
    private var scanning = false
    private var startedAtNanos = 0L
    private val quaternion = FloatArray(4) // w, x, y, z; 0 until the sensor sends one

    // The one sensor's identity while connected, so a battery reading can be
    // pushed as a fresh "connected" state, and the last battery percent (-1
    // until the first reading, then 0..100), carried on every state event.
    private var connectedId = ""
    private var connectedName: String? = null
    private var batteryPercent = -1

    // address -> {name, rssi}, insertion-ordered so the list stays stable as it
    // grows under the member's eyes.
    private val discovered = LinkedHashMap<String, Pair<String?, Int>>()

    companion object {
        private const val CONTROL = "sf/bar_speed_capture"
        private const val DEVICES = "sf/bar_speed_capture/devices"
        private const val STATE = "sf/bar_speed_capture/state"
        private const val SAMPLES = "sf/bar_speed_capture/samples"
        private val SERVICE_UUID: UUID = UUID.fromString("0000ffe5-0000-1000-8000-00805f9a34fb")
        private val NOTIFY_UUID: UUID = UUID.fromString("0000ffe4-0000-1000-8000-00805f9a34fb")
        // The write characteristic, for vendor commands - here, reading the
        // battery register.
        private val WRITE_UUID: UUID = UUID.fromString("0000ffe9-0000-1000-8000-00805f9a34fb")
        private val CCCD_UUID: UUID = UUID.fromString("00002902-0000-1000-8000-00805f9b34fb")
        private const val G = 9.80665
        private const val PERMISSION_REQUEST = 7021
        // The WitMotion "read register" command for register 0x64 (battery
        // voltage): FF AA 27 64 00, written to the write characteristic. The
        // sensor answers with a 0x55 0x71 register frame on the notify one.
        private val READ_BATTERY_REGISTER =
            byteArrayOf(0xFF.toByte(), 0xAA.toByte(), 0x27, 0x64, 0x00)
        // Battery moves slowly; a reading shortly after connecting and every
        // half-minute after is plenty, and keeps the radio quiet between.
        private const val BATTERY_INITIAL_MS = 1500L
        private const val BATTERY_POLL_MS = 30000L
    }

    init {
        MethodChannel(messenger, CONTROL).setMethodCallHandler { call, result ->
            when (call.method) {
                "isAvailable" -> result.success(hasBle())
                "startScan" -> { startScan(); result.success(null) }
                "stopScan" -> { stopScan(); result.success(null) }
                "connect" -> { connect(call.argument<String>("deviceId")); result.success(null) }
                "disconnect" -> { disconnect(); result.success(null) }
                else -> result.notImplemented()
            }
        }
        EventChannel(messenger, DEVICES).setStreamHandler(object : EventChannel.StreamHandler {
            override fun onListen(arguments: Any?, events: EventChannel.EventSink?) {
                devicesSink = events
                emitDevices()
            }
            override fun onCancel(arguments: Any?) { devicesSink = null }
        })
        EventChannel(messenger, STATE).setStreamHandler(object : EventChannel.StreamHandler {
            override fun onListen(arguments: Any?, events: EventChannel.EventSink?) { stateSink = events }
            override fun onCancel(arguments: Any?) { stateSink = null }
        })
        EventChannel(messenger, SAMPLES).setStreamHandler(object : EventChannel.StreamHandler {
            override fun onListen(arguments: Any?, events: EventChannel.EventSink?) { samplesSink = events }
            override fun onCancel(arguments: Any?) { samplesSink = null }
        })
    }

    private fun hasBle(): Boolean =
        appContext.packageManager.hasSystemFeature(PackageManager.FEATURE_BLUETOOTH_LE)

    private fun manager(): BluetoothManager? =
        appContext.getSystemService(Context.BLUETOOTH_SERVICE) as? BluetoothManager

    // BLUETOOTH_SCAN/CONNECT on Android 12+, nothing extra below it (the legacy
    // permissions are granted at install).
    private fun blePermissions(): Array<String> =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            arrayOf(Manifest.permission.BLUETOOTH_SCAN, Manifest.permission.BLUETOOTH_CONNECT)
        } else {
            emptyArray()
        }

    private fun hasPermissions(): Boolean = blePermissions().all {
        ContextCompat.checkSelfPermission(appContext, it) == PackageManager.PERMISSION_GRANTED
    }

    // --- scanning ---

    private fun startScan() {
        if (!hasPermissions()) {
            // Ask, then let the member tap scan again once granted - there is no
            // need to thread the grant result back through the capture.
            ActivityCompat.requestPermissions(activity, blePermissions(), PERMISSION_REQUEST)
            emitState("failed", error = "grant the Bluetooth permission and try again")
            return
        }
        val scanner = manager()?.adapter?.bluetoothLeScanner ?: run {
            emitState("failed", error = "bluetooth is off")
            return
        }
        if (scanning) return
        discovered.clear()
        emitDevices()
        val settings = ScanSettings.Builder()
            .setScanMode(ScanSettings.SCAN_MODE_LOW_LATENCY)
            .build()
        try {
            // No filter on purpose: see the class note. We list everything and
            // let the member pick their sensor.
            scanner.startScan(null, settings, scanCallback)
            scanning = true
            emitState("scanning")
        } catch (e: SecurityException) {
            emitState("failed", error = "bluetooth scan permission not granted")
        }
    }

    private fun stopScan() {
        if (!scanning) return
        try {
            manager()?.adapter?.bluetoothLeScanner?.stopScan(scanCallback)
        } catch (_: Exception) {}
        scanning = false
    }

    private val scanCallback = object : ScanCallback() {
        override fun onScanResult(callbackType: Int, result: ScanResult) {
            val address = result.device?.address ?: return
            val name = try { result.device.name } catch (_: SecurityException) { null }
            discovered[address] = Pair(name, result.rssi)
            emitDevices()
        }

        override fun onScanFailed(errorCode: Int) {
            scanning = false
            emitState("failed", error = "bluetooth scan failed ($errorCode)")
        }
    }

    // --- connecting ---

    private fun connect(deviceId: String?) {
        val address = deviceId?.takeIf { it.isNotBlank() } ?: return
        if (!hasPermissions()) {
            ActivityCompat.requestPermissions(activity, blePermissions(), PERMISSION_REQUEST)
            emitState("failed", error = "grant the Bluetooth permission and try again")
            return
        }
        stopScan()
        val device: BluetoothDevice = try {
            manager()?.adapter?.getRemoteDevice(address) ?: return
        } catch (e: IllegalArgumentException) {
            emitState("failed", error = "unknown device")
            return
        }
        val name = try { device.name } catch (_: SecurityException) { null }
        emitState("connecting", deviceId = address, name = name)
        try {
            closeGatt()
            gatt = device.connectGatt(appContext, false, gattCallback)
        } catch (e: SecurityException) {
            emitState("failed", deviceId = address, error = "bluetooth connect permission not granted")
        }
    }

    private fun disconnect() {
        closeGatt()
        emitState("disconnected")
    }

    private fun closeGatt() {
        main.removeCallbacks(batteryPoll)
        try {
            gatt?.disconnect()
            gatt?.close()
        } catch (_: Exception) {}
        gatt = null
        writeChar = null
        connectedId = ""
        connectedName = null
        batteryPercent = -1
    }

    // --- battery ---

    // Polls the battery register while connected: once shortly after the
    // stream is up, then every half-minute. Each run writes the read command;
    // the answer arrives as a 0x55 0x71 frame and is parsed in [parse].
    private val batteryPoll = object : Runnable {
        override fun run() {
            requestBattery()
            main.postDelayed(this, BATTERY_POLL_MS)
        }
    }

    private fun startBatteryPolling() {
        main.removeCallbacks(batteryPoll)
        main.postDelayed(batteryPoll, BATTERY_INITIAL_MS)
    }

    @Suppress("DEPRECATION") // characteristic.value + writeCharacteristic on this path
    private fun requestBattery() {
        val connection = gatt ?: return
        val characteristic = writeChar ?: return
        try {
            characteristic.value = READ_BATTERY_REGISTER
            // No response: it is a fire-and-forget request, and the sensor
            // answers on the notify characteristic, not on the write ack.
            characteristic.writeType = BluetoothGattCharacteristic.WRITE_TYPE_NO_RESPONSE
            connection.writeCharacteristic(characteristic)
        } catch (_: SecurityException) {}
    }

    private val gattCallback = object : BluetoothGattCallback() {
        override fun onConnectionStateChange(gatt: BluetoothGatt, status: Int, newState: Int) {
            val address = gatt.device?.address ?: ""
            when (newState) {
                BluetoothProfile.STATE_CONNECTED ->
                    try { gatt.discoverServices() } catch (_: SecurityException) {}
                BluetoothProfile.STATE_DISCONNECTED -> {
                    closeGatt()
                    emitState("disconnected", deviceId = address)
                }
            }
        }

        override fun onServicesDiscovered(gatt: BluetoothGatt, status: Int) {
            val address = gatt.device?.address ?: ""
            val name = try { gatt.device?.name } catch (_: SecurityException) { null }
            val characteristic = gatt.getService(SERVICE_UUID)?.getCharacteristic(NOTIFY_UUID) ?: run {
                emitState("failed", deviceId = address, name = name,
                    error = "this sensor has no bar-speed stream")
                closeGatt()
                return
            }
            try {
                gatt.setCharacteristicNotification(characteristic, true)
                val cccd = characteristic.getDescriptor(CCCD_UUID)
                if (cccd != null) {
                    cccd.value = BluetoothGattDescriptor.ENABLE_NOTIFICATION_VALUE
                    gatt.writeDescriptor(cccd)
                }
                // The same service's write characteristic carries the battery
                // read; absent on a sensor that is not a WitMotion, in which
                // case the battery simply stays unknown.
                writeChar = gatt.getService(SERVICE_UUID)?.getCharacteristic(WRITE_UUID)
                startedAtNanos = System.nanoTime()
                quaternion.fill(0f)
                connectedId = address
                connectedName = name
                batteryPercent = -1
                emitState("connected", deviceId = address, name = name)
                startBatteryPolling()
            } catch (_: SecurityException) {
                emitState("failed", deviceId = address, name = name,
                    error = "bluetooth connect permission not granted")
            }
        }

        @Suppress("DEPRECATION") // the value comes from characteristic.value on this path
        override fun onCharacteristicChanged(gatt: BluetoothGatt, characteristic: BluetoothGattCharacteristic) {
            parse(characteristic.value ?: return)
        }
    }

    // --- the WitMotion frame ---

    // parse walks the buffer for 0x55 frames. Most are 11-byte data frames (an
    // acceleration frame emits a sample, a quaternion frame updates the pose);
    // a 0x71 frame is the 20-byte answer to a register read, which is how the
    // battery comes back. Length-aware because the frame types differ in size,
    // so a 20-byte battery frame cannot be stepped over as if it were 11.
    private fun parse(data: ByteArray) {
        var i = 0
        while (i + 2 <= data.size) {
            if (data[i].toInt() and 0xFF != 0x55) { i++; continue }
            when (data[i + 1].toInt() and 0xFF) {
                0x71 -> {
                    if (i + 20 > data.size) break
                    parseBattery(data, i)
                    i += 20
                }
                0x51 -> {
                    if (i + 11 > data.size) break
                    val ax = int16(data, i + 2) / 32768.0 * 16.0 * G
                    val ay = int16(data, i + 4) / 32768.0 * 16.0 * G
                    val az = int16(data, i + 6) / 32768.0 * 16.0 * G
                    emitSample(ax, ay, az)
                    i += 11
                }
                0x59 -> {
                    if (i + 11 > data.size) break
                    quaternion[0] = (int16(data, i + 2) / 32768.0).toFloat()
                    quaternion[1] = (int16(data, i + 4) / 32768.0).toFloat()
                    quaternion[2] = (int16(data, i + 6) / 32768.0).toFloat()
                    quaternion[3] = (int16(data, i + 8) / 32768.0).toFloat()
                    i += 11
                }
                else -> {
                    // Some other 11-byte frame (angle, magnetometer…): skip it.
                    if (i + 11 > data.size) break
                    i += 11
                }
            }
        }
    }

    // parseBattery reads register 0x64 (battery voltage ×100) out of a 0x55 0x71
    // register frame and turns it into a percentage, pushing a fresh connected
    // state when it changes.
    private fun parseBattery(data: ByteArray, i: Int) {
        if (data[i + 2].toInt() and 0xFF != 0x64) return
        val voltage = int16(data, i + 4) / 100.0
        val percent = batteryPercentFromVoltage(voltage)
        if (percent != batteryPercent) {
            batteryPercent = percent
            emitState("connected", deviceId = connectedId, name = connectedName)
        }
    }

    // The WitMotion LiPo voltage-to-percent steps the sensor's own app uses, so
    // StrongFish shows the same figure the member would see there.
    private fun batteryPercentFromVoltage(v: Double): Int = when {
        v > 3.96 -> 100
        v > 3.93 -> 90
        v > 3.87 -> 75
        v > 3.82 -> 60
        v > 3.79 -> 50
        v > 3.77 -> 40
        v > 3.73 -> 30
        v > 3.70 -> 20
        v > 3.68 -> 15
        v > 3.50 -> 10
        v > 3.40 -> 5
        else -> 0
    }

    private fun int16(data: ByteArray, offset: Int): Int {
        val lo = data[offset].toInt() and 0xFF
        val hi = data[offset + 1].toInt()
        return (hi shl 8) or lo
    }

    // --- emitting (always on the main thread; the sinks require it) ---

    private fun emitDevices() {
        val list = discovered.entries.map {
            mapOf("id" to it.key, "name" to (it.value.first ?: ""), "rssi" to it.value.second)
        }
        main.post { devicesSink?.success(list) }
    }

    private fun emitState(state: String, deviceId: String = "", name: String? = null, error: String = "") {
        val event = mapOf(
            "state" to state,
            "deviceId" to deviceId,
            "name" to (name ?: ""),
            "error" to error,
            // -1 until the first reading (and once disconnected); 0..100 once the
            // sensor has answered the battery poll.
            "battery" to batteryPercent,
        )
        main.post { stateSink?.success(event) }
    }

    private fun emitSample(ax: Double, ay: Double, az: Double) {
        val t = (System.nanoTime() - startedAtNanos) / 1_000_000_000.0
        val sample = mutableMapOf<String, Any>("t" to t, "ax" to ax, "ay" to ay, "az" to az)
        if (quaternion.any { it != 0f }) {
            sample["qw"] = quaternion[0].toDouble()
            sample["qx"] = quaternion[1].toDouble()
            sample["qy"] = quaternion[2].toDouble()
            sample["qz"] = quaternion[3].toDouble()
        }
        main.post { samplesSink?.success(sample) }
    }
}
