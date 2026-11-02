package cli

import (
	"context"

	"simmer/internal/device"
)

// doctor prints the Doctor report. It exits 1 when any check fails, so a
// script can gate on it before relying on Simmer.
func (c *command) doctor(ctx context.Context, args []string) int {
	fs := c.flags("doctor")
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	report := c.sess.Doctor(ctx)

	code := exitOK
	if report.Failed() {
		code = exitError
	}

	if *asJSON {
		type row struct {
			Section string `json:"section"`
			Check   string `json:"check"`
			Status  string `json:"status"`
			Detail  string `json:"detail"`
			Fix     string `json:"fix,omitempty"`
		}
		out := make([]row, 0, len(report.Checks))
		for _, ch := range report.Checks {
			out = append(out, row{ch.Section, ch.Name, string(ch.Severity), ch.Detail, ch.Fix})
		}
		if err := emitJSON(c.io.Out, out); err != nil {
			return c.fail(err)
		}
		return code
	}

	rows := make([][]string, 0, len(report.Checks))
	for _, ch := range report.Checks {
		rows = append(rows, []string{severityMark(ch.Severity), ch.Section, ch.Name, ch.Detail})
		if ch.Fix != "" {
			rows = append(rows, []string{"", "", "", "fix: " + ch.Fix})
		}
	}
	table(c.io.Out, []string{"", "SECTION", "CHECK", "DETAIL"}, rows)

	return code
}

func severityMark(s device.Severity) string {
	switch s {
	case device.SeverityFail:
		return "✗"
	case device.SeverityWarn:
		return "!"
	default:
		return "✓"
	}
}

// warnDiscovery reports what discovery could not answer. A stopped adb
// server is called out by name: without it an empty Android list reads as
// "no devices" rather than "devices unreachable".
func (c *command) warnDiscovery(res device.DiscoveryResult) {
	for _, err := range res.Errors {
		writef(c.io.Err, "warning: %v\n", err)
	}
	if res.AdbServer == device.AdbServerStopped {
		writef(c.io.Err, "warning: adb server not running; Android devices are unreachable — run 'simmer doctor'\n")
	}
}
