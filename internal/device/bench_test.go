package device

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkParseSimctlOutput(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`{"devices":{`)
	for r := 0; r < 4; r++ {
		if r > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `"com.apple.CoreSimulator.SimRuntime.iOS-1%d-0":[`, r)
		for i := 0; i < 25; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, `{"state":"Shutdown","isAvailable":true,"name":"iPhone %d","udid":"UDID-%d-%d"}`, i, r, i)
		}
		sb.WriteString("]")
	}
	sb.WriteString("}}")
	data := []byte(sb.String())
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := parseSimctlOutput(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseLsLaR(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("/data/data/com.x:\n")
	for d := 0; d < 30; d++ {
		for i := 0; i < 30; i++ {
			fmt.Fprintf(&sb, "-rw-rw---- 1 u0_a1 u0_a1 %d 2024-01-01 10:00 file%d.txt\n", i*100, i)
		}
		fmt.Fprintf(&sb, "\n/data/data/com.x/dir%d:\n", d)
	}
	data := sb.String()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parseLsLaR(data, "com.x", 4)
	}
}
