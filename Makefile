# Single source of truth for the project's checks: CI runs these targets, so
# a green `make check` locally means a green pipeline.

GO ?= go
BINARY ?= simmer
DEADCODE ?= golang.org/x/tools/cmd/deadcode@v0.52.0

.PHONY: build run test test-race lint fmt-check vet deadcode exclusions check check-device tidy clean

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
# Gating the whole tree, not just the diff: the backlog it used to report is
# cleared, and a whole-tree gate has no base ref to get wrong. A --new-from-rev
# scope silently passes when the base is wrong, which is how two findings
# reached CI.
lint:
	@if command -v golangci-lint >/dev/null; then \
		golangci-lint run; \
	else \
		echo "skipping lint: golangci-lint is not installed"; \
	fi

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
check: exclusions fmt-check vet build test deadcode lint

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
