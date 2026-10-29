# Single source of truth for the project's checks: CI runs these targets, so
# a green `make check` locally means a green pipeline.

GO ?= go
BINARY ?= simmer
DEADCODE ?= golang.org/x/tools/cmd/deadcode@v0.52.0

.PHONY: build run test test-race lint lint-new fmt-check vet deadcode exclusions check check-device tidy clean

## build: compile the CLI
build:
	$(GO) build -o $(BINARY) ./cmd/simmer

## run: start the TUI
run:
	$(GO) run ./cmd/simmer

## test: unit tests
#
# Pi agents: prefer `rtk proxy go test ./...` locally - plain `go test` output
# is summarised by the harness and drops t.Logf lines and failure detail.
test:
	$(GO) test ./...

## test-race: unit tests under the race detector, as CI runs them
test-race:
	$(GO) test -race -coverprofile=coverage.out -covermode=atomic ./...

## vet: go vet
vet:
	$(GO) vet ./...

## lint: golangci-lint over the whole tree
#
# Reports a backlog that predates the CI job; CI only gates new code.
lint:
	golangci-lint run

## lint-new: exactly what CI lints - code that differs from the base branch
#
# Skips rather than fails when golangci-lint or the base ref is missing, so a
# fresh clone or an offline checkout can still run `make check`. The default
# base matches CI; overriding it with HEAD~1 lints only the last commit and
# will miss findings CI reports.
LINT_BASE ?= origin/main
lint-new:
	@if ! command -v golangci-lint >/dev/null; then 		echo "skipping lint: golangci-lint is not installed"; 	elif ! git rev-parse --verify --quiet $(LINT_BASE) >/dev/null; then 		echo "skipping lint: $(LINT_BASE) not found (git fetch origin main)"; 	else 		golangci-lint run --new-from-rev=$(LINT_BASE) --timeout=5m; 	fi

## fmt-check: fail when anything is unformatted
fmt-check:
	@unformatted=$$(gofmt -l cmd internal); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed:"; echo "$$unformatted"; exit 1; \
	fi

## deadcode: fail on code unreachable from the binary and the tests
deadcode:
	$(GO) run $(DEADCODE) -test ./...
	@test -z "$$($(GO) run $(DEADCODE) -test ./... 2>/dev/null)" || \
		{ echo "unreachable code above: delete it or call it"; exit 1; }

## exclusions: fail on files the build silently ignores (see the script)
exclusions:
	./scripts/check-build-exclusions.sh

## check: everything CI enforces, including the lint job
check: exclusions fmt-check vet build test deadcode lint-new

## check-device: integration tests against attached simulators and phones
#
# Skipped by default because they need real hardware. Each test skips itself
# when the device it needs is absent, so a partial setup still reports.
check-device:
	$(GO) test -tags device -count=1 ./internal/device/ -v

## tidy: prune go.mod
tidy:
	$(GO) mod tidy

clean:
	rm -f $(BINARY) coverage.out
