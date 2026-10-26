package cli

import (
	"context"
	"fmt"
	gopath "path"
	"strings"

	"simmer/internal/device"
	"simmer/internal/session"
)

// devices lists every discovered device.
func (c *command) devices(ctx context.Context, args []string) int {
	fs := c.flags("devices")
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	res := c.sess.ListDevices(ctx)
	for _, err := range res.Errors {
		writef(c.io.Err, "warning: %v\n", err)
	}

	if *asJSON {
		type row struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Platform string `json:"platform"`
			Version  string `json:"version"`
			Status   string `json:"status"`
			Kind     string `json:"kind"`
			Serial   string `json:"serial,omitempty"`
		}
		out := make([]row, 0, len(res.Devices))
		for _, d := range res.Devices {
			out = append(out, row{d.ID, d.Name, string(d.Platform), d.Version, string(d.Status), kindName(d.Kind), d.Serial})
		}
		if err := emitJSON(c.io.Out, out); err != nil {
			return c.fail(err)
		}
		return exitOK
	}

	rows := make([][]string, 0, len(res.Devices))
	for _, d := range res.Devices {
		rows = append(rows, []string{d.Name, string(d.Platform), d.Version, string(d.Status), kindName(d.Kind), d.ID})
	}
	table(c.io.Out, []string{"NAME", "PLATFORM", "VERSION", "STATUS", "KIND", "ID"}, rows)

	return exitOK
}

// apps lists the apps installed on one device.
func (c *command) apps(ctx context.Context, args []string) int {
	fs := c.flags("apps")
	ref := fs.String("device", "", "device reference (udid, serial, name, fragment, or \"booted\")")
	asJSON := fs.Bool("json", false, "emit JSON")
	fast := fs.Bool("fast", false, "skip enrichment: print the bare list without versions or React Native marks")
	withRN := fs.Bool("react-native", true, "detect React Native apps; --react-native=false skips one device round-trip per app")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	dev, code := c.resolve(ctx, *ref)
	if code != exitOK {
		return code
	}

	cat, err := c.sess.Apps(ctx, dev, session.EnrichOptions{SkipReactNative: !*withRN})
	if err != nil {
		return c.fail(err)
	}

	apps := cat.Apps
	if *fast {
		cat.Stop()
	} else {
		// Scripted output has to be complete or it is a liar: wait for the
		// per-app round-trips that carry version names and React Native
		// marks before printing anything.
		apps = cat.Settled(ctx)
	}

	if *asJSON {
		type row struct {
			BundleID     string `json:"bundle_id"`
			Label        string `json:"label"`
			Version      string `json:"version"`
			ShortVersion string `json:"short_version"`
			Type         string `json:"type"`
			Local        bool   `json:"local"`
			ReactNative  bool   `json:"react_native"`
			Path         string `json:"path,omitempty"`
		}
		out := make([]row, 0, len(apps))
		for _, a := range apps {
			out = append(out, row{a.BundleID, a.Label(), a.Version, a.ShortVersion, a.Type, a.IsLocal(), a.IsReactNative, a.Path})
		}
		if err := emitJSON(c.io.Out, out); err != nil {
			return c.fail(err)
		}
		return exitOK
	}

	rows := make([][]string, 0, len(apps))
	for _, a := range apps {
		rows = append(rows, []string{a.Label(), a.BundleID, appVersion(a), a.Type, marks(a)})
	}
	table(c.io.Out, []string{"APP", "BUNDLE ID", "VERSION", "TYPE", "MARKS"}, rows)

	return exitOK
}

// db dispatches the database subcommands.
func (c *command) db(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return c.misuse("db needs a subcommand: ls or query")
	}
	switch args[0] {
	case "ls":
		return c.dbList(ctx, args[1:])
	case "query":
		return c.dbQuery(ctx, args[1:])
	default:
		return c.misuse("unknown db subcommand %q: expected ls or query", args[0])
	}
}

// dbList lists the SQLite databases inside an app's sandbox.
func (c *command) dbList(ctx context.Context, args []string) int {
	fs := c.flags("db ls")
	ref := fs.String("device", "", "device reference")
	bundleID := fs.String("app", "", "bundle id / package name whose storage to search")
	path := fs.String("path", "", "absolute device directory to search instead of an app's storage")
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	switch {
	case *bundleID == "" && *path == "":
		return c.misuse("db ls needs --app <bundle id> or --path <directory>")
	case *bundleID != "" && *path != "":
		return c.misuse("db ls takes --app or --path, not both")
	}

	dev, code := c.resolve(ctx, *ref)
	if code != exitOK {
		return code
	}

	var (
		dbs []session.DatabaseFile
		err error
	)
	if *path != "" {
		dbs, err = c.sess.DatabasesAt(ctx, dev, *path)
	} else {
		dbs, err = c.sess.Databases(ctx, dev, *bundleID)
	}
	if err != nil {
		return c.fail(err)
	}

	if *asJSON {
		type row struct {
			Name string `json:"name"`
			Path string `json:"path"`
			Size int64  `json:"size"`
		}
		out := make([]row, 0, len(dbs))
		for _, d := range dbs {
			out = append(out, row{d.Name, d.Path, d.Size})
		}
		if err := emitJSON(c.io.Out, out); err != nil {
			return c.fail(err)
		}
		return exitOK
	}

	rows := make([][]string, 0, len(dbs))
	for _, d := range dbs {
		rows = append(rows, []string{d.Name, fmt.Sprintf("%d", d.Size), d.Path})
	}
	table(c.io.Out, []string{"DATABASE", "BYTES", "PATH"}, rows)

	return exitOK
}

// dbQuery runs one SQL statement against a device database.
func (c *command) dbQuery(ctx context.Context, args []string) int {
	fs := c.flags("db query")
	ref := fs.String("device", "", "device reference")
	bundleID := fs.String("app", "", "bundle id / package name owning the database")
	dbPath := fs.String("db", "", "path to the database inside the sandbox (see: simmer db ls)")
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	sql := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if sql == "" {
		return c.misuse("db query needs a SQL statement")
	}
	if *dbPath == "" {
		return c.misuse("db query needs --db <path>: run simmer db ls to find one")
	}

	dev, code := c.resolve(ctx, *ref)
	if code != exitOK {
		return code
	}

	target := session.DBTarget{
		Device:    dev,
		PackageID: *bundleID,
		Path:      *dbPath,
		Name:      gopath.Base(*dbPath),
	}

	rows, err := c.sess.Query(ctx, target, sql)
	if err != nil {
		return c.fail(err)
	}
	if len(rows) == 0 {
		return exitOK
	}

	header, body := rows[0], rows[1:]

	if *asJSON {
		out := make([]map[string]string, 0, len(body))
		for _, row := range body {
			rec := make(map[string]string, len(header))
			for i, col := range header {
				if i < len(row) {
					rec[col] = row[i]
				}
			}
			out = append(out, rec)
		}
		if err := emitJSON(c.io.Out, out); err != nil {
			return c.fail(err)
		}
		return exitOK
	}

	table(c.io.Out, header, body)

	return exitOK
}

// resolve discovers devices and resolves a reference, reporting failures on
// the error stream. The returned code is exitOK when dev is usable.
func (c *command) resolve(ctx context.Context, ref string) (device.Device, int) {
	res := c.sess.ListDevices(ctx)
	for _, err := range res.Errors {
		writef(c.io.Err, "warning: %v\n", err)
	}

	dev, err := c.sess.Resolve(ref)
	if err != nil {
		return device.Device{}, c.fail(err)
	}

	return dev, exitOK
}

func kindName(k device.DeviceKind) string {
	if k == device.KindPhysical {
		return "physical"
	}
	return "virtual"
}

// appVersion prefers the marketing version, falling back to the build version.
func appVersion(a device.App) string {
	if a.ShortVersion != "" {
		return a.ShortVersion
	}
	return a.Version
}

// marks renders the badges the Apps panel shows: a Local app and a React
// Native build.
func marks(a device.App) string {
	var out []string
	if a.IsLocal() {
		out = append(out, "dev")
	}
	if a.IsReactNative {
		out = append(out, "rn")
	}
	return strings.Join(out, ",")
}
