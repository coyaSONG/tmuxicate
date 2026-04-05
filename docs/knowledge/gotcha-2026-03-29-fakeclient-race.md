---
title: FakeClient needs mutex for concurrent test access
category: gotcha
status: active
date: 2026-03-29
tags: [testing, race-condition, tmux, fake]
---

# FakeClient needs mutex for concurrent test access

## Symptom
`go test -race` failed on `TestDaemonNotifiesUnreadReceipt` with data race between daemon goroutine calling `FakeClient.SendKeys()` and test goroutine reading `FakeClient.SendKeysCalls`.

## Root Cause
`FakeClient` in `internal/tmux/fake.go` stored call records in plain slices with no synchronization. The daemon runs in a goroutine and calls methods on the fake client, while the test reads the call slices from the main goroutine.

## Fix / Workaround
Added `sync.Mutex` (`Mu` field) to `FakeClient`, locked in every method. Tests that read shared fields must also lock:

```go
fakeTmux.Mu.Lock()
notified := len(fakeTmux.SendKeysCalls) > 0
fakeTmux.Mu.Unlock()
```
