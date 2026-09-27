package tree

import (
	"testing"
)

func TestTreeAggregationAndSorting(t *testing.T) {
	root := NewDirectory("root")
	child1 := NewDirectory("dir1")
	child2 := NewEntry("file1.txt", File, 500)
	child3 := NewEntry("file2.txt", File, 1500)

	child1.Children = append(child1.Children, child2)
	root.Children = append(root.Children, child1, child3)

	Aggregate(root, Bytes)

	if root.Bytes != 2000 {
		t.Fatalf("expected 2000 bytes, got %d", root.Bytes)
	}
	if root.Files != 2 {
		t.Fatalf("expected 2 files, got %d", root.Files)
	}
	if root.Dirs != 2 { // root and dir1
		t.Fatalf("expected 2 dirs, got %d", root.Dirs)
	}

	// Root's largest child should be file2.txt (1500 bytes)
	if root.Children[0].Name != "file2.txt" {
		t.Errorf("expected first child file2.txt, got %s", root.Children[0].Name)
	}
}

func TestHardlinkDedup(t *testing.T) {
	root := NewDirectory("root")
	f1 := NewEntry("link1", File, 1000)
	f1.Dev = 1
	f1.Ino = 42
	f1.HasInode = true

	f2 := NewEntry("link2", File, 1000)
	f2.Dev = 1
	f2.Ino = 42
	f2.HasInode = true

	root.Children = append(root.Children, f1, f2)
	seen := NewSeen()
	AggregateDeduped(root, Bytes, seen)

	// One file charged, other deduplicated to 0
	if root.Bytes != 1000 {
		t.Errorf("expected deduped 1000 bytes, got %d", root.Bytes)
	}
	if root.Files != 1 {
		t.Errorf("expected deduped 1 file, got %d", root.Files)
	}
}

func TestFind(t *testing.T) {
	root := NewDirectory("root")
	sub := NewDirectory("sub")
	target := NewEntry("target_file.go", File, 100)
	sub.Children = append(sub.Children, target)
	root.Children = append(root.Children, sub)

	crumbs, found := root.Find("target_file")
	if found == nil {
		t.Fatal("expected to find node")
	}
	if len(crumbs) != 2 || crumbs[0] != 0 || crumbs[1] != 0 {
		t.Errorf("unexpected crumbs: %v", crumbs)
	}
}
