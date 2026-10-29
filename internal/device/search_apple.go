package device

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielpaulus/go-ios/ios/afc"
)

// searchWalkDepth caps how deep a search descends when it has to walk the
// tree itself. Deep enough for an app's data layout, shallow enough that a
// mistyped root does not walk an entire disk.
const searchWalkDepth = 10

// SearchFiles implements FileSearcher for iOS simulators. A simulator's
// storage is a directory on this Mac, so the walk is local and needs no
// device round-trips at all.
func (m *iosManager) SearchFiles(ctx context.Context, _ Device, root, query string, limit int) ([]FileMatch, error) {
	root = expandPath(root)
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("stat %s: %w", root, err)
	}

	var matches []FileMatch

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			// An unreadable directory is normal; skip it and carry on.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if len(matches) >= limit {
			return filepath.SkipAll
		}
		if depthBelow(root, path) > searchWalkDepth {
			return fs.SkipDir
		}
		if path == root || !matchesQuery(d.Name(), path, query) {
			return nil
		}

		m := FileMatch{Name: d.Name(), Path: path, IsDir: d.IsDir()}
		if info, err := d.Info(); err == nil {
			m.Size = info.Size()
		}
		matches = append(matches, m)

		return nil
	})
	if err != nil && ctx.Err() != nil {
		return matches, ctx.Err()
	}

	return matches, nil
}

// SearchFiles implements FileSearcher for physical iOS devices. AFC has no
// search of its own, so the directories are walked over the service
// connection — bounded by depth and by the result limit.
func (m *physicalIOSManager) SearchFiles(ctx context.Context, dev Device, root, query string, limit int) ([]FileMatch, error) {
	entry, err := goIOSDevice(dev.ID)
	if err != nil {
		return nil, err
	}
	client, err := afc.New(entry)
	m.logger.LogExec("go-ios", []string{"afc", "connect"}, "", err)
	if err != nil {
		return nil, fmt.Errorf("afc: %w", err)
	}
	defer func() { _ = client.Close() }()

	var matches []FileMatch
	searchAFC(ctx, client, root, query, 0, limit, &matches)

	return matches, nil
}

// searchAFC walks the AFC tree collecting matches, depth-first.
func searchAFC(ctx context.Context, client *afc.Client, path, query string, depth, limit int, out *[]FileMatch) {
	if ctx.Err() != nil || depth > searchWalkDepth || len(*out) >= limit {
		return
	}

	names, err := client.List(path)
	if err != nil {
		return
	}

	for _, name := range names {
		if name == "." || name == ".." || len(*out) >= limit {
			continue
		}
		childPath := strings.TrimSuffix(path, "/") + "/" + name

		info, err := client.Stat(childPath)
		if err != nil {
			continue
		}

		if matchesQuery(name, childPath, query) {
			*out = append(*out, FileMatch{
				Name:  name,
				Path:  childPath,
				IsDir: info.IsDir(),
				Size:  info.Size,
			})
		}
		if info.IsDir() {
			searchAFC(ctx, client, childPath, query, depth+1, limit, out)
		}
	}
}

// depthBelow counts path separators between a root and one of its
// descendants.
func depthBelow(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return 0
	}
	return strings.Count(rel, string(filepath.Separator))
}
