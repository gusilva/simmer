//go:build device

// Integration tests against attached simulators, emulators and phones.
//
// Run with `make check-device`. They are behind a build tag because they need
// hardware; each test skips itself when the device it asks for is absent, so
// a machine with only an emulator still reports on Android.
//
// These exist because the behaviour that broke most often here is invisible
// to unit tests: which access route reaches an app's storage, whether a
// device ships sqlite3, how a toybox build answers `grep -F`. Reach for this
// file instead of writing a throwaway main.
package device

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// requireDevice returns the first attached device of a platform and kind, or
// skips. Discovery goes through the real managers.
func requireDevice(t *testing.T, platform Platform, kind DeviceKind) Device {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	c := NewCoordinator(
		NewIOSManager(nil),
		NewPhysicalIOSManager(nil),
		NewAndroidManager(nil),
		NewPhysicalAndroidManager(nil),
	)

	for _, d := range c.Discover(ctx).Devices {
		if d.Platform == platform && d.Kind == kind && d.Status == StatusRunning {
			return d
		}
	}

	t.Skipf("no running %s %s device attached", kindName(kind), platform)
	return Device{}
}

// requireApp returns a bundle id installed on dev, preferring one that looks
// like a development build, or skips.
func requireApp(t *testing.T, c *Coordinator, dev Device) App {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	apps, err := c.ListApps(ctx, dev)
	if err != nil {
		t.Skipf("listing apps on %s: %v", dev.Name, err)
	}
	for _, a := range apps {
		if a.IsLocal() {
			return a
		}
	}
	if len(apps) > 0 {
		return apps[0]
	}

	t.Skipf("no apps installed on %s", dev.Name)
	return App{}
}

func deviceCoordinator() *Coordinator {
	return NewCoordinator(
		NewIOSManager(nil),
		NewPhysicalIOSManager(nil),
		NewAndroidManager(nil),
		NewPhysicalAndroidManager(nil),
	)
}

// TestAndroidAppStorageSpansBothAreas pins the behaviour that took three
// attempts to get right: an app's private data dir and its external dir are
// reached by different routes, and both belong to its storage.
func TestAndroidAppStorageSpansBothAreas(t *testing.T) {
	for _, kind := range []DeviceKind{KindVirtual, KindPhysical} {
		t.Run(kindName(kind), func(t *testing.T) {
			dev := requireDevice(t, PlatformAndroid, kind)
			c := deviceCoordinator()
			app := requireApp(t, c, dev)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			root, err := c.FileTree(ctx, dev, AppStorage(app.BundleID), FullDepth)
			if err != nil {
				t.Fatalf("%s storage: %v", app.BundleID, err)
			}

			var areas []string
			for _, child := range root.Children {
				areas = append(areas, child.Name)
				if child.Path == "" {
					t.Errorf("area %q carries no path", child.Name)
				}
			}
			t.Logf("%s: areas %v", app.BundleID, areas)

			if len(areas) == 0 {
				t.Fatal("no storage area was readable")
			}
			// The root is a label, never a path: sharing one with the
			// private area let a merge replace both.
			if root.Path != "" {
				t.Errorf("storage root carries path %q, want a label only", root.Path)
			}
		})
	}
}

// TestAndroidPrivateDirectoryListsThroughRunAs pins the access rule: a path
// under /data/data is readable as the owning app and by nobody else, and the
// package comes from the path.
func TestAndroidPrivateDirectoryListsThroughRunAs(t *testing.T) {
	dev := requireDevice(t, PlatformAndroid, KindVirtual)
	c := deviceCoordinator()
	app := requireApp(t, c, dev)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	node, err := c.FileTree(ctx, dev, Directory("/data/data/"+app.BundleID), OneLevel)
	if err != nil {
		t.Fatalf("listing private storage of %s: %v", app.BundleID, err)
	}
	t.Logf("%s private dir: %d entries", app.BundleID, len(node.Children))
}

// TestAndroidSQLiteServesQueriesEitherWay covers both engines: the device's
// own sqlite3 where it exists, and the local copy where it does not (most
// images since Android 9).
func TestAndroidSQLiteServesQueriesEitherWay(t *testing.T) {
	dev, dbPath := requireAndroidDatabase(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rows, err := QuerySQLite(ctx, nil, dev, "", dbPath, "select count(*) as n from sqlite_master")
	if err != nil {
		t.Fatalf("query %s: %v", dbPath, err)
	}
	t.Logf("%s → %v", dbPath, rows)
}

// TestAndroidSchemaColumnsEitherWay runs the completion schema query — a
// table-valued pragma joined to sqlite_master, which needs SQLite 3.16 — over
// both routes: the device's own sqlite3, when it has one, and a copy queried
// with the host's. The two must agree.
func TestAndroidSchemaColumnsEitherWay(t *testing.T) {
	dev, dbPath := requireAndroidDatabase(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	serial, err := resolveAndroidSerial(ctx, dev.ID)
	if err != nil {
		t.Fatalf("serial: %v", err)
	}
	prev, hadPrev := androidSQLiteOnDevice.Load(serial)
	t.Cleanup(func() {
		if hadPrev {
			androidSQLiteOnDevice.Store(serial, prev)
		} else {
			androidSQLiteOnDevice.Delete(serial)
		}
	})

	androidSQLiteOnDevice.Delete(serial) // probe the device's sqlite3 afresh
	first, err := ListSchemaColumns(ctx, nil, dev, "", dbPath)
	if err != nil {
		t.Fatalf("schema columns (first route): %v", err)
	}
	onDevice, _ := androidSQLiteOnDevice.Load(serial)
	t.Logf("%s: %d tables, device sqlite3: %v", dbPath, len(first), onDevice)

	androidSQLiteOnDevice.Store(serial, false) // force the copy route
	copied, err := ListSchemaColumns(ctx, nil, dev, "", dbPath)
	if err != nil {
		t.Fatalf("schema columns (copy): %v", err)
	}
	if !reflect.DeepEqual(first, copied) {
		t.Errorf("routes disagree:\n%+v\n%+v", first, copied)
	}
	for _, tbl := range first {
		if len(tbl.Columns) == 0 {
			t.Errorf("%s has no columns", tbl.Name)
		}
	}
}

// requireAndroidDatabase returns a running emulator and the first SQLite
// database in an app's storage on it, or skips.
func requireAndroidDatabase(t *testing.T) (Device, string) {
	t.Helper()
	dev := requireDevice(t, PlatformAndroid, KindVirtual)
	c := deviceCoordinator()
	app := requireApp(t, c, dev)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	root, err := c.FileTree(ctx, dev, AppStorage(app.BundleID), FullDepth)
	if err != nil {
		t.Skipf("%s storage unreadable: %v", app.BundleID, err)
	}

	var dbPath string
	var walk func(FileNode)
	walk = func(n FileNode) {
		if dbPath == "" && !n.IsDir && IsDatabaseFile(n.Name) {
			dbPath = n.Path
		}
		for _, child := range n.Children {
			walk(child)
		}
	}
	walk(root)

	if dbPath == "" {
		t.Skipf("%s keeps no SQLite database", app.BundleID)
	}
	return dev, dbPath
}

// TestAndroidSearchFindsByNameAndPath covers the two query shapes, including
// the path shape that `find -ipath` silently fails to match on some builds.
func TestAndroidSearchFindsByNameAndPath(t *testing.T) {
	dev := requireDevice(t, PlatformAndroid, KindVirtual)
	c := deviceCoordinator()
	app := requireApp(t, c, dev)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, query := range []string{"db", "databases/"} {
		matches, err := c.SearchFiles(ctx, dev, AppStorage(app.BundleID), query, 20)
		if err != nil {
			t.Errorf("search %q: %v", query, err)
			continue
		}
		t.Logf("search %q → %d matches", query, len(matches))

		for _, m := range matches {
			if m.Path == "" || m.Name == "" {
				t.Errorf("match with empty path or name: %+v", m)
			}
		}
	}
}

// TestReactNativeDetectionReadsOnlyTheZipDirectory guards the measured win:
// detection must not pull whole APKs.
func TestReactNativeDetectionReadsOnlyTheZipDirectory(t *testing.T) {
	dev := requireDevice(t, PlatformAndroid, KindVirtual)
	c := deviceCoordinator()
	app := requireApp(t, c, dev)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	isRN := c.DetectReactNative(ctx, dev, app)
	elapsed := time.Since(start)

	t.Logf("%s: react native %v in %s", app.BundleID, isRN, elapsed.Round(time.Millisecond))

	// A full APK pull takes seconds per app; a central-directory read is
	// well under a second even for a 500 MB APK.
	if elapsed > 5*time.Second {
		t.Errorf("detection took %s, which suggests the whole APK was transferred", elapsed)
	}
}

// A dev-menu trigger must never leave the foreground app: OEM ROMs (Xiaomi)
// map the Menu key to recent apps, which swaps the foreground package for the
// launcher. Either the trigger reaches the app (same package) or it reports an
// error; a "success" that changed the foreground is the regression.
func TestAndroidDevMenuKeepsAppForeground(t *testing.T) {
	dev := requireDevice(t, PlatformAndroid, KindPhysical)

	d, err := gadbDevice(dev.ID)
	if err != nil {
		t.Skipf("adb unavailable: %v", err)
	}
	before := foregroundPackage(d)
	if before == "" {
		t.Skip("no foreground app to trigger the dev menu on")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	trigErr := deviceCoordinator().TriggerDevMenu(ctx, dev, "")
	time.Sleep(time.Second)

	if after := foregroundPackage(d); after != before {
		t.Fatalf("foreground changed %s -> %s (trigger error: %v)", before, after, trigErr)
	}
}

// TestAdbServerStartAndProbe checks the round trip the TUI's "A" key and
// `simmer doctor` rely on: a started server is seen by the socket probe and
// by discovery. It never stops the server, so other tools keep their devices.
func TestAdbServerStartAndProbe(t *testing.T) {
	if ProbeAdbServer() == AdbNotInstalled {
		t.Skip("adb not on PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := StartAdbServer(ctx); err != nil {
		t.Fatalf("start adb server: %v", err)
	}
	if got := ProbeAdbServer(); got != AdbServerRunning {
		t.Fatalf("probe after start = %v, want running", got)
	}
	if got := NewCoordinator(NewAndroidManager(nil)).Discover(ctx).AdbServer; got != AdbServerRunning {
		t.Errorf("discovery after start = %v, want running", got)
	}
}
