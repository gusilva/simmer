package device

import "testing"

func TestFileTargetRejectsNamingNothing(t *testing.T) {
	// The zero value used to fall through to a device-root listing, so a
	// caller that forgot to say what it wanted got the whole device.
	if err := (FileTarget{}).validate(); err == nil {
		t.Error("a target naming neither an app nor a directory must be an error")
	}
	if err := Directory("relative/path").validate(); err == nil {
		t.Error("a relative directory must be an error")
	}

	for _, t2 := range []FileTarget{DeviceRoot(), AppStorage("com.acme.app"), Directory("/storage/emulated/0")} {
		if err := t2.validate(); err != nil {
			t.Errorf("%v: %v", t2, err)
		}
	}
}

func TestFileTargetDescribesItself(t *testing.T) {
	for target, want := range map[FileTarget]string{
		DeviceRoot():                   "device root",
		AppStorage("com.acme.app"):     "com.acme.app storage",
		Directory("/storage/emulated"): "/storage/emulated",
	} {
		if got := target.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}

func TestAndroidStorageRootsCoverBothAreas(t *testing.T) {
	// An app owns two directories on Android and only this layer knows it.
	roots, err := androidStorageRoots(AppStorage("com.acme.app"), "/")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/data/data/com.acme.app", "/storage/emulated/0/Android/data/com.acme.app"}
	if len(roots) != 2 || roots[0] != want[0] || roots[1] != want[1] {
		t.Errorf("roots = %v, want %v", roots, want)
	}

	if roots, _ = androidStorageRoots(Directory("/sdcard/Download"), "/"); len(roots) != 1 || roots[0] != "/sdcard/Download" {
		t.Errorf("directory roots = %v", roots)
	}
	if roots, _ = androidStorageRoots(DeviceRoot(), "/storage/emulated/0"); len(roots) != 1 || roots[0] != "/storage/emulated/0" {
		t.Errorf("device-root roots = %v, want the one a phone can actually read", roots)
	}
}
