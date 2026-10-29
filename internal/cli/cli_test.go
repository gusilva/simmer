package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"simmer/internal/device"
)

func TestTableAlignsColumns(t *testing.T) {
	var buf bytes.Buffer
	table(&buf, []string{"NAME", "ID"}, [][]string{
		{"iPhone 16 Pro", "A1"},
		{"SE", "B2"},
	})

	want := "NAME           ID\niPhone 16 Pro  A1\nSE             B2\n"
	if buf.String() != want {
		t.Errorf("got:\n%q\nwant:\n%q", buf.String(), want)
	}
}

func TestTableDoesNotPadTheLastColumn(t *testing.T) {
	var buf bytes.Buffer
	table(&buf, nil, [][]string{{"a", "long value"}, {"b", "x"}})

	for _, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		if strings.HasSuffix(line, " ") {
			t.Errorf("line %q has trailing padding", line)
		}
	}
}

func TestMarks(t *testing.T) {
	tests := []struct {
		app  device.App
		want string
	}{
		{device.App{Type: "User"}, "dev"},
		{device.App{Type: "System"}, ""},
		{device.App{Type: "User", IsReactNative: true}, "dev,rn"},
		{device.App{Type: "System", IsReactNative: true}, "rn"},
	}
	for _, tc := range tests {
		if got := marks(tc.app); got != tc.want {
			t.Errorf("marks(%+v) = %q, want %q", tc.app, got, tc.want)
		}
	}
}

func TestAppVersionPrefersShortVersion(t *testing.T) {
	if got := appVersion(device.App{Version: "42", ShortVersion: "1.2.0"}); got != "1.2.0" {
		t.Errorf("got %q, want 1.2.0", got)
	}
	if got := appVersion(device.App{Version: "42"}); got != "42" {
		t.Errorf("got %q, want 42", got)
	}
}

func TestRunUnknownVerbIsUsageError(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run(context.Background(), []string{"wobble"}, nil, Streams{Out: &out, Err: &errBuf})

	if code != exitUsage {
		t.Errorf("exit code %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errBuf.String(), `unknown command "wobble"`) {
		t.Errorf("stderr does not name the command: %q", errBuf.String())
	}
}

func TestRunHelpIsSuccess(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run(context.Background(), []string{"help"}, nil, Streams{Out: &out, Err: &errBuf})

	if code != exitOK {
		t.Errorf("exit code %d, want %d", code, exitOK)
	}
	if !strings.Contains(out.String(), "simmer db query") {
		t.Errorf("help omits the db query verb: %q", out.String())
	}
}

func TestDBSubcommandRequiresKnownName(t *testing.T) {
	var out, errBuf bytes.Buffer
	c := &command{io: Streams{Out: &out, Err: &errBuf}}

	if code := c.db(context.Background(), nil); code != exitUsage {
		t.Errorf("missing subcommand: exit %d, want %d", code, exitUsage)
	}
	if code := c.db(context.Background(), []string{"drop"}); code != exitUsage {
		t.Errorf("unknown subcommand: exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errBuf.String(), "ls or query") {
		t.Errorf("stderr does not list the subcommands: %q", errBuf.String())
	}
}

func TestDBQueryRejectsMissingFlags(t *testing.T) {
	var out, errBuf bytes.Buffer
	c := &command{io: Streams{Out: &out, Err: &errBuf}}

	if code := c.dbQuery(context.Background(), []string{"--db", "/p/x.db"}); code != exitUsage {
		t.Errorf("missing SQL: exit %d, want %d", code, exitUsage)
	}
	if code := c.dbQuery(context.Background(), []string{"select 1"}); code != exitUsage {
		t.Errorf("missing --db: exit %d, want %d", code, exitUsage)
	}
	if code := c.dbList(context.Background(), nil); code != exitUsage {
		t.Errorf("missing --app: exit %d, want %d", code, exitUsage)
	}
}

func TestDBListRequiresExactlyOneTarget(t *testing.T) {
	var out, errBuf bytes.Buffer
	c := &command{io: Streams{Out: &out, Err: &errBuf}}

	// Neither: there is nothing to search.
	if code := c.dbList(context.Background(), []string{"--device", "booted"}); code != exitUsage {
		t.Errorf("neither --app nor --path: exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errBuf.String(), "--app <bundle id> or --path <directory>") {
		t.Errorf("stderr does not name both options: %q", errBuf.String())
	}

	// Both: the two name different trees, so there is no sensible merge.
	errBuf.Reset()
	code := c.dbList(context.Background(), []string{"--app", "com.acme.app", "--path", "/storage/emulated/0"})
	if code != exitUsage {
		t.Errorf("both --app and --path: exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errBuf.String(), "not both") {
		t.Errorf("stderr does not explain the conflict: %q", errBuf.String())
	}
}
