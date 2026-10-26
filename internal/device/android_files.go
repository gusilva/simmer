package device

import (
	"context"
	"fmt"
	"os/exec"
	gopath "path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/electricbubble/gadb"

	"simmer/internal/logging"
)

// AndroidRootFileSystem lists the root "/" directory of a running Android
// emulator via `adb shell ls -la /`. The result is a single-level tree.
type AndroidRootFileSystem struct {
	logger *logging.Logger
}

// NewAndroidRootFileSystem returns a FileSystem rooted at "/" on the device.
func NewAndroidRootFileSystem(logger *logging.Logger) FileSystem {
	return &AndroidRootFileSystem{logger: logger}
}

// Tree escalates to root, polls until uid=0 is confirmed, then builds a
// three-level tree. /storage/emulated/N is FUSE-restricted even for root;
// its contents are read from /data/media/N (same data, no FUSE layer) and
// injected under the correct /storage/emulated/N: header.
func (f *AndroidRootFileSystem) Tree(ctx context.Context, dev Device) (FileNode, error) {
	serial, err := resolveAndroidSerial(ctx, dev.ID)
	if err != nil {
		return FileNode{}, fmt.Errorf("find serial: %w", err)
	}

	if err := ensureAdbRoot(ctx, serial); err != nil {
		return FileNode{}, err
	}

	// [TODO]: Refactor this shell command.
	//
	// Walk three levels: / → /x → /x/y (following symlinks via [ -d ]).
	// /proc, /sys, /dev are skipped — virtual FSes that generate enormous output.
	// /storage/emulated/N is FUSE-restricted; contents are read from /data/media/N
	// (same backing store, directly accessible as root) and injected under the
	// correct /storage/emulated/N headers. find -maxdepth 6 handles deep paths
	// like Android/data/<pkg>/files/<user>/databases without hardcoded loop depth.
	const shellCmd = `ls -la / 2>/dev/null; ` +
		`for d in $(ls / 2>/dev/null); do ` +
		`case $d in proc|sys|dev) continue ;; esac; ` +
		`[ -d "/$d" ] || continue; ` +
		`echo "/$d:"; ls -la "/$d" 2>/dev/null; ` +
		`for e in $(ls "/$d" 2>/dev/null); do ` +
		`[ -d "/$d/$e" ] || continue; ` +
		`echo "/$d/$e:"; ls -la "/$d/$e" 2>/dev/null; ` +
		`done; ` +
		`done; ` +
		`for uid in $(ls /data/media 2>/dev/null); do ` +
		`echo "/storage/emulated/$uid:"; ls -la "/data/media/$uid" 2>/dev/null; ` +
		`find "/data/media/$uid" -mindepth 1 -maxdepth 6 -type d 2>/dev/null | ` +
		`while read dir; do ` +
		`rel="${dir#/data/media/$uid}"; ` +
		`echo "/storage/emulated/$uid$rel:"; ls -la "$dir" 2>/dev/null; ` +
		`done; ` +
		`done`

	var stderr strings.Builder
	shellArgs := []string{"-s", serial, "shell", shellCmd}
	cmd := exec.CommandContext(ctx, "adb", shellArgs...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	f.logger.LogExec("adb", []string{"-s", serial, "shell", "<ls tree>"}, string(out), err)
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return FileNode{}, fmt.Errorf("ls root: %s", msg)
		}
		return FileNode{}, fmt.Errorf("ls root: %w", err)
	}

	return parseAbsLsTree(string(out)), nil
}

// ensureAdbRoot restarts adbd as root and polls until `adb shell id` confirms
// uid=0. Returns an error if root cannot be confirmed within ~5 seconds.
func ensureAdbRoot(ctx context.Context, serial string) error {
	exec.CommandContext(ctx, "adb", "-s", serial, "root").Run()
	for range 5 {
		exec.CommandContext(ctx, "adb", "-s", serial, "wait-for-device").Run()
		out, err := exec.CommandContext(ctx, "adb", "-s", serial, "shell", "id").Output()
		if err == nil && strings.Contains(string(out), "uid=0") {
			return nil
		}
		time.Sleep(time.Second)
		exec.CommandContext(ctx, "adb", "-s", serial, "root").Run()
	}
	return fmt.Errorf("adb root: could not confirm uid=0 after 5 attempts")
}

// parseAbsLsTree converts the output of the two-level shell listing into a
// FileNode tree rooted at "/". Directory headers use absolute paths (/data:)
// rather than the relative paths (./cache:) used by parseLsLaR.
func parseAbsLsTree(data string) FileNode {
	type rawEntry struct {
		absPath string
		perms   string
		size    int64
		mod     time.Time
		isDir   bool
	}

	curDir := "/" // entries before the first header belong to root
	var entries []rawEntry

	for raw := range strings.SplitSeq(data, "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "total ") {
			continue
		}

		// Absolute directory header: "/data:" or "/sdcard:" etc.
		if before, ok := strings.CutSuffix(trimmed, ":"); ok {
			dir := before
			if dir == "/" || strings.HasPrefix(dir, "/") {
				curDir = dir
				continue
			}
		}

		e, ok := parseLsLine(line, "")
		if !ok {
			continue
		}
		name := e.relPath // just the filename (no curDir in parseLsLine)
		var absPath string
		if curDir == "/" {
			absPath = "/" + name
		} else {
			absPath = curDir + "/" + name
		}

		entries = append(entries, rawEntry{
			absPath: absPath,
			perms:   e.perms,
			size:    e.size,
			mod:     e.mod,
			isDir:   e.isDir,
		})
	}

	nodes := map[string]*FileNode{
		"/": {Name: "/", Path: "/", IsDir: true},
	}
	for i := range entries {
		e := &entries[i]
		nodes[e.absPath] = &FileNode{
			Name:        gopath.Base(e.absPath),
			Path:        e.absPath,
			IsDir:       e.isDir,
			Size:        e.size,
			Modified:    e.mod,
			Permissions: e.perms,
		}
	}

	childrenOf := make(map[string][]string)
	for path := range nodes {
		if path == "/" {
			continue
		}
		parent := gopath.Dir(path)
		if parent == "." || parent == "" {
			parent = "/"
		}
		childrenOf[parent] = append(childrenOf[parent], path)
	}

	var assemble func(string) FileNode
	assemble = func(path string) FileNode {
		node := *nodes[path]
		kids := childrenOf[path]
		// A symlink whose target was followed by the shell loop has children
		// but isDir=false (perms start with 'l'). Promote it so the tree can
		// expand it.
		if len(kids) > 0 {
			node.IsDir = true
		}
		sort.SliceStable(kids, func(i, j int) bool {
			a, b := nodes[kids[i]], nodes[kids[j]]
			if a.IsDir != b.IsDir {
				return a.IsDir
			}
			return a.Name < b.Name
		})
		for _, kid := range kids {
			node.Children = append(node.Children, assemble(kid))
		}
		return node
	}

	return assemble("/")
}

// AndroidFileSystem reads an app's private data directory on a running Android
// emulator via `adb shell run-as <packageID> ls -laR`.
type AndroidFileSystem struct {
	PackageID string
	MaxDepth  int
	logger    *logging.Logger
}

// NewAndroidFileSystem returns a FileSystem scoped to the given app's sandbox.
func NewAndroidFileSystem(packageID string, logger *logging.Logger) FileSystem {
	return &AndroidFileSystem{PackageID: packageID, MaxDepth: 4, logger: logger}
}

// androidExternalRoot returns an app's external, app-specific storage dir.
func androidExternalRoot(packageID string) string {
	return "/storage/emulated/0/Android/data/" + packageID
}

// Tree returns the app's sandbox: both the storage areas an Android app owns,
// under one root.
//
// An Android app keeps files in two places and they are reached differently:
//
//   - /data/data/<pkg> — private storage, readable only through `run-as`.
//   - /storage/emulated/0/Android/data/<pkg> — external app-specific storage,
//     readable only as the shell user. `run-as` is refused here (SELinux
//     denies an app the FUSE path, verified on Android 11 and 15), which is
//     why one route cannot serve both.
//
// Listing only the private area is why databases an app writes under
// files/<account>/databases were invisible. Either area may be missing or
// refused; whatever can be read is returned, and only a failure of both is
// an error.
func (f *AndroidFileSystem) Tree(ctx context.Context, dev Device) (FileNode, error) {
	serial, err := resolveAndroidSerial(ctx, dev.ID)
	if err != nil {
		return FileNode{}, fmt.Errorf("find serial: %w", err)
	}

	d, err := gadbDevice(serial)
	if err != nil {
		return FileNode{}, err
	}

	internal, internalErr := f.internalTree(d)
	external, externalErr := f.externalTree(d)

	// The root is a label for "this app's storage", not a place: its two
	// children are the real roots. It carries no path of its own, because
	// sharing one with the internal area made a listing for that path
	// replace both children and lose the external area entirely.
	root := FileNode{
		Name:  f.PackageID,
		IsDir: true,
	}
	if internalErr == nil {
		root.Children = append(root.Children, internal)
	}
	if externalErr == nil {
		root.Children = append(root.Children, external)
	}

	if len(root.Children) == 0 {
		// Neither area could be read: a release-signed app refuses run-as
		// and may have no external directory. Report both reasons — an
		// empty listing would read as "this app has no files".
		return FileNode{}, fmt.Errorf("%s storage unreadable: private: %w; external: %v",
			f.PackageID, internalErr, externalErr)
	}
	if internalErr != nil {
		f.logger.LogError("read private storage for "+f.PackageID, internalErr)
	}
	if externalErr != nil {
		f.logger.LogError("read external storage for "+f.PackageID, externalErr)
	}

	return root, nil
}

// internalTree reads /data/data/<pkg> through run-as. Stderr is captured so
// callers see the actual error (e.g. "run-as: package unknown" or
// "Permission denied") rather than just "exit status 1".
func (f *AndroidFileSystem) internalTree(d gadb.Device) (FileNode, error) {
	maxD := f.MaxDepth
	if maxD <= 0 {
		maxD = 4
	}

	out, err := d.RunShellCommand("run-as", f.PackageID, "ls", "-laR")
	f.logger.LogExec("adb shell", []string{"run-as", f.PackageID, "ls", "-laR"}, out, err)
	if err != nil {
		return FileNode{}, fmt.Errorf("run-as %s ls -laR: %w", f.PackageID, err)
	}
	if msg := strings.TrimSpace(string(out)); strings.Contains(msg, "run-as:") {
		return FileNode{}, fmt.Errorf("%s", msg)
	}

	node := parseLsLaR(string(out), f.PackageID, maxD)
	node.Name = "internal"

	return node, nil
}

// externalTree reads /storage/emulated/0/Android/data/<pkg> as the shell
// user. Depth is left to the listing itself: an app-specific directory is
// small, and capping it is what hid the databases in the first place.
func (f *AndroidFileSystem) externalTree(d gadb.Device) (FileNode, error) {
	path := androidExternalRoot(f.PackageID)

	out, err := d.RunShellCommand("ls", "-laR", path)
	f.logger.LogExec("adb shell", []string{"ls", "-laR", path}, out, err)
	if err != nil {
		return FileNode{}, fmt.Errorf("ls -laR %s: %w", path, err)
	}
	if msg := strings.TrimSpace(string(out)); msg == "" || strings.Contains(msg, "No such file") || strings.Contains(msg, "Permission denied") {
		return FileNode{}, fmt.Errorf("%s: %s", path, msg)
	}

	node := parseAbsLsTreeAt(string(out), path)
	if len(node.Children) == 0 {
		return FileNode{}, fmt.Errorf("%s: empty", path)
	}
	node.Name = "external"

	return node, nil
}

// parseAbsLsTreeAt converts `ls -laR <base>` output into a tree rooted at
// base.
//
// This is distinct from parseAbsLsTree, which assumes the listing starts at
// "/" and so has a node for every ancestor. A listing that starts deep in the
// tree has none, and every entry in it would be orphaned — which is exactly
// how an app's external databases went missing. Intermediate directories are
// created here as needed instead.
func parseAbsLsTreeAt(data, base string) FileNode {
	base = strings.TrimSuffix(base, "/")

	nodes := map[string]*FileNode{
		base: {Name: gopath.Base(base), Path: base, IsDir: true},
	}
	childrenOf := make(map[string][]string)

	// ensure creates path and every missing directory between it and base,
	// linking each to its parent exactly once.
	var ensure func(path string) *FileNode
	ensure = func(path string) *FileNode {
		if n, ok := nodes[path]; ok {
			return n
		}
		n := &FileNode{Name: gopath.Base(path), Path: path, IsDir: true}
		nodes[path] = n

		parent := gopath.Dir(path)
		if parent != path && (parent == base || strings.HasPrefix(parent, base+"/")) {
			ensure(parent)
			childrenOf[parent] = append(childrenOf[parent], path)
		}
		return n
	}

	curDir := base
	for raw := range strings.SplitSeq(data, "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "total ") {
			continue
		}
		if before, ok := strings.CutSuffix(trimmed, ":"); ok && strings.HasPrefix(before, "/") {
			curDir = strings.TrimSuffix(before, "/")
			ensure(curDir)
			continue
		}

		e, ok := parseLsLine(line, "")
		if !ok {
			continue
		}

		path := curDir + "/" + e.relPath
		if path != base && !strings.HasPrefix(path, base+"/") {
			continue
		}

		node := ensure(path)
		node.IsDir = e.isDir
		node.Size = e.size
		node.Modified = e.mod
		node.Permissions = e.perms
	}

	var assemble func(string) FileNode
	assemble = func(path string) FileNode {
		node := *nodes[path]
		kids := childrenOf[path]
		if len(kids) > 0 {
			node.IsDir = true
		}
		sort.SliceStable(kids, func(i, j int) bool {
			a, b := nodes[kids[i]], nodes[kids[j]]
			if a.IsDir != b.IsDir {
				return a.IsDir
			}
			return a.Name < b.Name
		})
		node.Children = nil
		for _, kid := range kids {
			node.Children = append(node.Children, assemble(kid))
		}
		return node
	}

	return assemble(base)
}

// ── parser ─────────────────────────────────────────────────────────────────

type lsEntry struct {
	relPath string // relative to app data root, e.g. "cache/http.db"
	perms   string
	size    int64
	mod     time.Time
	isDir   bool
}

// parseLsLaR converts `ls -laR` output (run from the app data directory) into
// a FileNode tree capped at maxDepth levels.
//
// Android toybox `ls -la` line format:
//
//	perms  links  owner  group  size  date  time[tz]  name
//	drwx------  5  u0_a1  u0_a1  100  2024-01-15  10:30  .
//
// Directory headers emitted by -R:
//
//	.:
//	./cache:
//	./cache/subdir:
func parseLsLaR(data, packageID string, maxDepth int) FileNode {
	var entries []lsEntry
	curDir := "" // relPath of the directory currently being listed

	for _, raw := range strings.Split(data, "\n") {
		line := strings.TrimRight(raw, "\r")

		// ── directory header ─────────────────────────────
		// Matches ".", "./foo", "./foo/bar", etc. followed by ":"
		if isLsDirHeader(line) {
			dir := strings.TrimSuffix(strings.TrimSpace(line), ":")
			dir = strings.TrimPrefix(dir, "./")
			if dir == "." {
				dir = ""
			}
			curDir = dir
			continue
		}

		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "total ") {
			continue
		}

		// Skip entries in directories that are too deep.
		depth := 0
		if curDir != "" {
			depth = strings.Count(curDir, "/") + 1
		}
		if depth >= maxDepth {
			continue
		}

		// ── file entry ────────────────────────────────────
		e, ok := parseLsLine(line, curDir)
		if !ok {
			continue
		}
		entries = append(entries, e)
	}

	return buildLsTree(entries, packageID)
}

// isLsDirHeader reports whether line is an ls -R directory header (".:").
func isLsDirHeader(line string) bool {
	line = strings.TrimSpace(line)
	if !strings.HasSuffix(line, ":") {
		return false
	}
	dir := strings.TrimSuffix(line, ":")
	return dir == "." || strings.HasPrefix(dir, "./")
}

// parseLsLine parses one `ls -la` entry relative to curDir.
// Returns (entry, true) on success; (zero, false) if the line should be skipped.
func parseLsLine(line, curDir string) (lsEntry, bool) {
	fields := strings.Fields(line)
	// Minimum: perms links owner group size date time name
	if len(fields) < 8 {
		return lsEntry{}, false
	}

	perms := fields[0]
	// Must start with a known type character.
	if len(perms) == 0 || !strings.ContainsRune("dlbcps-", rune(perms[0])) {
		return lsEntry{}, false
	}

	size, _ := strconv.ParseInt(fields[4], 10, 64)

	// Date is fields[5], time is fields[6].
	// Time may include seconds ("10:30:45") or sub-seconds; trim to HH:MM.
	timeStr := fields[6]
	if len(timeStr) > 5 {
		timeStr = timeStr[:5]
	}
	mod, _ := time.Parse("2006-01-02 15:04", fields[5]+" "+timeStr)

	// Name starts at fields[7]; skip a timezone token if present (+0000 / -0500).
	nameStart := 7
	if nameStart < len(fields) {
		if f7 := fields[nameStart]; len(f7) == 5 && (f7[0] == '+' || f7[0] == '-') {
			nameStart = 8
		}
	}
	if nameStart >= len(fields) {
		return lsEntry{}, false
	}
	name := strings.Join(fields[nameStart:], " ")

	// Strip symlink target ("name -> /target" → "name").
	if idx := strings.Index(name, " -> "); idx >= 0 {
		name = name[:idx]
	}

	// Skip . and ..
	if name == "." || name == ".." {
		return lsEntry{}, false
	}

	relPath := name
	if curDir != "" {
		relPath = curDir + "/" + name
	}

	return lsEntry{
		relPath: relPath,
		perms:   perms,
		size:    size,
		mod:     mod,
		isDir:   strings.HasPrefix(perms, "d"),
	}, true
}

// buildLsTree assembles a FileNode tree from the flat entry list using the
// recursive-assemble strategy so each node's Children slice is fully populated
// before being copied into its parent.
func buildLsTree(entries []lsEntry, packageID string) FileNode {
	basePath := "/data/data/" + packageID

	// Map relPath → *FileNode (without children).
	nodes := map[string]*FileNode{
		"": {
			Name:     packageID,
			Path:     basePath,
			IsDir:    true,
			Modified: time.Time{},
		},
	}
	for i := range entries {
		e := &entries[i]
		name := gopath.Base(e.relPath)
		fullPath := basePath + "/" + e.relPath
		nodes[e.relPath] = &FileNode{
			Name:        name,
			Path:        fullPath,
			IsDir:       e.isDir,
			Size:        e.size,
			Modified:    e.mod,
			Permissions: e.perms,
		}
	}

	// Build parent → children index (by relPath, no copies yet).
	childrenOf := make(map[string][]string, len(entries))
	for relPath := range nodes {
		if relPath == "" {
			continue
		}
		parent := gopath.Dir(relPath)
		if parent == "." {
			parent = ""
		}
		childrenOf[parent] = append(childrenOf[parent], relPath)
	}

	// Recursively assemble so children are fully built before being copied.
	var assemble func(string) FileNode
	assemble = func(relPath string) FileNode {
		node := *nodes[relPath]
		kids := childrenOf[relPath]
		sort.SliceStable(kids, func(i, j int) bool {
			a, b := nodes[kids[i]], nodes[kids[j]]
			if a.IsDir != b.IsDir {
				return a.IsDir
			}
			return a.Name < b.Name
		})
		for _, kid := range kids {
			node.Children = append(node.Children, assemble(kid))
		}
		return node
	}

	return assemble("")
}

// FileSystemFor returns the emulator FileSystem serving scope: the app's own
// storage for ScopeAppSandbox, a named directory for ScopePath, the device
// root otherwise.
func (m *androidManager) FileSystemFor(scope FileScope, target string) FileSystem {
	switch {
	case scope == ScopeAppSandbox && target != "":
		return NewAndroidFileSystem(target, m.logger)
	case scope == ScopePath && target != "":
		return NewAndroidPathFileSystem(target, m.logger)
	case scope == ScopeDirectory && target != "":
		return NewAndroidDirFileSystem(target, m.logger)
	default:
		return NewAndroidRootFileSystem(m.logger)
	}
}

// AndroidPathFileSystem lists any directory an adb shell user can read,
// which is how shared storage (/storage/emulated/0, /sdcard) is reached: it
// belongs to no app, so no sandbox scope can name it.
//
// EntriesOnly lists the directory without descending, for a caller that
// expands one level at a time.
type AndroidPathFileSystem struct {
	Path        string
	EntriesOnly bool
	logger      *logging.Logger
}

// NewAndroidPathFileSystem returns a FileSystem that walks everything under
// an absolute device path.
func NewAndroidPathFileSystem(path string, logger *logging.Logger) FileSystem {
	return &AndroidPathFileSystem{Path: strings.TrimSuffix(path, "/"), logger: logger}
}

// NewAndroidDirFileSystem returns a FileSystem that lists one directory's
// entries and stops.
func NewAndroidDirFileSystem(path string, logger *logging.Logger) FileSystem {
	return &AndroidPathFileSystem{Path: strings.TrimSuffix(path, "/"), EntriesOnly: true, logger: logger}
}

// androidListArgs builds the shell command that lists a path: recursive
// unless entriesOnly, and run as the owning app when the path is inside
// private storage.
//
// The shell user is refused in /data/data, so without the run-as every
// directory in an app's own data failed to open — a tree that silently
// would not expand.
func androidListArgs(path string, entriesOnly bool) []string {
	flag := "-laR"
	if entriesOnly {
		flag = "-la"
	}

	args := []string{"ls", flag, path}
	if pkg, private := androidPackageForPath(path); private {
		args = append([]string{"run-as", pkg}, args...)
	}

	return args
}

// Tree lists the directory, recursively unless EntriesOnly is set.
func (f *AndroidPathFileSystem) Tree(ctx context.Context, dev Device) (FileNode, error) {
	if f.Path == "" || !strings.HasPrefix(f.Path, "/") {
		return FileNode{}, fmt.Errorf("path must be absolute, got %q", f.Path)
	}

	serial, err := resolveAndroidSerial(ctx, dev.ID)
	if err != nil {
		return FileNode{}, fmt.Errorf("find serial: %w", err)
	}
	d, err := gadbDevice(serial)
	if err != nil {
		return FileNode{}, err
	}

	args := androidListArgs(f.Path, f.EntriesOnly)

	out, err := d.RunShellCommand(args[0], args[1:]...)
	f.logger.LogExec("adb shell", args, out, err)
	if err != nil {
		return FileNode{}, fmt.Errorf("ls %s: %w", f.Path, err)
	}
	if msg := strings.TrimSpace(string(out)); msg == "" ||
		strings.Contains(msg, "No such file") ||
		strings.Contains(msg, "Permission denied") ||
		strings.Contains(msg, "Not a directory") {
		return FileNode{}, fmt.Errorf("%s: %s", f.Path, msg)
	}

	return parseAbsLsTreeAt(string(out), f.Path), nil
}
