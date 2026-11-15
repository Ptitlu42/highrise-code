//go:build unix

package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func write_fake_picker(t *testing.T, body string) picker_command {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake_picker")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return picker_command{path: path}
}

func TestRun_picker_selected(t *testing.T) {
	root := make_project(t)
	got, err := run_picker(context.Background(), write_fake_picker(t, "printf '%s\\n' '"+root+"'"))
	if err != nil || got != root {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestRun_picker_cancelled_exit_one(t *testing.T) {
	_, err := run_picker(context.Background(), write_fake_picker(t, "exit 1"))
	if !errors.Is(err, Err_pick_cancelled) {
		t.Fatalf("got %v", err)
	}
}

func TestRun_picker_cancelled_empty_output(t *testing.T) {
	_, err := run_picker(context.Background(), write_fake_picker(t, "exit 0"))
	if !errors.Is(err, Err_pick_cancelled) {
		t.Fatalf("got %v", err)
	}
}

func TestRun_picker_failure(t *testing.T) {
	_, err := run_picker(context.Background(), write_fake_picker(t, "echo boom >&2; exit 2"))
	if err == nil || errors.Is(err, Err_pick_cancelled) {
		t.Fatalf("got %v", err)
	}
}

func TestRun_picker_rejects_file(t *testing.T) {
	root := make_project(t)
	_, err := run_picker(context.Background(), write_fake_picker(t, "printf '%s' '"+root+"/main.go'"))
	if err == nil {
		t.Fatal("file accepted")
	}
}

func TestRun_picker_context_cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := run_picker(ctx, write_fake_picker(t, "sleep 5"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
