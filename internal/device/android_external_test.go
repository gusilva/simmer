package device

import (
	"strings"
	"testing"
)

// realExternalListing is `ls -laR /storage/emulated/0/Android/data/<pkg>`
// as a device actually answers it: the listing starts deep in the tree, so
// none of the ancestors appear in it.
const realExternalListing = `/storage/emulated/0/Android/data/com.acme.app:
total 32
drwxrws--- 4 u0_a208  ext_data_rw 4096 2026-05-03 00:39 .
drwxrws--x 6 media_rw ext_data_rw 4096 2026-05-08 20:29 ..
drwxrws--- 2 u0_a208  ext_data_rw 4096 2026-05-03 00:39 cache
drwxrws--- 9 u0_a208  ext_data_rw 4096 2026-05-03 00:39 files

/storage/emulated/0/Android/data/com.acme.app/cache:
total 8
drwxrws--- 2 u0_a208 ext_data_rw 4096 2026-05-03 00:39 .

/storage/emulated/0/Android/data/com.acme.app/files:
total 12
drwxrws--- 9 u0_a208 ext_data_rw 4096 2026-05-03 00:39 .
drwxrws--- 3 u0_a208 ext_data_rw 4096 2026-05-03 00:39 g.test3@cmms-test.com

/storage/emulated/0/Android/data/com.acme.app/files/g.test3@cmms-test.com:
total 8
drwxrws--- 2 u0_a208 ext_data_rw 4096 2026-05-03 00:39 databases

/storage/emulated/0/Android/data/com.acme.app/files/g.test3@cmms-test.com/databases:
total 7176
-rw-rw---- 1 u0_a208 ext_data_rw 819200 2026-05-03 00:39 g.test3@cmms-test.com#ApplicationData.db
-rw-rw---- 1 u0_a208 ext_data_rw  32768 2026-05-03 00:40 g.test3@cmms-test.com#ApplicationData.db-shm
-rw-rw---- 1 u0_a208 ext_data_rw 524288 2026-05-03 00:39 g.test3@cmms-test.com#ApplicationData.db-wal
-rw-rw---- 1 u0_a208 ext_data_rw  12288 2026-05-03 00:39 g.test3@cmms-test.com#BlobStorage.db
`

const externalBase = "/storage/emulated/0/Android/data/com.acme.app"

// flatten returns every node path in the tree.
func flatten(n FileNode, out *[]string) {
	*out = append(*out, n.Path)
	for _, c := range n.Children {
		flatten(c, out)
	}
}

func TestParseAbsLsTreeAtBuildsMissingAncestors(t *testing.T) {
	root := parseAbsLsTreeAt(realExternalListing, externalBase)

	if root.Path != externalBase {
		t.Fatalf("root path = %q, want %q", root.Path, externalBase)
	}
	if len(root.Children) == 0 {
		t.Fatal("root has no children: the listing was orphaned, which is the bug this guards")
	}

	var paths []string
	flatten(root, &paths)
	joined := strings.Join(paths, "\n")

	// The databases sit four levels below the root, past directories that
	// the listing only ever names in its headers.
	for _, want := range []string{
		externalBase + "/files",
		externalBase + "/files/g.test3@cmms-test.com",
		externalBase + "/files/g.test3@cmms-test.com/databases",
		externalBase + "/files/g.test3@cmms-test.com/databases/g.test3@cmms-test.com#ApplicationData.db",
		externalBase + "/files/g.test3@cmms-test.com/databases/g.test3@cmms-test.com#BlobStorage.db",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("tree is missing %q", want)
		}
	}

	// "." and ".." must never become nodes.
	for _, p := range paths {
		if strings.HasSuffix(p, "/.") || strings.HasSuffix(p, "/..") {
			t.Errorf("dot entry became a node: %q", p)
		}
	}
}

func TestParseAbsLsTreeAtKeepsFileMetadata(t *testing.T) {
	root := parseAbsLsTreeAt(realExternalListing, externalBase)

	var found *FileNode
	var walk func(n *FileNode)
	walk = func(n *FileNode) {
		if n.Name == "g.test3@cmms-test.com#ApplicationData.db" {
			found = n
			return
		}
		for i := range n.Children {
			walk(&n.Children[i])
		}
	}
	walk(&root)

	if found == nil {
		t.Fatal("database not found in tree")
	}
	if found.Size != 819200 {
		t.Errorf("size = %d, want 819200", found.Size)
	}
	if found.IsDir {
		t.Error("database reported as a directory")
	}
	if found.Permissions != "-rw-rw----" {
		t.Errorf("permissions = %q, want -rw-rw----", found.Permissions)
	}
}

func TestParseAbsLsTreeAtIgnoresEntriesOutsideBase(t *testing.T) {
	listing := realExternalListing + `
/storage/emulated/0/Android/data/com.other.app:
total 8
-rw-rw---- 1 u0_a1 ext_data_rw 100 2026-05-03 00:39 secret.db
`
	root := parseAbsLsTreeAt(listing, externalBase)

	var paths []string
	flatten(root, &paths)
	for _, p := range paths {
		if strings.Contains(p, "com.other.app") {
			t.Errorf("entry outside the base leaked into the tree: %q", p)
		}
	}
}

func TestAndroidAccessRuleIsDerivedFromThePath(t *testing.T) {
	// One rule serves listing, searching and SQLite: the owning app is
	// taken from the path. The two rules that preceded it disagreed — a
	// query on a private path without --app was refused while a listing of
	// the same path worked.
	tests := []struct {
		path string
		want string
	}{
		{"/data/data/com.acme.app/databases/x.db", "run-as 'com.acme.app' "},
		{"/data/user/0/com.acme.app/databases/x.db", "run-as 'com.acme.app' "},
		{"/data/user/10/com.acme.app/files", "run-as 'com.acme.app' "}, // secondary user
		{"/data/user_de/0/com.acme.app", "run-as 'com.acme.app' "},

		{"/storage/emulated/0/Android/data/com.acme.app/files/x.db", ""},
		{"/sdcard/Download/x.db", ""},
		{"/data/data/", ""},
		{"", ""},
	}

	for _, tc := range tests {
		if got := androidRunAsForPath(tc.path); got != tc.want {
			t.Errorf("androidRunAsForPath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestAndroidListArgsUsesRunAsForPrivateStorage(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		entriesOnly bool
		want        []string
	}{
		{
			// The regression: without run-as the shell user is refused in
			// /data/data, so the directory simply never opened.
			name: "private storage is listed as the owning app",
			path: "/data/data/com.acme.app/databases",
			want: []string{"run-as", "com.acme.app", "ls", "-laR", "/data/data/com.acme.app/databases"},
		},
		{
			name:        "one level only",
			path:        "/data/data/com.acme.app/databases",
			entriesOnly: true,
			want:        []string{"run-as", "com.acme.app", "ls", "-la", "/data/data/com.acme.app/databases"},
		},
		{
			name: "device-encrypted storage",
			path: "/data/user_de/0/com.acme.app/files",
			want: []string{"run-as", "com.acme.app", "ls", "-laR", "/data/user_de/0/com.acme.app/files"},
		},
		{
			// run-as is refused on the external path, so it must not be used.
			name: "external app storage is listed as the shell user",
			path: "/storage/emulated/0/Android/data/com.acme.app/files",
			want: []string{"ls", "-laR", "/storage/emulated/0/Android/data/com.acme.app/files"},
		},
		{
			name: "shared storage",
			path: "/storage/emulated/0/Download",
			want: []string{"ls", "-laR", "/storage/emulated/0/Download"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := androidListArgs(tc.path, tc.entriesOnly)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestAndroidPackageForPath(t *testing.T) {
	for path, want := range map[string]string{
		"/data/data/com.acme.app/databases/x.db":        "com.acme.app",
		"/data/user/0/com.acme.app":                     "com.acme.app",
		"/data/user_de/0/com.acme.app/files":            "com.acme.app",
		"/storage/emulated/0/Android/data/com.acme.app": "",
		"/data/data/": "",
		"/sdcard":     "",
	} {
		got, ok := androidPackageForPath(path)
		if got != want || ok != (want != "") {
			t.Errorf("androidPackageForPath(%q) = %q,%v want %q", path, got, ok, want)
		}
	}
}
