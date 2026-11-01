package device

import "testing"

// Each case here is a bug this package shipped. The command is a value now,
// so the exact text a device would receive is asserted without a device.

func TestAndroidListArgs(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		entriesOnly bool
		want        []string
	}{
		{
			// Without the run-as, the shell user is refused in /data/data
			// and the directory silently never opened.
			name: "private storage is listed as the owning app",
			path: "/data/data/com.acme.app/databases",
			want: []string{"run-as", "com.acme.app", "ls", "-laR", "/data/data/com.acme.app/databases"},
		},
		{
			name:        "one level only, for an interactive expansion",
			path:        "/data/data/com.acme.app",
			entriesOnly: true,
			want:        []string{"run-as", "com.acme.app", "ls", "-la", "/data/data/com.acme.app"},
		},
		{
			name: "device-encrypted storage carries a user id",
			path: "/data/user_de/0/com.acme.app/files",
			want: []string{"run-as", "com.acme.app", "ls", "-laR", "/data/user_de/0/com.acme.app/files"},
		},
		{
			// run-as is refused on the FUSE path, so using it here fails.
			name: "external app storage is listed as the shell user",
			path: "/storage/emulated/0/Android/data/com.acme.app/files",
			want: []string{"ls", "-laR", "/storage/emulated/0/Android/data/com.acme.app/files"},
		},
		{
			name: "shared storage belongs to no app",
			path: "/storage/emulated/0/Download",
			want: []string{"ls", "-laR", "/storage/emulated/0/Download"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertArgs(t, androidListArgs(tc.path, tc.entriesOnly), tc.want)
		})
	}
}

func TestAndroidSQLiteCmd(t *testing.T) {
	// A query against private storage runs as the app that owns the path,
	// with the package taken from the path rather than a flag: --app and
	// the path used to disagree, and the same file answered differently
	// depending on which was given.
	got := androidSQLiteCmd("/data/data/com.acme.app/databases/rooms.db", "select 1")
	want := "run-as 'com.acme.app' sqlite3 -csv -header '/data/data/com.acme.app/databases/rooms.db' 'select 1'"
	if got != want {
		t.Errorf("private path:\n got %s\nwant %s", got, want)
	}

	got = androidSQLiteCmd("/sdcard/Download/rooms.db", "select 1")
	want = "sqlite3 -csv -header '/sdcard/Download/rooms.db' 'select 1'"
	if got != want {
		t.Errorf("shared path:\n got %s\nwant %s", got, want)
	}
}

func TestAndroidSQLiteCmdQuotesEverything(t *testing.T) {
	// Device paths carry "~~", "==", "#" and "@"; queries carry spaces,
	// quotes and semicolons. All of it has to survive the device shell.
	path := "/storage/emulated/0/Android/data/com.acme/files/a@b.com#Store.db"
	query := `select * from "t" where name = 'x'; -- done`

	got := androidSQLiteCmd(path, query)
	if !containsSub(got, "'"+path+"'") {
		t.Errorf("path was not quoted intact: %s", got)
	}
	if !containsSub(got, `'select * from "t" where name = '\''x'\''; -- done'`) {
		t.Errorf("query quoting lost the inner quotes: %s", got)
	}
}

func TestAndroidFindCmd(t *testing.T) {
	const root = "/data/data/com.acme.app"

	t.Run("a bare name uses find -iname", func(t *testing.T) {
		got := androidFindCmd(root, "rooms", 20)
		want := "run-as 'com.acme.app' find '/data/data/com.acme.app' -iname '*rooms*' 2>/dev/null | head -n 20"
		if got != want {
			t.Errorf("\n got %s\nwant %s", got, want)
		}
	})

	t.Run("a path query is filtered with grep", func(t *testing.T) {
		// find -ipath is accepted by toybox and matches nothing, so the
		// path case goes through grep instead.
		got := androidFindCmd(root, "databases/rooms", 20)
		if containsSub(got, "-ipath") {
			t.Errorf("path query must not use -ipath: %s", got)
		}
		if !containsSub(got, "| grep -i -- 'databases/rooms'") {
			t.Errorf("path query is not filtered by grep: %s", got)
		}
	})

	t.Run("grep -F is never used", func(t *testing.T) {
		// That toybox build treats -F as "match every line", which turned
		// a narrow query into the whole device.
		if containsSub(androidFindCmd(root, "a/b", 5), "grep -iF") {
			t.Error("grep -F matches everything on some devices")
		}
	})

	t.Run("regex metacharacters in a path query are escaped", func(t *testing.T) {
		got := androidFindCmd(root, "files/a.b*c", 5)
		if !containsSub(got, `'files/a\.b\*c'`) {
			t.Errorf("metacharacters not escaped: %s", got)
		}
	})

	t.Run("the pipeline is not nested in sh -c", func(t *testing.T) {
		// A second level of quoting does not survive adb: the grep pattern
		// arrived empty and every path matched.
		if containsSub(androidFindCmd(root, "a/b", 5), "sh -c") {
			t.Error("nested sh -c loses the quoting through adb")
		}
	})
}

func TestAndroidStatCmd(t *testing.T) {
	got := androidStatCmd("/data/data/com.acme.app", []string{
		"/data/data/com.acme.app/databases/rooms.db",
		"  ",
		"/data/data/com.acme.app/files/x.txt",
	})

	want := "run-as 'com.acme.app' stat -c '%s %A %n' " +
		"'/data/data/com.acme.app/databases/rooms.db' '/data/data/com.acme.app/files/x.txt' 2>/dev/null"
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}

	if androidStatCmd("/sdcard", nil) != "" {
		t.Error("no paths means no command to run")
	}
}

func TestAndroidSQLiteVersionCmd(t *testing.T) {
	if got := androidSQLiteVersionCmd(""); got != "sqlite3 --version" {
		t.Errorf("got %q", got)
	}
	if got := androidSQLiteVersionCmd("com.acme.app"); got != "run-as 'com.acme.app' sqlite3 --version" {
		t.Errorf("got %q", got)
	}
}

func TestAndroidCatCmd(t *testing.T) {
	if got := androidCatCmd("/data/data/com.acme.app/databases/rooms.db"); got != "run-as 'com.acme.app' cat '/data/data/com.acme.app/databases/rooms.db'" {
		t.Errorf("private path: got %q", got)
	}
	if got := androidCatCmd("/sdcard/x.db"); got != "cat '/sdcard/x.db'" {
		t.Errorf("shared path: got %q", got)
	}
}

func assertArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func containsSub(s, sub string) bool { return indexOf(s, sub) >= 0 }
