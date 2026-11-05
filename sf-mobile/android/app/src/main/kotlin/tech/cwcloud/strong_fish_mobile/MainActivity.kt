package tech.cwcloud.strong_fish_mobile

import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine

class MainActivity : FlutterActivity() {
    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        // Bar-speed capture over BLE, behind the sf/bar_speed_capture channels.
        BarSpeedCapturePlugin(this, flutterEngine.dartExecutor.binaryMessenger)
    }
}
