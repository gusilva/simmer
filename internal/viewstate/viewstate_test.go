package viewstate

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestClaim_ResumesPreviousOwnersView(t *testing.T) {
	dir := t.TempDir()
	a := Open(dir)
	if _, err := a.Claim(false); err != nil {
		t.Fatal(err)
	}
	want := View{DeviceID: "dev-1", Loaded: true, Panel: PanelApps, AppCursor: "com.x",
		DB: &DB{DeviceID: "dev-1", Path: "/d/x.db", Name: "x.db", Query: "select 1;", Table: "users"}}
	if owned, err := a.Save(want); err != nil || !owned {
		t.Fatalf("save: owned=%v err=%v", owned, err)
	}

	b := Open(dir)
	got, err := b.Claim(false)
	if err != nil {
		t.Fatal(err)
	}
	if got.DeviceID != want.DeviceID || got.AppCursor != want.AppCursor || got.DB == nil || *got.DB != *want.DB {
		t.Fatalf("resumed %+v, want %+v", got, want)
	}
}

func TestTakeover_SupersedesAndBlocksStaleSave(t *testing.T) {
	dir := t.TempDir()
	a, b := Open(dir), Open(dir)
	_, _ = a.Claim(false)
	_, _ = b.Claim(false)

	if owned, _ := a.Owned(); owned {
		t.Fatal("a still owned after b claimed")
	}
	if owned, _ := a.Save(View{DeviceID: "stale"}); owned {
		t.Fatal("superseded save reported owned")
	}
	if owned, _ := b.Save(View{DeviceID: "fresh"}); !owned {
		t.Fatal("b lost ownership")
	}

	// Taking back keeps a's current view.
	if err := a.TakeOver(View{DeviceID: "a-now"}); err != nil {
		t.Fatal(err)
	}
	if owned, _ := b.Owned(); owned {
		t.Fatal("b still owned after a took over")
	}
	c := Open(dir)
	got, _ := c.Claim(false)
	if got.DeviceID != "a-now" {
		t.Fatalf("got %q, want a-now", got.DeviceID)
	}
}

func TestClaim_Fresh(t *testing.T) {
	dir := t.TempDir()
	a := Open(dir)
	_, _ = a.Claim(false)
	_, _ = a.Save(View{DeviceID: "x"})

	b := Open(dir)
	got, err := b.Claim(true)
	if err != nil || got.DeviceID != "" {
		t.Fatalf("fresh claim got %+v err=%v", got, err)
	}
	if owned, _ := a.Owned(); owned {
		t.Fatal("fresh claim did not supersede")
	}
	got, _ = Open(dir).Claim(false)
	if got.DeviceID != "" {
		t.Fatalf("fresh claim should record an empty view, got %q", got.DeviceID)
	}
}

func TestClaim_UnreadableFileStartsFresh(t *testing.T) {
	for name, content := range map[string]string{
		"corrupt":        "{not json",
		"future version": `{"version": 99, "owner": "x", "view": {"deviceId": "d"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, stateFile), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			s := Open(dir)
			got, err := s.Claim(false)
			if err == nil {
				t.Fatal("want a warning error")
			}
			if got.DeviceID != "" {
				t.Fatalf("got %+v, want empty", got)
			}
			if owned, _ := s.Owned(); !owned {
				t.Fatal("claim over an unreadable file must still own it")
			}
		})
	}
}

func TestOwned_MissingFileStaysOwned(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	_, _ = s.Claim(false)
	_ = os.Remove(filepath.Join(dir, stateFile))
	if owned, _ := s.Owned(); !owned {
		t.Fatal("a deleted file must not supersede the owner")
	}
}

func TestClaim_ConcurrentLaunchesLeaveOneOwner(t *testing.T) {
	dir := t.TempDir()
	stores := make([]*Store, 16)
	var wg sync.WaitGroup
	for i := range stores {
		stores[i] = Open(dir)
		wg.Add(1)
		go func(s *Store) {
			defer wg.Done()
			if _, err := s.Claim(false); err != nil {
				t.Error(err)
			}
		}(stores[i])
	}
	wg.Wait()
	owners := 0
	for _, s := range stores {
		if owned, _ := s.Owned(); owned {
			owners++
		}
	}
	if owners != 1 {
		t.Fatalf("%d owners, want 1", owners)
	}
}
