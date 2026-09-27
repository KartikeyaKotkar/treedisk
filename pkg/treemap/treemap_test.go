package treemap

import (
	"github.com/KartikeyaKotkar/treedisk/pkg/tree"
	"math"
	"testing"
)

func TestSquarifyFillsArea(t *testing.T) {
	values := []float64{40.0, 30.0, 20.0, 5.0, 3.0, 2.0}
	area := NewRect(0, 0, 800, 500)
	rects := Squarify(values, area)

	var covered float32
	for _, r := range rects {
		covered += r.Area()
	}

	diff := math.Abs(float64(covered - area.Area()))
	if diff > 1.0 {
		t.Errorf("covered %.2f of %.2f", covered, area.Area())
	}
}

func TestLayoutHierarchy(t *testing.T) {
	root := tree.NewDirectory("root")
	c1 := tree.NewEntry("file1", tree.File, 1000)
	c2 := tree.NewEntry("file2", tree.File, 2000)
	root.Children = append(root.Children, c1, c2)
	tree.Aggregate(root, tree.Bytes)

	opts := DefaultLayoutOptions()
	tiles := Layout(root, nil, NewRect(0, 0, 100, 100), tree.Bytes, &opts)

	if len(tiles) != 2 {
		t.Fatalf("expected 2 tiles, got %d", len(tiles))
	}
}

// Constraint 1: Aspect Ratio Control
func TestAspectRatioControl(t *testing.T) {
	// Wide container: W=80, H=10 -> Wv = 40, Hv = 10 -> Wv >= Hv -> vertical split (slices across H)
	wideBox := TerminalRect{X: 0, Y: 0, W: 80, H: 10}
	wideRects := SquarifyTerminal([]float64{50, 50}, wideBox)
	if len(wideRects) != 2 {
		t.Fatalf("expected 2 rects, got %d", len(wideRects))
	}
	// With vertical splitting, strip width is partitioned along X and each spans full H
	if wideRects[0].H != 10 || wideRects[1].H != 10 {
		t.Errorf("expected vertical split with full height 10, got H=%d, %d", wideRects[0].H, wideRects[1].H)
	}
	if wideRects[0].W+wideRects[1].W != 80 {
		t.Errorf("expected total width 80, got %d", wideRects[0].W+wideRects[1].W)
	}

	// Tall container: W=10, H=50 -> Wv = 5, Hv = 50 -> Wv < Hv -> horizontal split (slices across W)
	tallBox := TerminalRect{X: 0, Y: 0, W: 10, H: 50}
	tallRects := SquarifyTerminal([]float64{50, 50}, tallBox)
	if len(tallRects) != 2 {
		t.Fatalf("expected 2 rects, got %d", len(tallRects))
	}
	if tallRects[0].W != 10 || tallRects[1].W != 10 {
		t.Errorf("expected horizontal split with full width 10, got W=%d, %d", tallRects[0].W, tallRects[1].W)
	}
	if tallRects[0].H+tallRects[1].H != 50 {
		t.Errorf("expected total height 50, got %d", tallRects[0].H+tallRects[1].H)
	}
}

// Constraint 2: Minimum Dimensions
func TestMinimumDimensions(t *testing.T) {
	root := tree.NewDirectory("root")
	// Name has 15 characters -> minW = 15 + 2 = 17
	longFile := tree.NewEntry("very_long_filename.txt", tree.File, 100)
	root.Children = append(root.Children, longFile)
	tree.Aggregate(root, tree.Bytes)

	// Box width 10 < 17 -> ShowBorderAndText must be false
	smallBox := TerminalRect{X: 0, Y: 0, W: 10, H: 10}
	tiles := LayoutTerminal(root, nil, smallBox, tree.Bytes, 2)
	if len(tiles) != 1 {
		t.Fatalf("expected 1 tile, got %d", len(tiles))
	}
	if tiles[0].ShowBorderAndText {
		t.Errorf("expected ShowBorderAndText to be false when W=10 < len(label)+2")
	}

	// Box height 1 < 2 -> ShowBorderAndText must be false
	shortBox := TerminalRect{X: 0, Y: 0, W: 50, H: 1}
	tilesShort := LayoutTerminal(root, nil, shortBox, tree.Bytes, 2)
	if len(tilesShort) != 1 {
		t.Fatalf("expected 1 tile, got %d", len(tilesShort))
	}
	if tilesShort[0].ShowBorderAndText {
		t.Errorf("expected ShowBorderAndText to be false when H=1 < 2")
	}

	// Sufficient dimensions W=50 >= 17 and H=5 >= 2 -> ShowBorderAndText must be true
	goodBox := TerminalRect{X: 0, Y: 0, W: 50, H: 5}
	tilesGood := LayoutTerminal(root, nil, goodBox, tree.Bytes, 2)
	if len(tilesGood) != 1 {
		t.Fatalf("expected 1 tile, got %d", len(tilesGood))
	}
	if !tilesGood[0].ShowBorderAndText {
		t.Errorf("expected ShowBorderAndText to be true when W=50 and H=5")
	}
}

// Constraint 3: Box-Drawing Margins
func TestBoxDrawingMargins(t *testing.T) {
	root := tree.NewDirectory("parent_dir")
	childDir := tree.NewDirectory("child_dir")
	childFile := tree.NewEntry("leaf.txt", tree.File, 100)
	childDir.Children = append(childDir.Children, childFile)
	root.Children = append(root.Children, childDir)
	tree.Aggregate(root, tree.Bytes)

	containerBox := TerminalRect{X: 0, Y: 0, W: 60, H: 20}
	tiles := LayoutTerminal(root, nil, containerBox, tree.Bytes, 3)

	// We should have parent child_dir tile and nested leaf.txt tile
	var parentTile, childTile *Tile
	for i := range tiles {
		if tiles[i].Label == "child_dir" {
			parentTile = &tiles[i]
		} else if tiles[i].Label == "leaf.txt" {
			childTile = &tiles[i]
		}
	}

	if parentTile == nil || childTile == nil {
		t.Fatalf("expected both parent and child tiles, found parent=%v, child=%v", parentTile, childTile)
	}

	// Verify strict 1-cell padding margin between parent border and child
	// Left margin: child X must be >= parent X + 2 (col X is border, col X+1 is padding)
	if childTile.Box.X < parentTile.Box.X+2 {
		t.Errorf("child X (%d) violates left margin (parent X: %d, min child X: %d)",
			childTile.Box.X, parentTile.Box.X, parentTile.Box.X+2)
	}

	// Right margin: child Right must be <= parent Right - 2 (col Right-1 is border, col Right-2 is padding)
	if childTile.Box.Right() > parentTile.Box.Right()-2 {
		t.Errorf("child Right (%d) violates right margin (parent Right: %d, max child Right: %d)",
			childTile.Box.Right(), parentTile.Box.Right(), parentTile.Box.Right()-2)
	}

	// Top margin: child Y must be >= parent Y + 3 (row Y is border, Y+1 is label, Y+2 is padding)
	if childTile.Box.Y < parentTile.Box.Y+3 {
		t.Errorf("child Y (%d) violates top margin (parent Y: %d, min child Y: %d)",
			childTile.Box.Y, parentTile.Box.Y, parentTile.Box.Y+3)
	}

	// Bottom margin: child Bottom must be <= parent Bottom - 2 (row Bottom-1 is border, Bottom-2 is padding)
	if childTile.Box.Bottom() > parentTile.Box.Bottom()-2 {
		t.Errorf("child Bottom (%d) violates bottom margin (parent Bottom: %d, max child Bottom: %d)",
			childTile.Box.Bottom(), parentTile.Box.Bottom(), parentTile.Box.Bottom()-2)
	}
}
