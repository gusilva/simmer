package device

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"testing"
)

// buildZip returns a real zip archive containing the named entries, so the
// central-directory reader is tested against bytes the stdlib produced
// rather than bytes this test invented.
func buildZip(t *testing.T, names ...string) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range names {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
		if _, err := f.Write([]byte("payload")); err != nil {
			t.Fatalf("write %q: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// readNames resolves the central directory the way apkEntryNames does, but
// against an in-memory archive: the tail is the whole file, so TailStart is
// zero and the directory is always already present.
func readNames(t *testing.T, archive []byte) []string {
	t.Helper()

	dir, err := locateCentralDirectory(archive)
	if err != nil {
		t.Fatalf("locate central directory: %v", err)
	}
	if !dir.InTail() {
		t.Fatalf("whole archive was given, yet the directory is reported outside it: %+v", dir)
	}
	if dir.TailStart != 0 {
		t.Errorf("TailStart = %d, want 0 when the tail is the whole file", dir.TailStart)
	}
	start := dir.Offset - dir.TailStart
	return parseCentralDirectoryNames(archive[start : start+dir.Size])
}

func TestCentralDirectoryNames(t *testing.T) {
	want := []string{
		"AndroidManifest.xml",
		"assets/index.android.bundle",
		"lib/arm64-v8a/libhermes.so",
		"classes.dex",
	}

	got := readNames(t, buildZip(t, want...))

	if len(got) != len(want) {
		t.Fatalf("got %d names (%v), want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("name[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCentralDirectoryWithManyEntries(t *testing.T) {
	// Entry count and long names push the directory well past one record,
	// which is where an off-by-one in the stride would show up.
	var names []string
	for i := range 500 {
		names = append(names, "res/drawable-xxhdpi-v4/a_fairly_long_resource_name_"+string(rune('a'+i%26))+string(rune('0'+i%10))+".png")
	}
	names = append(names, "assets/index.android.bundle")

	got := readNames(t, buildZip(t, names...))

	if len(got) != len(names) {
		t.Fatalf("got %d names, want %d", len(got), len(names))
	}
	if got[len(got)-1] != "assets/index.android.bundle" {
		t.Errorf("last name = %q, want the bundle", got[len(got)-1])
	}
}

func TestLocateCentralDirectoryDerivesTailStart(t *testing.T) {
	archive := buildZip(t, "a.txt", "assets/x.bundle")

	// Simulate having read only the last N bytes of a larger file.
	const tailLen = 160
	if len(archive) <= tailLen {
		t.Skipf("archive is only %d bytes", len(archive))
	}
	tail := archive[len(archive)-tailLen:]

	dir, err := locateCentralDirectory(tail)
	if err != nil {
		t.Fatalf("locate: %v", err)
	}

	wantTailStart := uint64(len(archive) - tailLen)
	if dir.TailStart != wantTailStart {
		t.Errorf("TailStart = %d, want %d", dir.TailStart, wantTailStart)
	}

	// Whether the directory is in the tail must agree with the real offset.
	if want := dir.Offset >= wantTailStart; dir.InTail() != want {
		t.Errorf("InTail() = %v, want %v (offset %d, tail starts %d)", dir.InTail(), want, dir.Offset, wantTailStart)
	}
}

func TestLocateCentralDirectoryRejectsNonZip(t *testing.T) {
	if _, err := locateCentralDirectory([]byte("not a zip at all")); err == nil {
		t.Error("expected an error for input with no end-of-central-directory record")
	}
	if _, err := locateCentralDirectory(nil); err == nil {
		t.Error("expected an error for empty input")
	}
}

func TestLocateCentralDirectoryReadsZip64Record(t *testing.T) {
	// A zip whose classic record carries sentinels must be read through the
	// zip64 record instead. Built by hand: the stdlib only emits zip64 for
	// archives too large to hold in a test.
	const (
		cdOffset = uint64(1) << 33 // past the 4 GiB the classic field can hold
		cdSize   = uint64(4096)
	)

	var buf bytes.Buffer

	z64 := make([]byte, 56)
	copy(z64, sigZip64EOCD)
	binary.LittleEndian.PutUint64(z64[4:12], 44) // size of this record
	binary.LittleEndian.PutUint64(z64[40:48], cdSize)
	binary.LittleEndian.PutUint64(z64[48:56], cdOffset)
	buf.Write(z64)

	loc := make([]byte, 20)
	copy(loc, sigZip64Locator)
	buf.Write(loc)

	eocd := make([]byte, 22)
	copy(eocd, sigEOCD)
	binary.LittleEndian.PutUint16(eocd[10:12], 0xFFFF)
	binary.LittleEndian.PutUint32(eocd[12:16], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(eocd[16:20], 0xFFFFFFFF)
	buf.Write(eocd)

	dir, err := locateCentralDirectory(buf.Bytes())
	if err != nil {
		t.Fatalf("locate: %v", err)
	}
	if dir.Offset != cdOffset {
		t.Errorf("Offset = %d, want %d", dir.Offset, cdOffset)
	}
	if dir.Size != cdSize {
		t.Errorf("Size = %d, want %d", dir.Size, cdSize)
	}
	if dir.InTail() {
		t.Error("a directory 8 GiB before the tail cannot be in it")
	}
}

func TestParseCentralDirectoryNamesStopsAtGarbage(t *testing.T) {
	archive := buildZip(t, "a.txt", "b.txt")
	dir, err := locateCentralDirectory(archive)
	if err != nil {
		t.Fatalf("locate: %v", err)
	}

	// Truncating mid-record must yield the complete names only, never a
	// partial one, and must not panic.
	raw := archive[dir.Offset : dir.Offset+dir.Size-10]
	got := parseCentralDirectoryNames(raw)

	for _, n := range got {
		if n != "a.txt" && n != "b.txt" {
			t.Errorf("partial name parsed: %q", n)
		}
	}
	if len(got) > 2 {
		t.Errorf("got %d names from a truncated directory, want at most 2", len(got))
	}
}

func TestIsReactNativeEntryName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"assets/index.android.bundle", true},
		{"assets/app.hash123.bundle", true},  // Expo / custom entry file
		{"lib/arm64-v8a/libhermes.so", true}, // Hermes engine
		{"lib/x86_64/libhermesvm.so", true},  // newer Hermes naming
		{"lib/arm64-v8a/libreactnativejni.so", true},
		{"lib/arm64-v8a/libjscexecutor.so", true},
		{"lib/arm64-v8a/libHermes.so", true}, // case-insensitive

		{"AndroidManifest.xml", false},
		{"classes.dex", false},
		{"res/layout/activity_main.xml", false},
		{"assets/fonts/Inter.ttf", false},               // under assets, not a bundle
		{"META-INF/assets/index.android.bundle", false}, // not at the assets/ root
		{"somewhere/libhermes.so", false},               // hermes outside lib/
		{"", false},
	}

	for _, tc := range tests {
		if got := isReactNativeEntryName(tc.name); got != tc.want {
			t.Errorf("isReactNativeEntryName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestSqSingleQuote(t *testing.T) {
	// APK paths carry "~~" and "==" that an unquoted shell would mangle.
	in := "/data/app/~~QLFtsLTFOe2Lg68KyZUdtQ==/club.chizhik-EzH5wF5uC3OQTp_Q0VF6XQ==/base.apk"
	if got, want := sqSingleQuote(in), "'"+in+"'"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if got, want := sqSingleQuote("a'b"), `'a'\''b'`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestCentralDirectorySuffixLen(t *testing.T) {
	// A directory 1 KB before a 64 KB tail of a 1 MB file is 65 KB from the
	// end, which is what the second read has to ask for.
	const (
		fileSize = uint64(1) << 20
		tailLen  = 64 << 10
	)
	dir := centralDirectory{
		Offset:    fileSize - tailLen - 1024,
		TailStart: fileSize - tailLen,
	}

	if got, want := dir.SuffixLen(tailLen), uint64(tailLen+1024); got != want {
		t.Errorf("SuffixLen = %d, want %d", got, want)
	}
	if dir.InTail() {
		t.Error("a directory starting before the tail is not in it")
	}
}
