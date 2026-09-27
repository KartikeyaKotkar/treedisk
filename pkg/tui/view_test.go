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

func TestSidebarAndSorting(t *testing.T) {
	root := tree.NewDirectory("testroot")
	c1 := tree.NewEntry("zeta.txt", tree.File, 1000)
	c2 := tree.NewEntry("alpha.txt", tree.File, 5000)
	c3 := tree.NewEntry("beta.txt", tree.File, 2000)
	root.Children = append(root.Children, c1, c2, c3)
	tree.Aggregate(root, tree.Bytes)

	app := NewApp("/tmp/testroot", root, tree.Bytes, 3)
	app.SidebarOpen = true

	// Default sort by size (descending: alpha 5000, beta 2000, zeta 1000)
	app.refreshSidebar()
	if len(app.SidebarItems) != 3 {
		t.Fatalf("expected 3 sidebar items, got %d", len(app.SidebarItems))
	}
	if app.SidebarItems[0].Name != "alpha.txt" || app.SidebarItems[2].Name != "zeta.txt" {
		t.Errorf("unexpected sort order by size: %s, %s, %s",
			app.SidebarItems[0].Name, app.SidebarItems[1].Name, app.SidebarItems[2].Name)
	}

	// Sort by name (alphabetical: alpha, beta, zeta)
	app.HandleEvent(Event{Type: KeyChar, Char: 'N'})
	if app.SidebarItems[0].Name != "alpha.txt" || app.SidebarItems[1].Name != "beta.txt" || app.SidebarItems[2].Name != "zeta.txt" {
		t.Errorf("unexpected sort order by name: %s, %s, %s",
			app.SidebarItems[0].Name, app.SidebarItems[1].Name, app.SidebarItems[2].Name)
	}

	// Verify sidebar rendered in frame
	output := string(app.RenderFrame(100, 24))
	if !strings.Contains(output, "Contents · [Name]") {
		t.Errorf("frame missing sidebar header")
	}
	if !strings.Contains(output, "alpha.txt") || !strings.Contains(output, "zeta.txt") {
		t.Errorf("frame missing sidebar items")
	}
}

func TestPaneFocusAndKeyboardNav(t *testing.T) {
	root := tree.NewDirectory("testroot")
	d1 := tree.NewDirectory("subdir")
	d1.Children = append(d1.Children, tree.NewEntry("subfile.go", tree.File, 500))
	c1 := tree.NewEntry("file1.go", tree.File, 1024)
	root.Children = append(root.Children, d1, c1)
	tree.Aggregate(root, tree.Bytes)

	app := NewApp("/tmp/testroot", root, tree.Bytes, 3)
	app.ComputeLayout(100, 24)

	if app.Focus != FocusTreemap {
		t.Errorf("expected initial focus to be FocusTreemap")
	}

	// Tab to switch to sidebar
	app.HandleEvent(Event{Type: KeyTab})
	if app.Focus != FocusSidebar {
		t.Errorf("expected focus to switch to FocusSidebar on Tab")
	}

	// Navigate down in sidebar
	app.HandleEvent(Event{Type: KeyDown})
	if app.SidebarSelected != 1 {
		t.Errorf("expected SidebarSelected=1, got %d", app.SidebarSelected)
	}

	// Tab back to treemap
	app.HandleEvent(Event{Type: KeyTab})
	if app.Focus != FocusTreemap {
		t.Errorf("expected focus to switch back to FocusTreemap")
	}

	// Toggle sidebar with 's'
	app.HandleEvent(Event{Type: KeyChar, Char: 's'})
	if app.SidebarOpen {
		t.Errorf("expected SidebarOpen to be false after 's' toggle")
	}
}
