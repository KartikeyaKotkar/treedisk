package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestMainHelp(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run . --help failed: %v", err)
	}
	if !strings.Contains(string(out), "disktree — a treemap of what is using your disk") {
		t.Errorf("help output missing usage string")
	}
}

func TestMainOnce(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--once", "pkg")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run . --once pkg failed: %v\nOutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "disktree") {
		t.Errorf("treemap output missing disktree header")
	}
}
