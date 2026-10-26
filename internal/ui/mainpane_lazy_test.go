package ui

import (
	"testing"

	"simmer/internal/device"
)

// dirNode is a directory that arrived without its entries, as every
// directory below the first listing's depth cap does.
func dirNode(path, name string) device.FileNode {
	return device.FileNode{Path: path, Name: name, IsDir: true}
}

func paneWithTree(children ...device.FileNode) MainPane {
	m := newPaneWithDevice()
	root := &device.FileNode{Path: "/storage/emulated/0", Name: "0", IsDir: true, Children: children}
	m.SetTree(root)
	return m
}

func TestExpandingUnlistedDirRequestsItsEntries(t *testing.T) {
	m := paneWithTree(dirNode("/storage/emulated/0/Android", "Android"))
	m.treeIdx = 0

	cmd := m.ActivateTreeRow()
	if cmd == nil {
		t.Fatal("expected a listing request")
	}
	msg, ok := cmd().(RequestSubtreeMsg)
	if !ok {
		t.Fatalf("got %#v, want RequestSubtreeMsg", cmd())
	}
	if msg.Path != "/storage/emulated/0/Android" {
		t.Errorf("path = %q", msg.Path)
	}
}

func TestExpandingTwiceDoesNotRequestTwice(t *testing.T) {
	m := paneWithTree(dirNode("/storage/emulated/0/Android", "Android"))
	m.treeIdx = 0

	if cmd := m.ActivateTreeRow(); cmd == nil {
		t.Fatal("first expansion should request entries")
	}
	// Collapse and expand again while the first request is in flight.
	if cmd := m.ActivateTreeRow(); cmd != nil {
		t.Error("collapsing must not request anything")
	}
	if cmd := m.ActivateTreeRow(); cmd != nil {
		t.Error("a second expansion must not duplicate an in-flight request")
	}
}

func TestMergeSubtreeAttachesEntriesAndStopsAsking(t *testing.T) {
	const path = "/storage/emulated/0/Android"
	m := paneWithTree(dirNode(path, "Android"))
	m.treeIdx = 0
	m.ActivateTreeRow()

	m.MergeSubtree(path, device.FileNode{
		Path:  path,
		Name:  "Android",
		IsDir: true,
		Children: []device.FileNode{
			dirNode(path+"/data", "data"),
			dirNode(path+"/obb", "obb"),
		},
	})

	rows := m.flattenTree()
	var names []string
	for _, r := range rows {
		names = append(names, r.node.Name)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %v, want Android plus its two entries", names)
	}
	if names[1] != "data" || names[2] != "obb" {
		t.Errorf("rows = %v, want the merged entries", names)
	}

	// Re-expanding a listed directory asks for nothing.
	m.treeIdx = 0
	m.ActivateTreeRow() // collapse
	if cmd := m.ActivateTreeRow(); cmd != nil {
		t.Error("a listed directory must not be requested again")
	}
}

func TestMergeSubtreeOfADeeplyNestedDir(t *testing.T) {
	// The node being merged into is several levels down, which is the
	// case that matters: shared-storage databases sit well below the
	// depth the first listing covers.
	const base = "/storage/emulated/0/Android/data/com.acme.app/files"
	root := &device.FileNode{
		Path: "/storage/emulated/0", Name: "0", IsDir: true,
		Children: []device.FileNode{{
			Path: "/storage/emulated/0/Android", Name: "Android", IsDir: true,
			Children: []device.FileNode{{
				Path: "/storage/emulated/0/Android/data", Name: "data", IsDir: true,
				Children: []device.FileNode{{
					Path: "/storage/emulated/0/Android/data/com.acme.app", Name: "com.acme.app", IsDir: true,
					Children: []device.FileNode{dirNode(base, "files")},
				}},
			}},
		}},
	}
	m := newPaneWithDevice()
	m.SetTree(root)

	m.MergeSubtree(base, device.FileNode{
		Path: base, Name: "files", IsDir: true,
		Children: []device.FileNode{{Path: base + "/rooms.db", Name: "rooms.db", Size: 4096}},
	})

	node := findNode(m.tree, base)
	if node == nil {
		t.Fatal("node not found after merge")
	}
	if len(node.Children) != 1 || node.Children[0].Name != "rooms.db" {
		t.Errorf("children = %+v, want rooms.db", node.Children)
	}
}

func TestSubtreeFailedAllowsRetry(t *testing.T) {
	const path = "/storage/emulated/0/Android"
	m := paneWithTree(dirNode(path, "Android"))
	m.treeIdx = 0
	m.ActivateTreeRow()

	m.SubtreeFailed(path)

	m.ActivateTreeRow() // collapse
	if cmd := m.ActivateTreeRow(); cmd == nil {
		t.Error("a failed listing should be retryable by expanding again")
	}
}

func TestMergeSubtreeForUnknownPathIsIgnored(t *testing.T) {
	m := paneWithTree(dirNode("/storage/emulated/0/Android", "Android"))

	// A late reply for a tree that has since been replaced must not panic
	// or graft itself somewhere wrong.
	m.MergeSubtree("/some/other/path", device.FileNode{Path: "/some/other/path", IsDir: true})

	if len(m.flattenTree()) != 1 {
		t.Error("tree changed after a reply for an unknown path")
	}
}

func TestSandboxAreasSurviveAMerge(t *testing.T) {
	// Regression: the sandbox root and its internal area used to share a
	// path, so one listing for that path replaced both areas and the
	// external one — where most databases live — disappeared.
	const internalPath = "/data/data/com.acme.app"
	const externalPath = "/storage/emulated/0/Android/data/com.acme.app"

	root := &device.FileNode{
		Name: "com.acme.app", IsDir: true,
		Children: []device.FileNode{
			dirNode(internalPath, "internal"),
			dirNode(externalPath, "external"),
		},
	}

	m := newPaneWithDevice()
	m.SetTree(root)

	m.MergeSubtree(internalPath, device.FileNode{
		Path: internalPath, IsDir: true,
		Children: []device.FileNode{dirNode(internalPath+"/databases", "databases")},
	})

	if len(m.tree.Children) != 2 {
		var names []string
		for _, c := range m.tree.Children {
			names = append(names, c.Name)
		}
		t.Fatalf("tree children = %v, want both storage areas", names)
	}
	if m.tree.Children[1].Name != "external" {
		t.Errorf("second area = %q, want external", m.tree.Children[1].Name)
	}
	if n := len(m.tree.Children[0].Children); n != 1 {
		t.Errorf("internal area has %d entries after the merge, want 1", n)
	}
}

func TestSearchRootsOfAnAppSandboxAreItsAreas(t *testing.T) {
	m := newPaneWithDevice()
	m.SetTree(&device.FileNode{
		Name: "com.acme.app", IsDir: true,
		Children: []device.FileNode{
			dirNode("/data/data/com.acme.app", "internal"),
			dirNode("/storage/emulated/0/Android/data/com.acme.app", "external"),
		},
	})

	roots := m.SearchRoots()
	if len(roots) != 2 || roots[0] != "/data/data/com.acme.app" || roots[1] != "/storage/emulated/0/Android/data/com.acme.app" {
		t.Errorf("roots = %v, want both areas", roots)
	}
}
