package removal

import (
	"testing"
)

func TestBuildPlanDeduplication(t *testing.T) {
	root := "/home/user/project"
	targets := []Target{
		{Path: "/home/user/project/node_modules", IsDir: true, Bytes: 2000},
		{Path: "/home/user/project/node_modules/lodash", IsDir: true, Bytes: 500},
		{Path: "/home/user/project/build.log", IsDir: false, Bytes: 100},
		{Path: "/etc/passwd", IsDir: false, Bytes: 50}, // outside root & critical
	}

	plan := BuildPlan(targets, root)

	if len(plan.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(plan.Targets))
	}
	if len(plan.Covered) != 1 || plan.Covered[0].Path != "/home/user/project/node_modules/lodash" {
		t.Errorf("expected lodash to be covered, got %v", plan.Covered)
	}
	if len(plan.Blocked) != 1 || plan.Blocked[0].Path != "/etc/passwd" {
		t.Errorf("expected /etc/passwd to be blocked, got %v", plan.Blocked)
	}
}

func TestRefuseScannedRoot(t *testing.T) {
	root := "/home/user"
	targets := []Target{
		{Path: "/home/user", IsDir: true, Bytes: 100000},
	}
	plan := BuildPlan(targets, root)
	if len(plan.Targets) != 0 {
		t.Errorf("expected 0 targets when root requested, got %d", len(plan.Targets))
	}
	if len(plan.Blocked) != 1 {
		t.Errorf("expected root to be blocked, got %d", len(plan.Blocked))
	}
}
