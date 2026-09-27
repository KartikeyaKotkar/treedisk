package export

import (
	"disktree/pkg/removal"
	"strings"
	"testing"
)

func TestDeleteList(t *testing.T) {
	targets := []removal.Target{
		{Path: "/home/user/cache", IsDir: true, Bytes: 100},
		{Path: "/home/user/test.txt", IsDir: false, Bytes: 200},
	}
	out := DeleteList(targets)
	expected := "/home/user/cache\n/home/user/test.txt\n"
	if out != expected {
		t.Errorf("got %q, want %q", out, expected)
	}
}

func TestAgentPrompt(t *testing.T) {
	targets := []removal.Target{
		{Path: "/home/user/cache", IsDir: true, Bytes: 1000},
	}
	prompt := AgentPrompt(targets, "/home/user", nil)
	if !strings.Contains(prompt, "Please remove them for me, carefully:") {
		t.Errorf("prompt missing key instructions")
	}
	if !strings.Contains(prompt, "/home/user/cache") {
		t.Errorf("prompt missing target path")
	}
}
