package device

import (
	"strconv"
	"strings"
)

// The shell commands sent to an Android device, built as values.
//
// Every Android bug this package has had was a command-construction bug, not
// a logic one: a missing run-as on a private path, a grep pattern lost to a
// nested `sh -c`, `grep -F` matching every line on one toybox build, a
// `find -ipath` that matched nothing on another. None of that is visible
// from a function that builds a string and immediately executes it — the
// only way to see what went to the device was to attach one.
//
// Building the command separately makes the exact text a value a table test
// can assert on, which is how these are pinned now.

// androidListArgs builds the argv that lists a path: recursive unless
// entriesOnly, and run as the owning app when the path is private.
//
// The shell user is refused in /data/data, so without the run-as every
// directory in an app's own data failed to open — a tree that silently would
// not expand.
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

// androidSQLiteCmd builds the shell command that queries a database with the
// device's own sqlite3.
func androidSQLiteCmd(dbPath, query string) string {
	return androidRunAsForPath(dbPath) +
		"sqlite3 -csv -header " + sqSingleQuote(dbPath) + " " + sqSingleQuote(query)
}

// androidSQLiteVersionCmd builds the shell command that reports the device's
// sqlite3 version. packageID is honoured when given so the probe runs in the
// same context a query would.
func androidSQLiteVersionCmd(packageID string) string {
	if packageID == "" {
		return "sqlite3 --version"
	}
	return "run-as " + sqSingleQuote(packageID) + " sqlite3 --version"
}

// androidCatCmd builds the shell command that streams a file off the device,
// as the owning app when the path is private.
func androidCatCmd(src string) string {
	return androidRunAsForPath(src) + "cat " + sqSingleQuote(src)
}

// androidFindCmd builds the shell command that searches a root by name.
//
// Two shapes, because -iname matches the file name alone: a query naming a
// location ("databases/business") has to be matched against the whole path.
// The path case is filtered with grep rather than find -ipath, which toybox
// accepts and then matches nothing (seen on Android 9), and the query is
// escaped into a regex rather than passed to grep -F, because that same
// toybox grep treats -F as "match every line".
//
// The pipeline is handed to the device shell as written, with run-as as its
// first command rather than wrapping it: nesting it in `sh -c '…'` needs a
// second level of quoting that adb does not carry reliably — the grep
// pattern arrived empty and every path matched.
func androidFindCmd(root, query string, limit int) string {
	cmd := androidRunAsForPath(root) + "find " + sqSingleQuote(root)

	if strings.Contains(query, "/") {
		cmd += " 2>/dev/null | grep -i -- " + sqSingleQuote(escapeBRE(query))
	} else {
		pattern := "*" + strings.ReplaceAll(query, "*", "") + "*"
		cmd += " -iname " + sqSingleQuote(pattern) + " 2>/dev/null"
	}

	return cmd + " | head -n " + strconv.Itoa(limit)
}

// androidStatCmd builds the shell command that reports size, mode and name
// for a set of paths in one round-trip. root decides the access route; the
// paths are assumed to live under it.
func androidStatCmd(root string, paths []string) string {
	quoted := make([]string, 0, len(paths))
	for _, p := range paths {
		if p = strings.TrimSpace(p); p != "" {
			quoted = append(quoted, sqSingleQuote(p))
		}
	}
	if len(quoted) == 0 {
		return ""
	}

	return androidRunAsForPath(root) + "stat -c " + sqSingleQuote("%s %A %n") + " " +
		strings.Join(quoted, " ") + " 2>/dev/null"
}
