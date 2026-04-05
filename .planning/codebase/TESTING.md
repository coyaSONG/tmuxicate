# Testing Patterns

**Analysis Date:** 2026-04-05

## Test Framework

**Runner:**
- Go standard library `testing` package.
- Primary commands live in `Makefile`: `test`, `test-integration`, and `ci`.
- Integration tests use the `integration` build tag, demonstrated by `internal/tmux/real_test.go`.

**Assertion Library:**
- No third-party assertion library is used.
- Assertions are written with `t.Fatal`, `t.Fatalf`, `t.Error`, and direct value comparisons in files such as `internal/config/loader_test.go`, `internal/mailbox/store_test.go`, and `internal/adapter/factory_test.go`.

**Run Commands:**
```bash
make test                 # go test ./... -count=1 -race
make test-integration     # go test ./... -count=1 -race -tags=integration
go test ./... -cover      # ad hoc package coverage
```

## Test File Organization

**Location:**
- Tests are co-located with the package they exercise.
- Current test files are `internal/adapter/factory_test.go`, `internal/adapter/generic_test.go`, `internal/config/loader_test.go`, `internal/mailbox/store_test.go`, `internal/protocol/protocol_test.go`, `internal/runtime/daemon_test.go`, `internal/tmux/fake_test.go`, and `internal/tmux/real_test.go`.

**Naming:**
- Use `*_test.go` and keep tests in the same package as production code, not an external `_test` package.
- Test names follow `TestXxx` with behavior-oriented suffixes, for example `TestCreateMessage_AtomicVisibility` in `internal/mailbox/store_test.go` and `TestProbe_Ready_WhenQuiet` in `internal/adapter/generic_test.go`.

**Structure:**
```text
internal/<package>/<file>_test.go
```

## Test Structure

**Suite Organization:**
```go
func TestCreateMessage_AtomicVisibility(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())
	env, body := testEnvelope(142)

	if err := store.CreateMessage(&env, body); err != nil {
		t.Fatalf("CreateMessage() unexpected error: %v", err)
	}
}
```

**Patterns:**
- Call `t.Parallel()` at the top of almost every test. This is consistent across `internal/config/loader_test.go`, `internal/mailbox/store_test.go`, `internal/adapter/*.go`, `internal/protocol/protocol_test.go`, and `internal/tmux/*.go`.
- Use `t.TempDir()` for filesystem isolation. This is the dominant setup pattern in `internal/config/loader_test.go`, `internal/mailbox/store_test.go`, and `internal/runtime/daemon_test.go`.
- Use explicit cleanup when external resources are involved. `t.Cleanup(...)` appears in `internal/tmux/real_test.go` to kill the tmux session.
- Prefer direct setup over shared fixtures when the scenario is small. Tests typically construct configs, envelopes, and receipts inline.

## Mocking

**Framework:** Hand-rolled fakes and concrete test helpers

**Patterns:**
```go
client := tmux.NewFakeClient()
client.PaneCaptures["%1"] = "❯\n"

a, err := NewClaudeCodeAdapter(client, "%1")
if err != nil {
	t.Fatalf("NewClaudeCodeAdapter() unexpected error: %v", err)
}
```

**What to Mock:**
- Mock tmux interactions with `internal/tmux/FakeClient` from `internal/tmux/fake.go`.
- Preload fake pane state through `PaneCaptures`, then assert side effects by inspecting call slices like `SendKeysCalls`, `SetBufferCalls`, and `PasteBufferCalls`.
- Use temporary directories and real filesystem I/O for mailbox/config packages instead of abstracting the filesystem away.

**What NOT to Mock:**
- Do not mock protocol validation or YAML encoding. Tests in `internal/mailbox/store_test.go` and `internal/config/loader_test.go` exercise real serialization paths.
- Do not add a third-party mocking framework unless a new dependency boundary cannot be modeled with a small fake like `tmux.FakeClient`.

## Fixtures and Factories

**Test Data:**
```go
func testEnvelope(seq int64) (protocol.Envelope, []byte) {
	body := []byte(fmt.Sprintf("# Message %d\n\nHello.\n", seq))
	sum := sha256.Sum256(body)
	now := time.Now().UTC()

	return protocol.Envelope{ ... }, body
}
```

**Location:**
- Package-local helpers live in the same test file when only one package needs them, such as `testEnvelope` and `testReceipt` in `internal/mailbox/store_test.go` and `writeTestFile` in `internal/config/loader_test.go`.
- There is no shared `internal/testutil` package in active use for current tests.

## Coverage

**Requirements:** None enforced

**Observed shape:**
- `go test ./... -count=1 -race` passes for all current unit-test packages.
- `cmd/tmuxicate` has no test files and showed `0.0%` coverage from `go test ./... -cover`.
- `internal/session` has no test files and showed `0.0%` coverage across files such as `internal/session/up.go`, `internal/session/status.go`, `internal/session/task_cmd.go`, and `internal/session/log_view.go`.
- Core lower-level packages have meaningful but incomplete coverage: `internal/adapter` is mostly covered, `internal/config` is partially covered, `internal/mailbox` covers core happy paths and concurrency, `internal/protocol` covers validation rules, `internal/runtime` has targeted daemon coverage, and `internal/tmux` coverage is split between fake-client unit tests and tagged real-tmux integration tests.
- Overall statement coverage from the sampled run was `24.5%`.

**View Coverage:**
```bash
go test ./... -coverprofile=/tmp/tmuxicate.cover.out
go tool cover -func=/tmp/tmuxicate.cover.out
```

## Test Types

**Unit Tests:**
- Most tests are package-level unit tests that exercise real code paths with isolated temp directories or fake tmux clients.
- Representative unit suites are `internal/config/loader_test.go`, `internal/mailbox/store_test.go`, `internal/protocol/protocol_test.go`, `internal/adapter/generic_test.go`, and `internal/tmux/fake_test.go`.

**Integration Tests:**
- Real tmux integration is isolated behind `//go:build integration` in `internal/tmux/real_test.go`.
- The integration test checks actual `tmux` session lifecycle, pane options, `SendKeys`, `CapturePane`, and `ListPanes`.
- The test skips when `tmux` is not installed by using `exec.LookPath("tmux")`.

**E2E Tests:**
- Not used.
- There are no full CLI black-box tests for flows like `tmuxicate up`, `tmuxicate send`, `tmuxicate status`, or `tmuxicate down` through `cmd/tmuxicate/main.go`.

## Common Patterns

**Async Testing:**
```go
deadline := time.Now().Add(2 * time.Second)
for time.Now().Before(deadline) {
	got, err := store.ReadReceipt(agentName, env.ID)
	if err != nil {
		t.Fatal(err)
	}
	...
	time.Sleep(20 * time.Millisecond)
}
```
- Polling with deadlines is used in `internal/runtime/daemon_test.go` to wait for background notification side effects.
- Concurrency tests rely on `sync.WaitGroup` and buffered channels, as in `internal/mailbox/store_test.go`.

**Error Testing:**
```go
_, err := Load(cfgPath)
if err == nil {
	t.Fatal("Load() expected error, got nil")
}
if !strings.Contains(err.Error(), "session.name") {
	t.Fatalf("Load() error = %q, want session.name failure", err)
}
```
- Error assertions commonly verify both presence of an error and a stable substring, especially in `internal/config/loader_test.go`.
- Negative-path validation tests are concentrated in `internal/protocol/protocol_test.go` and `internal/adapter/generic_test.go`.

## Current Gaps To Preserve Awareness

- When adding tests around user-facing flows, start with `internal/session/*.go`; it is the largest uncovered package and contains orchestration-heavy code that drives runtime behavior.
- When adding CLI tests, target `cmd/tmuxicate/main.go` through Cobra command execution rather than only testing helpers indirectly.
- When changing `internal/runtime/daemon.go`, preserve the existing fake-based strategy from `internal/runtime/daemon_test.go` instead of introducing live tmux dependencies.
- When changing `internal/tmux/real.go`, keep coverage split: unit coverage through `internal/tmux/fake.go` patterns and integration coverage behind the `integration` build tag.

---

*Testing analysis: 2026-04-05*
