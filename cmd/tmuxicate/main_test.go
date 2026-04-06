package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestBlockerResolveCommandRequiresAction(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"blocker",
		"resolve",
		"run_000000000001",
		"task_000000000001",
		"--state-dir",
		t.TempDir(),
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected blocker resolve without --action to fail")
	}
	if !strings.Contains(err.Error(), `required flag(s) "action" not set`) {
		t.Fatalf("error = %q, want required action flag", err)
	}
}
