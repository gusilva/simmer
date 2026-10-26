package device

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// DefaultSearchLimit caps how many matches a search returns. A search is a
// navigation aid, not a report: past a screenful the useful response is a
// narrower query, not more rows.
const DefaultSearchLimit = 200

// FileMatch is one file or directory a search found.
type FileMatch struct {
	Name  string
	Path  string
	IsDir bool
	Size  int64
}

// FileSearcher is an optional interface a Manager may implement to find files
// by name under a given root.
//
// Search exists alongside the FileSystem scopes rather than inside them
// because the two answer different questions: a scope says what a listing
// covers, while a search reaches past whatever has been listed. An
// interactive tree loads one directory at a time, so a filter over loaded
// nodes can only ever see the part of the device already visited.
type FileSearcher interface {
	Platform() Platform
	SearchFiles(ctx context.Context, dev Device, root, query string, limit int) ([]FileMatch, error)
}

// Every manager searches. These assertions are not ceremony: a file named
// search_android.go or search_ios.go is silently excluded from the build on
// a Mac, because _android and _ios are GOOS suffixes — the capability then
// disappears at run time with nothing failing at compile time.
var (
	_ FileSearcher = (*androidManager)(nil)
	_ FileSearcher = (*physicalAndroidManager)(nil)
	_ FileSearcher = (*iosManager)(nil)
	_ FileSearcher = (*physicalIOSManager)(nil)
)

// SearchFiles finds files and directories whose name contains query, under
// every given root, case-insensitively. Roots are searched in order and
// results are merged, deduplicated by path, and capped at limit.
//
// An app's storage spans more than one root on Android, which is why this
// takes a list: a single root cannot name both the private data dir and the
// external app directory.
func (c *Coordinator) SearchFiles(ctx context.Context, dev Device, roots []string, query string, limit int) ([]FileMatch, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search needs a query")
	}
	if limit <= 0 {
		limit = DefaultSearchLimit
	}

	s, err := pickCapability[FileSearcher](c.Managers, dev.Platform, dev.Kind, "file searcher")
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var out []FileMatch
	var firstErr error

	for _, root := range roots {
		if root == "" || len(out) >= limit {
			continue
		}

		matches, err := s.SearchFiles(ctx, dev, root, query, limit-len(out))
		if err != nil {
			// One unreadable root must not hide what the others found.
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		for _, m := range matches {
			if seen[m.Path] {
				continue
			}
			seen[m.Path] = true
			out = append(out, m)
			if len(out) >= limit {
				break
			}
		}
	}

	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}

	sortMatches(out)

	return out, nil
}

// sortMatches orders results the way they are read: files and directories
// together, shallowest first, then alphabetically — so the match nearest the
// root, which is usually the one meant, is at the top.
func sortMatches(matches []FileMatch) {
	sort.SliceStable(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if da, db := strings.Count(a.Path, "/"), strings.Count(b.Path, "/"); da != db {
			return da < db
		}
		return strings.ToLower(a.Path) < strings.ToLower(b.Path)
	})
}

// matchesQuery reports whether a file name satisfies the query. A query
// containing a separator is matched against the whole path, so
// "databases/business" narrows by location as well as by name.
func matchesQuery(name, path, query string) bool {
	if strings.Contains(query, "/") {
		return strings.Contains(strings.ToLower(path), strings.ToLower(query))
	}
	return strings.Contains(strings.ToLower(name), strings.ToLower(query))
}
