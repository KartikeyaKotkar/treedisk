package tui

import (
	"github.com/KartikeyaKotkar/treedisk/pkg/tree"
	"strings"
	"testing"
)

func TestAppRenderFrame(t *testing.T) {
	root := tree.NewDirectory("testroot")
	c1 := tree.NewEntry("file1.go", tree.File, 1024)
	c2 := tree.NewDirectory(".cache")
	c2.Reclaim = tree.Regenerable
	c2.Children = append(c2.Children, tree.NewEntry("cache.dat", tree.File, 2048))
	root.Children = append(root.Children, c1, c2)
	tree.Aggregate(root, tree.Bytes)

	app := NewApp("/tmp/testroot", root, tree.Bytes, 3)
	output := string(app.RenderFrame(80, 24))

	if !strings.Contains(output, "treedisk") {
		t.Errorf("rendered frame missing app title")
	}
	if !strings.Contains(output, "file1.go") {
		t.Errorf("rendered frame missing tile name file1.go")
	}
}

func TestAppNavigationAndMark(t *testing.T) {
	root := tree.NewDirectory("testroot")
	c1 := tree.NewEntry("file1.go", tree.File, 1024)
	root.Children = append(root.Children, c1)
	tree.Aggregate(root, tree.Bytes)

	app := NewApp("/tmp/testroot", root, tree.Bytes, 3)
	app.ComputeLayout(80, 24)

	if len(app.Tiles) == 0 {
		t.Fatal("expected tiles to be computed")
	}

	// Toggle mark with Space
	app.HandleEvent(Event{Type: KeySpace})
	if len(app.Marks) != 1 {
		t.Errorf("expected 1 mark, got %d", len(app.Marks))
	}

	// Toggle mark again to unmark
	app.HandleEvent(Event{Type: KeySpace})
	if len(app.Marks) != 0 {
		t.Errorf("expected 0 marks, got %d", len(app.Marks))
	}
}
