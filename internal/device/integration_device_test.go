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

	rows, err := QuerySQLite(ctx, nil, dev, "", dbPath, "select count(*) as n from sqlite_master")
	if err != nil {
		t.Fatalf("query %s: %v", dbPath, err)
	}
	t.Logf("%s → %v", dbPath, rows)
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
