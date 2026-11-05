package tech.cwcloud.strong_fish_mobile

import android.Manifest
import android.app.Activity
import android.bluetooth.BluetoothGatt
import android.bluetooth.BluetoothGattCallback
import android.bluetooth.BluetoothGattCharacteristic
import android.bluetooth.BluetoothGattDescriptor
import android.bluetooth.BluetoothManager
import android.bluetooth.BluetoothProfile
import android.bluetooth.le.ScanCallback
import android.bluetooth.le.ScanFilter
import android.bluetooth.le.ScanResult
import android.bluetooth.le.ScanSettings
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.ParcelUuid
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.EventChannel
import io.flutter.plugin.common.MethodChannel
import java.util.UUID

/**
 * Native bar-speed capture over BLE, behind the sf/bar_speed_capture channels
 * the Dart [BarSpeedCapture] codes against.
 *
 * PROVISIONAL — the channel wiring and GATT lifecycle are correct, but the
 * sensor specifics (service/characteristic UUIDs and the frame layout) are the
 * WitMotion-class defaults and MUST be validated against the real device, then
 * adjusted. It cannot be exercised in CI. Requires BLUETOOTH_SCAN and
 * BLUETOOTH_CONNECT (manifest + runtime grant) on Android 12+.
 *
 * WitMotion serial-BLE defaults: notify on 0xFFE4 under service 0xFFE5; data is
 * 11-byte frames led by 0x55, then a type (0x51 acceleration, 0x59 quaternion),
 * eight data bytes and a checksum. Acceleration is int16/32768 * 16 g (±16 g
 * range) in g; this converts to m/s².
 */
class BarSpeedCapturePlugin(private val activity: Activity, messenger: BinaryMessenger) {
    private val appContext = activity.applicationContext
    private val main = Handler(Looper.getMainLooper())

    private var sink: EventChannel.EventSink? = null
    private var gatt: BluetoothGatt? = null
    private var startedAtNanos = 0L
    private val quaternion = FloatArray(4) // w, x, y, z; 0 until the sensor sends one

    companion object {
        private const val CONTROL = "sf/bar_speed_capture"
        private const val SAMPLES = "sf/bar_speed_capture/samples"
        private val SERVICE_UUID: UUID = UUID.fromString("0000ffe5-0000-1000-8000-00805f9a34fb")
        private val NOTIFY_UUID: UUID = UUID.fromString("0000ffe4-0000-1000-8000-00805f9a34fb")
        private val CCCD_UUID: UUID = UUID.fromString("00002902-0000-1000-8000-00805f9b34fb")
        private const val G = 9.80665
    }

    init {
        MethodChannel(messenger, CONTROL).setMethodCallHandler { call, result ->
            when (call.method) {
                "isAvailable" -> result.success(hasBle())
                "start" -> { start(); result.success(null) }
                "stop" -> { stop(); result.success(null) }
                else -> result.notImplemented()
            }
        }
        EventChannel(messenger, SAMPLES).setStreamHandler(object : EventChannel.StreamHandler {
            override fun onListen(arguments: Any?, events: EventChannel.EventSink?) { sink = events }
            override fun onCancel(arguments: Any?) { sink = null }
        })
    }

    private fun hasBle(): Boolean =
        appContext.packageManager.hasSystemFeature(PackageManager.FEATURE_BLUETOOTH_LE)

    private fun manager(): BluetoothManager? =
        appContext.getSystemService(Context.BLUETOOTH_SERVICE) as? BluetoothManager

    // The Bluetooth permissions the scan needs: BLUETOOTH_SCAN/CONNECT on
    // Android 12+, nothing extra below it (the legacy permissions are granted at
    // install).
    private fun blePermissions(): Array<String> =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            arrayOf(Manifest.permission.BLUETOOTH_SCAN, Manifest.permission.BLUETOOTH_CONNECT)
        } else {
            emptyArray()
        }

    private fun hasPermissions(): Boolean = blePermissions().all {
        ContextCompat.checkSelfPermission(appContext, it) == PackageManager.PERMISSION_GRANTED
    }

    private fun start() {
        if (!hasPermissions()) {
            // Ask, then let the member tap record again once granted - there is
            // no need to thread the grant result back through the capture.
            ActivityCompat.requestPermissions(activity, blePermissions(), 7021)
            emitError("grant the Bluetooth permission and start again")
            return
        }
        val scanner = manager()?.adapter?.bluetoothLeScanner ?: return
        startedAtNanos = System.nanoTime()
        val filters = listOf(
            ScanFilter.Builder().setServiceUuid(ParcelUuid(SERVICE_UUID)).build(),
        )
        val settings = ScanSettings.Builder()
            .setScanMode(ScanSettings.SCAN_MODE_LOW_LATENCY)
            .build()
        try {
            scanner.startScan(filters, settings, scanCallback)
        } catch (e: SecurityException) {
            emitError("bluetooth scan permission not granted")
        }
    }

    private fun stop() {
        try {
            manager()?.adapter?.bluetoothLeScanner?.stopScan(scanCallback)
        } catch (_: Exception) {}
        try {
            gatt?.disconnect()
            gatt?.close()
        } catch (_: Exception) {}
        gatt = null
    }

    private val scanCallback = object : ScanCallback() {
        override fun onScanResult(callbackType: Int, result: ScanResult) {
            // The first matching device wins; stop scanning and connect.
            try {
                manager()?.adapter?.bluetoothLeScanner?.stopScan(this)
                gatt = result.device.connectGatt(appContext, false, gattCallback)
            } catch (e: SecurityException) {
                emitError("bluetooth connect permission not granted")
            }
        }

        override fun onScanFailed(errorCode: Int) {
            emitError("bluetooth scan failed ($errorCode)")
        }
    }

    private val gattCallback = object : BluetoothGattCallback() {
        override fun onConnectionStateChange(gatt: BluetoothGatt, status: Int, newState: Int) {
            if (newState == BluetoothProfile.STATE_CONNECTED) {
                try { gatt.discoverServices() } catch (_: SecurityException) {}
            }
        }

        override fun onServicesDiscovered(gatt: BluetoothGatt, status: Int) {
            val characteristic = gatt.getService(SERVICE_UUID)?.getCharacteristic(NOTIFY_UUID) ?: run {
                emitError("sensor has no bar-speed stream characteristic")
                return
            }
            try {
                gatt.setCharacteristicNotification(characteristic, true)
                val cccd = characteristic.getDescriptor(CCCD_UUID)
                if (cccd != null) {
                    cccd.value = BluetoothGattDescriptor.ENABLE_NOTIFICATION_VALUE
                    gatt.writeDescriptor(cccd)
                }
            } catch (_: SecurityException) {}
        }

        @Suppress("DEPRECATION") // the value comes from characteristic.value on this path
        override fun onCharacteristicChanged(gatt: BluetoothGatt, characteristic: BluetoothGattCharacteristic) {
            parse(characteristic.value ?: return)
        }
    }

    // parse walks the buffer for 11-byte 0x55 frames and emits a sample on each
    // acceleration frame, carrying the most recent quaternion.
    private fun parse(data: ByteArray) {
        var i = 0
        while (i + 11 <= data.size) {
            if (data[i].toInt() and 0xFF != 0x55) { i++; continue }
            when (data[i + 1].toInt() and 0xFF) {
                0x51 -> {
                    val ax = int16(data, i + 2) / 32768.0 * 16.0 * G
                    val ay = int16(data, i + 4) / 32768.0 * 16.0 * G
                    val az = int16(data, i + 6) / 32768.0 * 16.0 * G
                    emitSample(ax, ay, az)
                }
                0x59 -> {
                    quaternion[0] = (int16(data, i + 2) / 32768.0).toFloat()
                    quaternion[1] = (int16(data, i + 4) / 32768.0).toFloat()
                    quaternion[2] = (int16(data, i + 6) / 32768.0).toFloat()
                    quaternion[3] = (int16(data, i + 8) / 32768.0).toFloat()
                }
            }
            i += 11
        }
    }

    private fun int16(data: ByteArray, offset: Int): Int {
        val lo = data[offset].toInt() and 0xFF
        val hi = data[offset + 1].toInt()
        return (hi shl 8) or lo
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
        main.post { sink?.success(sample) }
    }

    private fun emitError(message: String) {
        main.post { sink?.error("bar_speed_capture", message, null) }
    }
}
