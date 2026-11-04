package tech.cwcloud.strong_fish_mobile

import android.content.Context
import android.content.Intent
import android.content.pm.LauncherApps
import android.os.UserManager
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {
    private val channelName = "sf/app_launcher"

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, channelName)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "launchPackageAnyProfile" -> {
                        val pkg = call.argument<String>("package")
                        if (pkg.isNullOrBlank()) {
                            result.success(false)
                        } else {
                            result.success(launchPackageAnyProfile(pkg))
                        }
                    }
                    else -> result.notImplemented()
                }
            }
    }

    /**
     * Launches an app by package name in any user profile on the device - the
     * personal one or a managed work profile - through LauncherApps, the API
     * launchers use to show and open work apps. A personal app cannot open a
     * work-profile app with an ordinary intent, but it can this way.
     *
     * Returns true when a launchable activity was found in some profile and
     * started, false otherwise (including when it is installed in none).
     */
    private fun launchPackageAnyProfile(pkg: String): Boolean {
        // Same profile first: the ordinary launch intent, which is the plain
        // and reliable way when the app is in this profile.
        try {
            val intent = packageManager.getLaunchIntentForPackage(pkg)
            if (intent != null) {
                intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                startActivity(intent)
                return true
            }
        } catch (e: Exception) {
            // Fall through to the cross-profile path.
        }

        // Other profiles (a managed work profile): LauncherApps, the API a
        // launcher uses to open a work app from the personal profile.
        try {
            val launcherApps = getSystemService(Context.LAUNCHER_APPS_SERVICE) as LauncherApps
            val userManager = getSystemService(Context.USER_SERVICE) as UserManager
            for (profile in userManager.userProfiles) {
                val activities = launcherApps.getActivityList(pkg, profile)
                if (activities.isNotEmpty()) {
                    launcherApps.startMainActivity(activities[0].componentName, profile, null, null)
                    return true
                }
            }
        } catch (e: Exception) {
            // Nothing launchable found or allowed; fall back to the URL.
        }
        return false
    }
}
