// Package viewstate records where the user is in the TUI — the View state —
// and which open Simmer owns it — the Active instance (see CONTEXT.md and
// docs/adr/0002-single-active-instance-takeover.md).
//
// Everything lives in one machine-owned JSON file. Every read and write
// holds an flock on a sibling lock file, so a takeover and a save can never
// interleave: an instance writes only while the file still names it as the
// owner.
package viewstate

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// formatVersion is bumped whenever View changes shape incompatibly. A file
// carrying any other version is treated as unreadable and replaced.
const formatVersion = 1

const (
	stateFile = "state.json"
	lockFile  = "state.lock"
)

// ErrUnreadable marks a state file that exists but cannot be used: corrupt,
// or written by an incompatible version. Claim still succeeds over it.
var ErrUnreadable = errors.New("view state unreadable")

// Focus names the pane that holds keyboard focus.
type Focus string

const (
	FocusSidebar Focus = "sidebar"
	FocusMain    Focus = "main"
)

// Panel names the expanded main-pane panel.
type Panel string

const (
	PanelApps  Panel = "apps"
	PanelFiles Panel = "files"
)

// View is the View state: where the user is, never anything transient.
type View struct {
	// DeviceID is the UDID or adb serial of the device the user was on.
	DeviceID string `json:"deviceId,omitempty"`
	// Loaded reports whether that device was open in the main pane, rather
	// than only under the sidebar cursor.
	Loaded    bool   `json:"loaded,omitempty"`
	Focus     Focus  `json:"focus,omitempty"`
	Panel     Panel  `json:"panel,omitempty"`
	AppCursor string `json:"appCursor,omitempty"` // bundle id under the cursor
	PinnedApp string `json:"pinnedApp,omitempty"` // bundle id pinned for Files
	DB        *DB    `json:"db,omitempty"`        // nil when the DB viewer was closed
}

// DB is the database open in the DB viewer.
type DB struct {
	DeviceID  string `json:"deviceId"`
	PackageID string `json:"packageId,omitempty"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Query     string `json:"query,omitempty"`
	Table     string `json:"table,omitempty"`
}

type file struct {
	Version int    `json:"version"`
	Owner   string `json:"owner"`
	View    View   `json:"view"`
}

// Store is one instance's handle on the shared state file.
type Store struct {
	dir   string
	token string
}

// DefaultDir is where the state file lives: the XDG state directory, kept
// apart from the hand-edited config.toml.
func DefaultDir() (string, error) {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "simmer"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", "simmer"), nil
}

// Open returns a Store over dir with a fresh owner token. It touches nothing
// on disk until Claim.
func Open(dir string) *Store {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return &Store{dir: dir, token: hex.EncodeToString(b)}
}

// Claim takes over unconditionally and returns the View state to resume.
// With fresh, the resumed state is empty and recorded as such. A file that
// cannot be read yields an empty View and a non-nil error alongside a
// successful claim: the caller should warn, not fail.
func (s *Store) Claim(fresh bool) (View, error) {
	var view View
	var readErr error
	err := s.locked(func() error {
		f, rerr := s.read()
		if rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
			readErr = fmt.Errorf("%w: %v", ErrUnreadable, rerr)
		}
		if rerr == nil && !fresh {
			view = f.View
		}
		return s.write(file{Version: formatVersion, Owner: s.token, View: view})
	})
	if err != nil {
		return View{}, err
	}
	return view, readErr
}

// TakeOver makes this instance the owner again and records v — what it
// shows now — as the View state.
func (s *Store) TakeOver(v View) error {
	return s.locked(func() error {
		return s.write(file{Version: formatVersion, Owner: s.token, View: v})
	})
}

// Save records v if this instance still owns the file, and reports whether
// it did. It never overwrites another owner's state.
func (s *Store) Save(v View) (owned bool, err error) {
	err = s.locked(func() error {
		owned = s.ownedLocked()
		if !owned {
			return nil
		}
		return s.write(file{Version: formatVersion, Owner: s.token, View: v})
	})
	return owned, err
}

// Owned reports whether this instance is still the Active instance.
func (s *Store) Owned() (bool, error) {
	var owned bool
	err := s.locked(func() error {
		owned = s.ownedLocked()
		return nil
	})
	return owned, err
}

// ownedLocked: only a readable file naming another owner supersedes us. A
// missing or corrupt file is ours to rewrite on the next save.
func (s *Store) ownedLocked() bool {
	f, err := s.read()
	if err != nil {
		return true
	}
	return f.Owner == s.token
}

func (s *Store) locked(fn func() error) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	lf, err := os.OpenFile(filepath.Join(s.dir, lockFile), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open state lock: %w", err)
	}
	defer func() { _ = lf.Close() }()
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock state: %w", err)
	}
	defer func() { _ = syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) }()
	return fn()
}

func (s *Store) read() (file, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, stateFile))
	if err != nil {
		return file{}, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return file{}, fmt.Errorf("parse view state: %w", err)
	}
	if f.Version != formatVersion {
		return file{}, fmt.Errorf("view state version %d, want %d", f.Version, formatVersion)
	}
	return f, nil
}

// write replaces the file atomically, so a crash mid-write leaves the
// previous state rather than a torn one.
func (s *Store) write(f file) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("encode view state: %w", err)
	}
	tmp, err := os.CreateTemp(s.dir, stateFile+".*")
	if err != nil {
		return fmt.Errorf("write view state: %w", err)
	}
	// After a successful rename the temp name is gone; this only cleans up
	// a failed write.
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write view state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write view state: %w", err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(s.dir, stateFile)); err != nil {
		return fmt.Errorf("write view state: %w", err)
	}
	return nil
}
