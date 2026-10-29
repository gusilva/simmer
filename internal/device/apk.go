package device

import (
	"context"
	"encoding/binary"
	"fmt"
	"os/exec"
	"strings"

	"simmer/internal/logging"
)

// Reading an APK's entry list without transferring the APK.
//
// Deciding whether an app is a React Native build means looking at the names
// of the files inside its APK. Pulling the whole APK to do that is ruinous on
// a physical device: on a 56-app phone it moved 5.3 GB over USB and took four
// and a half minutes, dominated by APKs of 80–500 MB each.
//
// A zip stores its complete name list in the central directory at the end of
// the file, so only that tail is needed. Two ranged reads at most, usually
// one: the last 64 KB nearly always contains the whole central directory, and
// when it doesn't the first read says exactly where the directory starts. The
// same phone then answers in seconds and moves ~20 MB.
//
// Reads go through `adb exec-out`, which passes bytes through untranslated.
// APKs under /data/app are world-readable, so no run-as or root is involved —
// the same access `adb pull` already relies on.
//
// Both reads ask for a number of bytes from the end of the file, never an
// offset from its start. This is not cosmetic: `tail -c +N` makes the device
// read and discard everything before N, which on a 500 MB APK costs 2.6 s,
// while `tail -c N` seeks and costs 0.4 s for the same bytes. The central
// directory is always a suffix of the file — the records that follow it are
// the last thing in the archive — so the cheap form always applies.

// apkTailBytes is how much of an APK's end is fetched first. It has to cover
// the end-of-central-directory record, the zip64 records when present, and —
// for the single-round-trip case — the central directory itself.
const apkTailBytes = 64 << 10

// Zip record signatures, little-endian on disk.
var (
	sigEOCD         = []byte{'P', 'K', 0x05, 0x06} // end of central directory
	sigCentralFile  = []byte{'P', 'K', 0x01, 0x02} // central directory file header
	sigZip64EOCD    = []byte{'P', 'K', 0x06, 0x06} // zip64 end of central directory
	sigZip64Locator = []byte{'P', 'K', 0x06, 0x07} // zip64 EOCD locator
)

// centralDirectory locates a zip's central directory within the file.
type centralDirectory struct {
	Offset uint64 // absolute offset of the first central directory header
	Size   uint64 // total bytes of the central directory
	// TailStart is the absolute offset the tail buffer begins at, derived
	// from where the end-of-central-directory record sat inside it.
	TailStart uint64
}

// InTail reports whether the whole central directory is already inside the
// tail buffer the record was read from, meaning no second read is needed.
func (d centralDirectory) InTail() bool { return d.Offset >= d.TailStart }

// SuffixLen returns how many bytes from the end of the file cover the whole
// central directory, given the length of the tail the record was read from.
func (d centralDirectory) SuffixLen(tailLen int) uint64 {
	return (d.TailStart + uint64(tailLen)) - d.Offset
}

// apkEntryNames returns the name of every entry in the APK at apkPath on the
// device with the given serial. The APK itself is never transferred; only its
// central directory is.
func apkEntryNames(ctx context.Context, logger *logging.Logger, serial, apkPath string) ([]string, error) {
	tail, err := readDeviceFileTail(ctx, logger, serial, apkPath, apkTailBytes)
	if err != nil {
		return nil, err
	}

	dir, err := locateCentralDirectory(tail)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", apkPath, err)
	}

	var raw []byte
	if dir.InTail() {
		start := dir.Offset - dir.TailStart
		end := min(start+dir.Size, uint64(len(tail)))
		raw = tail[start:end]
	} else {
		// The central directory starts before the slice already read; fetch
		// the whole suffix that contains it in one more read.
		raw, err = readDeviceFileTail(ctx, logger, serial, apkPath, int(dir.SuffixLen(len(tail))))
		if err != nil {
			return nil, err
		}
	}

	return parseCentralDirectoryNames(raw), nil
}

// readDeviceFileTail returns the last n bytes of a file on the device.
func readDeviceFileTail(ctx context.Context, logger *logging.Logger, serial, path string, n int) ([]byte, error) {
	return adbExecOut(ctx, logger, serial, fmt.Sprintf("tail -c %d %s", n, sqSingleQuote(path)))
}

// adbExecOut runs a shell command on the device and returns its raw stdout.
// exec-out is used rather than shell because it does not translate bytes.
func adbExecOut(ctx context.Context, logger *logging.Logger, serial, shellCmd string) ([]byte, error) {
	args := []string{"-s", serial, "exec-out", shellCmd}
	out, err := exec.CommandContext(ctx, "adb", args...).Output()
	// The output is binary; log only its size.
	logger.LogExec("adb exec-out", args, fmt.Sprintf("%d bytes", len(out)), err)
	if err != nil {
		return nil, fmt.Errorf("adb exec-out: %w", err)
	}
	return out, nil
}

// locateCentralDirectory finds the central directory described by the
// end-of-central-directory record in tail, which must be the final bytes of
// the file. Zip64 records are honoured when the classic record's sentinel
// values say the real numbers live there.
func locateCentralDirectory(tail []byte) (centralDirectory, error) {
	eocd := lastIndex(tail, sigEOCD)
	if eocd < 0 {
		return centralDirectory{}, fmt.Errorf("no end-of-central-directory record in the last %d bytes", len(tail))
	}
	if len(tail)-eocd < 22 {
		return centralDirectory{}, fmt.Errorf("truncated end-of-central-directory record")
	}

	size := uint64(binary.LittleEndian.Uint32(tail[eocd+12 : eocd+16]))
	offset := uint64(binary.LittleEndian.Uint32(tail[eocd+16 : eocd+20]))
	entries := uint64(binary.LittleEndian.Uint16(tail[eocd+10 : eocd+12]))
	recordEnd := eocd

	// Any sentinel means the true values are in the zip64 record, which sits
	// immediately before this one, after the central directory.
	if size == 0xFFFFFFFF || offset == 0xFFFFFFFF || entries == 0xFFFF {
		z64 := lastIndex(tail[:eocd], sigZip64EOCD)
		if z64 < 0 || len(tail)-z64 < 56 {
			return centralDirectory{}, fmt.Errorf("zip64 archive whose zip64 record is not in the last %d bytes", len(tail))
		}
		size = binary.LittleEndian.Uint64(tail[z64+40 : z64+48])
		offset = binary.LittleEndian.Uint64(tail[z64+48 : z64+56])
		recordEnd = z64
	}

	// The records after the central directory are the only fixed landmark, so
	// the tail's absolute start is derived from where they begin: the central
	// directory ends exactly where they do.
	end := offset + size
	if end < uint64(recordEnd) {
		return centralDirectory{}, fmt.Errorf("central directory extends past its own end record")
	}

	return centralDirectory{
		Offset:    offset,
		Size:      size,
		TailStart: end - uint64(recordEnd),
	}, nil
}

// parseCentralDirectoryNames returns the entry name from every central
// directory file header in raw, stopping at the first record that is not one
// (the end-of-central-directory record, or a truncated tail).
func parseCentralDirectoryNames(raw []byte) []string {
	const headerLen = 46
	var names []string

	for off := 0; off+headerLen <= len(raw); {
		if !hasPrefixAt(raw, off, sigCentralFile) {
			break
		}
		nameLen := int(binary.LittleEndian.Uint16(raw[off+28 : off+30]))
		extraLen := int(binary.LittleEndian.Uint16(raw[off+30 : off+32]))
		commentLen := int(binary.LittleEndian.Uint16(raw[off+32 : off+34]))

		nameStart := off + headerLen
		nameEnd := nameStart + nameLen
		if nameEnd > len(raw) {
			break
		}
		names = append(names, string(raw[nameStart:nameEnd]))

		off = nameEnd + extraLen + commentLen
	}

	return names
}

// isReactNativeEntryName reports whether one APK entry name identifies the
// app as a React Native build.
//
// Default Metro output is assets/index.android.bundle, but Expo and custom
// entry-file configs ship the JS bundle under a different or hashed name —
// still always under assets/ with a .bundle extension, so the match is on
// that shape rather than the exact default. The native libraries are the
// second signal: modern builds set extractNativeLibs="false" (the AGP
// default), so these .so files are read straight out of the APK and never
// appear on disk, which is why the APK's own entry list is the thing to look
// at.
func isReactNativeEntryName(name string) bool {
	if strings.HasPrefix(name, "assets/") && strings.HasSuffix(name, ".bundle") {
		return true
	}
	if !strings.HasPrefix(name, "lib/") {
		return false
	}
	lower := strings.ToLower(name)
	return strings.Contains(lower, "hermes") ||
		strings.Contains(lower, "reactnativejni") ||
		strings.Contains(lower, "jscexecutor")
}

func lastIndex(b, sig []byte) int {
	for i := len(b) - len(sig); i >= 0; i-- {
		if hasPrefixAt(b, i, sig) {
			return i
		}
	}
	return -1
}

func hasPrefixAt(b []byte, off int, sig []byte) bool {
	if off < 0 || off+len(sig) > len(b) {
		return false
	}
	for i, c := range sig {
		if b[off+i] != c {
			return false
		}
	}
	return true
}
