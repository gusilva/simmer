# Testing Patterns

**Analysis Date:** 2026-05-02

## Test Framework

**Runner:**
- Go `testing` package (standard library)
- Run via `go test ./...`

**Assertion Library:**
- Standard Go comparisons with `t.Errorf` or `t.Fatalf`
- `reflect.DeepEqual` used for complex struct/slice comparisons

**Run Commands:**
```bash
go test ./...          # Run all tests
go test -v ./...       # Verbose output
```

## Test File Organization

**Location:**
- Co-located with source files (e.g., `pkg/device/android_test.go` next to `pkg/device/android.go`)

**Naming:**
- `[filename]_test.go`

**Structure:**
```
pkg/device/
├── android.go
├── android_test.go
├── ios.go
└── ios_test.go
```

## Test Structure

**Suite Organization:**
```go
func TestParseAVDs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Single AVD",
			input:    "Pixel_6_API_33\n",
			expected: []string{"Pixel_6_API_33"},
		},
		// ...
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAVDs([]byte(tt.input))
			// assertion logic
		})
	}
}
```

**Patterns:**
- **Table-Driven Tests:** Extensively used for parsing logic (simctl/adb output)
- **Sub-tests:** `t.Run(tt.name, ...)` for clear reporting

## Mocking

**Framework:**
- Manual interface implementation (no third-party mocking library detected)

**Patterns:**
```go
// From pkg/device/files.go
type MockFileSystem struct {
	// ... fields to control behavior
}

func (m *MockFileSystem) Tree(_ context.Context, _ Device) (FileNode, error) {
	// ... return controlled values
}
```

**What to Mock:**
- External tool calls (xcrun simctl, adb, emulator)
- Filesystem operations (simulated by interfaces like `FileSystem`)

**What NOT to Mock:**
- Pure logic (parsers, data transformations)
- Internal data structures (`Device`, `App`)

## Fixtures and Factories

**Test Data:**
- Inline strings representing CLI tool output (e.g., JSON from `simctl`, raw text from `adb`)
- Hardcoded struct instances for expected results

**Location:**
- Mostly inline within test files

## Coverage

**Requirements:**
- None explicitly enforced, but critical parsing logic is covered

**View Coverage:**
```bash
go test -cover ./...
```

## Test Types

**Unit Tests:**
- Focus on parsing raw command output into internal structures
- Logic tests for the `Coordinator` and `Manager` implementations

**Integration Tests:**
- Not explicitly separated, but `Coordinator` tests often exercise the interaction between multiple components

**E2E Tests:**
- Not detected

## Common Patterns

**Async Testing:**
- `context.WithTimeout` used in production code; tests likely rely on short timeouts or mocked sync behavior

**Error Testing:**
- Table-driven tests include cases for empty/malformed input to verify robustness of parsers

---

*Testing analysis: 2026-05-02*
