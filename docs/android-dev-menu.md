# Android dev menu not opening

Read this when "Dev Menu" does nothing, or opens recent apps, on an Android device.
Terms: `CONTEXT.md` (Dev menu trigger, App Metro port).

Physical devices send a `devMenu` message over Metro's WebSocket instead of the Menu
key (`TriggerDevMenu` in `internal/device/android_physical.go`). It only reaches an app
connected to that Metro. Check in order:

1. The app's Metro port: `adb -s <serial> shell run-as <pkg> cat shared_prefs/<pkg>_preferences.xml`
   and read `debug_http_host`.
2. Something listens on that port on the Mac, and the device reaches it:
   `adb -s <serial> reverse --list`.
3. The app is connecting: `adb -s <serial> logcat -d | grep ReactNativeJNI` shows
   `Failed to connect to localhost:<port>` when it is not.

Fix a mismatch by running Metro on the app's port (with `adb reverse`) or by setting
Bundler Location in simmer.
