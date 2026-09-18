package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileIsolation(t *testing.T) {
	a, err := loadAdapter("adapters/go1-gazebo-classic.json")
	if err != nil {
		t.Fatal(err)
	}
	env, err := a.environment([]string{"KEEP=ok", "GO1SIM_SHARED_ODOM=1", "GO1SIM_SHARED_IMU=1", "GO1SIM_RELAY_MODE=1", "GO1SIM_STATE_BOARD=old"}, "motor", "new")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(env, "\n")
	for _, bad := range []string{"SHARED_ODOM", "SHARED_IMU", "RELAY_MODE", "=old"} {
		if strings.Contains(text, bad) {
			t.Fatal(text)
		}
	}
	for _, want := range []string{"KEEP=ok", "GO1SIM_SHARED_STATE=1", "GO1SIM_STATE_BOARD=new"} {
		if !strings.Contains(text, want) {
			t.Fatal(text)
		}
	}
	if _, err = a.environment(nil, "typo", ""); err == nil {
		t.Fatal("accepted unknown profile")
	}
}
func TestUnknownManifestFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "adapter.json")
	if err := os.WriteFile(path, []byte(`{"schema":1,"surprise":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadAdapter(path); err == nil {
		t.Fatal("accepted typo")
	}
}
func TestRunCleansBoardAndPreservesFailure(t *testing.T) {
	dir := t.TempDir()
	backend := filepath.Join(dir, "backend")
	if err := os.WriteFile(backend, []byte("#!/bin/sh\n: > \"$2\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	a, err := loadAdapter("adapters/go1-gazebo-classic.json")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "shm")
	if err = os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	err = runWithBoard(a, "motor", backend, root, []string{"sh", "-c", `test -f "$GO1SIM_STATE_BOARD" || exit 90; test "$GO1SIM_SHARED_STATE" = 1 || exit 91; exit 7`})
	e, ok := err.(*exec.ExitError)
	if !ok || e.ExitCode() != 7 {
		t.Fatalf("wrong error: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("board leaked: %v %v", entries, err)
	}
}
