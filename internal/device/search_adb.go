package device

import (
	"context"
	"fmt"
	gopath "path"
	"strconv"
	"strings"

	"simmer/internal/logging"
)

// SearchFiles implements FileSearcher for Android emulators.
func (m *androidManager) SearchFiles(ctx context.Context, dev Device, root, query string, limit int) ([]FileMatch, error) {
	return searchAndroidFiles(ctx, m.logger, dev, root, query, limit)
}

// SearchFiles implements FileSearcher for physical Android devices.
func (m *physicalAndroidManager) SearchFiles(ctx context.Context, dev Device, root, query string, limit int) ([]FileMatch, error) {
	return searchAndroidFiles(ctx, m.logger, dev, root, query, limit)
}

// searchAndroidFiles runs `find` on the device and stats the matches.
//
// The search runs on the device rather than by walking the tree from here:
// one shell round-trip answers it, where a walk would cost one per
// directory. A path inside private storage is searched as the owning app,
// which the path itself names.
func searchAndroidFiles(ctx context.Context, logger *logging.Logger, dev Device, root, query string, limit int) ([]FileMatch, error) {
	serial, err := resolveAndroidSerial(ctx, dev.ID)
	if err != nil {
		return nil, fmt.Errorf("find serial: %w", err)
	}

	// Two shapes, because -iname matches the file name alone: a query
	// naming a location ("databases/business") has to be matched against
	// the whole path, the same rule matchesQuery applies where the tree is
	// walked here.
	//
	// The path case is filtered with grep rather than find -ipath, which
	// toybox accepts and then matches nothing (seen on Android 9). The
	// query is escaped into a regex instead of passed to grep -F, because
	// that same toybox grep treats -F as "match every line".
	//
	// -type is left open so directories match too. Errors are dropped on
	// the device: an unreadable subdirectory is normal and must not fail
	// the whole search.
	// The pipeline is handed to the device shell as written, with run-as as
	// its first command rather than wrapping it: nesting it in `sh -c '…'`
	// needs a second level of quoting, which adb does not carry reliably —
	// the grep pattern arrived empty and every path matched.
	cmd := androidRunAsForPath(root) + "find " + sqSingleQuote(root)
	if strings.Contains(query, "/") {
		cmd += " 2>/dev/null | grep -i -- " + sqSingleQuote(escapeBRE(query))
	} else {
		pattern := "*" + strings.ReplaceAll(query, "*", "") + "*"
		cmd += " -iname " + sqSingleQuote(pattern) + " 2>/dev/null"
	}
	cmd += " | head -n " + strconv.Itoa(limit)

	out, err := adbExecOut(ctx, logger, serial, cmd)
	if err != nil {
		return nil, fmt.Errorf("find %s: %w", root, err)
	}

	paths := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(paths) == 1 && paths[0] == "" {
		return nil, nil
	}

	return statAndroidPaths(ctx, logger, serial, root, paths, limit)
}

// statAndroidPaths turns found paths into matches, asking the device once
// for the size and type of all of them.
func statAndroidPaths(ctx context.Context, logger *logging.Logger, serial, root string, paths []string, limit int) ([]FileMatch, error) {
	quoted := make([]string, 0, len(paths))
	for _, p := range paths {
		if p = strings.TrimSpace(p); p != "" {
			quoted = append(quoted, sqSingleQuote(p))
		}
	}
	if len(quoted) == 0 {
		return nil, nil
	}

	// "<size> <type> <path>" per line, where type is the first character of
	// the mode: "d" for a directory.
	cmd := androidRunAsForPath(root) + "stat -c " + sqSingleQuote("%s %A %n") + " " +
		strings.Join(quoted, " ") + " 2>/dev/null"

	out, err := adbExecOut(ctx, logger, serial, cmd)
	if err != nil {
		// Without stat the paths are still useful; report them bare.
		return bareMatches(paths, limit), nil
	}

	var matches []FileMatch
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		fields := strings.SplitN(strings.TrimSpace(line), " ", 3)
		if len(fields) < 3 {
			continue
		}
		size, _ := strconv.ParseInt(fields[0], 10, 64)
		path := fields[2]

		matches = append(matches, FileMatch{
			Name:  gopath.Base(path),
			Path:  path,
			IsDir: strings.HasPrefix(fields[1], "d"),
			Size:  size,
		})
		if len(matches) >= limit {
			break
		}
	}

	if len(matches) == 0 {
		return bareMatches(paths, limit), nil
	}

	return matches, nil
}

func bareMatches(paths []string, limit int) []FileMatch {
	var out []FileMatch
	for _, p := range paths {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		out = append(out, FileMatch{Name: gopath.Base(p), Path: p})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// escapeBRE turns a literal substring into a basic regular expression that
// matches it and nothing else, so grep can be used for fixed-string matching
// on devices where grep -F misbehaves.
func escapeBRE(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`\.[]^$*`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// androidPackageForPath returns the app that owns a private-storage path.
// /data/data/<pkg>/… is readable only as <pkg>, and the path names it, so no
// caller has to pass a package alongside a path it already contains.
// Reports false for anything outside private storage, which the shell user
// reads directly.
func androidPackageForPath(path string) (string, bool) {
	rest, ok := strings.CutPrefix(path, "/data/data/")
	if !ok {
		// /data/user/<N>/<pkg> and /data/user_de/<N>/<pkg> carry a user id
		// between the prefix and the package.
		for _, prefix := range []string{"/data/user/", "/data/user_de/"} {
			withUser, isUser := strings.CutPrefix(path, prefix)
			if !isUser {
				continue
			}
			user, after, found := strings.Cut(withUser, "/")
			if !found || user == "" {
				return "", false
			}
			rest, ok = after, true
			break
		}
	}
	if !ok {
		return "", false
	}

	pkg, _, _ := strings.Cut(rest, "/")
	if pkg == "" {
		return "", false
	}
	return pkg, true
}

// androidRunAsForPath returns the shell prefix needed to read a path, empty
// when the shell user can read it directly.
func androidRunAsForPath(path string) string {
	pkg, ok := androidPackageForPath(path)
	if !ok {
		return ""
	}
	return "run-as " + sqSingleQuote(pkg) + " "
}
