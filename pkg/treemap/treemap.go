package treemap

import (
	"github.com/KartikeyaKotkar/treedisk/pkg/tree"
	"math"
	"sort"
	"unicode/utf8"
)

// Rect represents an axis-aligned rectangle in float space.
type Rect struct {
	X float32
	Y float32
	W float32
	H float32
}

func NewRect(x, y, w, h float32) Rect {
	return Rect{X: x, Y: y, W: w, H: h}
}

func (r Rect) Right() float32 {
	return r.X + r.W
}

func (r Rect) Bottom() float32 {
	return r.Y + r.H
}

func (r Rect) Area() float32 {
	if r.W <= 0 || r.H <= 0 {
		return 0
	}
	return r.W * r.H
}

func (r Rect) Contains(x, y float32) bool {
	return x >= r.X && x < r.Right() && y >= r.Y && y < r.Bottom()
}

func (r Rect) Inset(padding float32) Rect {
	w := r.W - 2.0*padding
	h := r.H - 2.0*padding
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return Rect{X: r.X + padding, Y: r.Y + padding, W: w, H: h}
}

// TerminalRect represents an exact integer rectangle in terminal character grid cells.
type TerminalRect struct {
	X int
	Y int
	W int
	H int
}

func (tr TerminalRect) Right() int  { return tr.X + tr.W }
func (tr TerminalRect) Bottom() int { return tr.Y + tr.H }
func (tr TerminalRect) Area() int   { return tr.W * tr.H }
func (tr TerminalRect) Contains(x, y int) bool {
	return x >= tr.X && x < tr.Right() && y >= tr.Y && y < tr.Bottom()
}

// TileKind indicates what a tile stands for.
type TileKind uint8

const (
	TileNode TileKind = iota
	TileOthers
)

// Tile represents one box on the canvas mosaic.
type Tile struct {
	Kind              TileKind
	Crumbs            []int
	Count             int // for TileOthers
	Rect              Rect
	Box               TerminalRect
	Depth             uint32
	ShowBorderAndText bool   // false if min dimensions violated (width < len(label)+2 or height < 2)
	Label             string // text label
	SizeStr           string // formatted size label
}

// LayoutOptions configures treemap layout.
type LayoutOptions struct {
	MaxDepth     uint32
	Padding      float32
	PaddingOuter float32
	MinTile      float32
	MaxChildren  int
	Header       float32
	HeaderInner  float32
}

func DefaultLayoutOptions() LayoutOptions {
	return LayoutOptions{
		MaxDepth:     3,
		Padding:      0,
		PaddingOuter: 0,
		MinTile:      1.0,
		MaxChildren:  96,
		Header:       1.0,
		HeaderInner:  1.0,
	}
}

// LayoutTerminal computes squarified treemap layout directly in integer terminal cells.
// Constraints:
// 1. Aspect Ratio Control: dynamically picks horizontal or vertical splitting to keep container aspect ratios ~ 1:1.
// 2. Minimum Dimensions: if width < len(label)+2 or height < 2, omit internal borders and labels.
// 3. Box-Drawing Margins: strict 1-cell internal padding before rendering nested children.
func LayoutTerminal(
	root *tree.Node,
	rootCrumbs []int,
	box TerminalRect,
	metric tree.Metric,
	maxDepth uint32,
) []Tile {
	var tiles []Tile
	crumbs := append([]int(nil), rootCrumbs...)
	placeChildrenTerminal(root, box, metric, maxDepth, 0, crumbs, &tiles)
	return tiles
}

type rankedItem struct {
	index int
	node  *tree.Node
	value float64
	label string
}

func placeChildrenTerminal(
	parentNode *tree.Node,
	box TerminalRect,
	metric tree.Metric,
	maxDepth uint32,
	depth uint32,
	crumbs []int,
	out *[]Tile,
) {
	if len(parentNode.Children) == 0 || box.W <= 0 || box.H <= 0 {
		return
	}

	var ranked []rankedItem
	for idx, child := range parentNode.Children {
		val := float64(child.Value(metric))
		if val > 0 {
			ranked = append(ranked, rankedItem{
				index: idx,
				node:  child,
				value: val,
				label: child.Name,
			})
		}
	}
	if len(ranked) == 0 {
		return
	}

	// Sort largest first
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].value > ranked[j].value
	})

	const maxChildren = 96
	kept := len(ranked)
	if kept > maxChildren {
		kept = maxChildren
	}

	active := ranked[:kept]
	tailCount := len(ranked) - kept
	var tailSum float64
	if tailCount > 0 {
		for i := kept; i < len(ranked); i++ {
			tailSum += ranked[i].value
		}
	}

	// Prepare values array for Squarify
	nItems := len(active)
	if tailCount > 0 {
		nItems++
	}
	values := make([]float64, nItems)
	for i := 0; i < len(active); i++ {
		values[i] = active[i].value
	}
	if tailCount > 0 {
		values[len(active)] = tailSum
	}

	// Partition box using Squarified Treemap algorithm with aspect ratio control
	rectBoxes := SquarifyTerminal(values, box)

	for slot, itemBox := range rectBoxes {
		if itemBox.W <= 0 || itemBox.H <= 0 {
			continue
		}

		if slot >= len(active) {
			// Tail Others tile
			label := "Others"
			minW := utf8.RuneCountInString(label) + 2
			show := itemBox.W >= minW && itemBox.H >= 2

			t := Tile{
				Kind:              TileOthers,
				Crumbs:            append([]int(nil), crumbs...),
				Count:             tailCount,
				Box:               itemBox,
				Rect:              NewRect(float32(itemBox.X), float32(itemBox.Y), float32(itemBox.W), float32(itemBox.H)),
				Depth:             depth,
				ShowBorderAndText: show,
				Label:             label,
			}
			*out = append(*out, t)
			continue
		}

		item := active[slot]
		child := item.node
		childCrumbs := append(append([]int(nil), crumbs...), item.index)

		// Constraint 2: Minimum Dimensions Check
		// If width < (len(label) + 2) or height < 2 rows, omit internal borders and labels entirely
		label := item.label
		minW := utf8.RuneCountInString(label) + 2
		showBorderAndLabel := (itemBox.W >= minW && itemBox.H >= 2)

		t := Tile{
			Kind:              TileNode,
			Crumbs:            childCrumbs,
			Box:               itemBox,
			Rect:              NewRect(float32(itemBox.X), float32(itemBox.Y), float32(itemBox.W), float32(itemBox.H)),
			Depth:             depth,
			ShowBorderAndText: showBorderAndLabel,
			Label:             label,
		}
		*out = append(*out, t)

		// Constraint 3: Box-Drawing Margins
		// Add strict 1-cell internal padding before rendering nested children so text doesn't overlap container borders.
		if showBorderAndLabel && child.IsDir() && len(child.Children) > 0 && depth+1 < maxDepth {
			// Parent has:
			// row Y: top border
			// row Y+1: label text
			// row Y+2: strict 1-cell padding margin
			// row Y+H-2: strict 1-cell padding margin
			// row Y+H-1: bottom border
			// col X: left border
			// col X+1: strict 1-cell padding margin
			// col X+W-2: strict 1-cell padding margin
			// col X+W-1: right border
			nestedX := itemBox.X + 2
			nestedY := itemBox.Y + 3
			nestedW := itemBox.W - 4
			nestedH := itemBox.H - 5

			// Only subdivide if nested space can fit at least one child with minimum dimensions
			if nestedW >= 6 && nestedH >= 2 {
				nestedBox := TerminalRect{
					X: nestedX,
					Y: nestedY,
					W: nestedW,
					H: nestedH,
				}
				placeChildrenTerminal(child, nestedBox, metric, maxDepth, depth+1, childCrumbs, out)
			}
		}
	}
}

// SquarifyTerminal implements Bruls, Huizing, van Wijk Squarified Treemap layout in terminal characters.
// Constraint 1: Aspect Ratio Control:
// Terminal cells have ~1:2 width:height character ratio.
// Visual width Wv = W * 0.5. Visual height Hv = H.
// When Wv >= Hv: split vertically (vertical strip stacked top-to-bottom).
// When Wv < Hv: split horizontally (horizontal strip stacked left-to-right).
// Dynamically chooses split direction and optimizes worst visual aspect ratio to stay as close to 1:1 as possible.
func SquarifyTerminal(values []float64, box TerminalRect) []TerminalRect {
	rects := make([]TerminalRect, len(values))
	if len(values) == 0 || box.W <= 0 || box.H <= 0 {
		return rects
	}

	var total float64
	for _, v := range values {
		if v > 0 {
			total += v
		}
	}
	if total <= 0 {
		return rects
	}

	remX := box.X
	remY := box.Y
	remW := box.W
	remH := box.H
	remVal := total

	start := 0
	for start < len(values) && remW > 0 && remH > 0 && remVal > 0 {
		// Terminal character cell aspect: 1 char is ~0.5 as wide as 1 row tall
		visW := float64(remW) * 0.5
		visH := float64(remH)

		// Constraint 1: Aspect Ratio Control
		// Choose split direction along the shorter visual side
		splitVertical := (visW >= visH)
		var sideLength float64
		if splitVertical {
			sideLength = visH
		} else {
			sideLength = visW
		}

		end := start + 1
		rowSum := values[start]
		rowWorst := worstVisualRatio(values[start:end], rowSum, remVal, visW*visH, sideLength)

		for end < len(values) {
			candSum := rowSum + values[end]
			candWorst := worstVisualRatio(values[start:end+1], candSum, remVal, visW*visH, sideLength)
			if candWorst > rowWorst {
				break
			}
			rowSum = candSum
			rowWorst = candWorst
			end++
		}

		isLastStrip := (end >= len(values))

		// Partition exact integer cells without gaps or overlaps
		if splitVertical {
			// Slicing vertical strip of width stripW across full remH
			var stripW int
			if isLastStrip {
				stripW = remW
			} else {
				stripW = int(math.Round((rowSum / remVal) * float64(remW)))
				if stripW < 1 {
					stripW = 1
				}
				if stripW > remW {
					stripW = remW
				}
			}

			// Stack items vertically inside strip
			currY := remY
			var cumVal float64
			for i := start; i < end; i++ {
				cumVal += values[i]
				var nextY int
				if i == end-1 {
					nextY = remY + remH
				} else {
					nextY = remY + int(math.Round((cumVal/rowSum)*float64(remH)))
					if nextY < currY {
						nextY = currY
					}
					if nextY > remY+remH {
						nextY = remY + remH
					}
				}
				itemH := nextY - currY
				rects[i] = TerminalRect{
					X: remX,
					Y: currY,
					W: stripW,
					H: itemH,
				}
				currY = nextY
			}

			remX += stripW
			remW -= stripW
			remVal -= rowSum
		} else {
			// Slicing horizontal strip of height stripH across full remW
			var stripH int
			if isLastStrip {
				stripH = remH
			} else {
				stripH = int(math.Round((rowSum / remVal) * float64(remH)))
				if stripH < 1 {
					stripH = 1
				}
				if stripH > remH {
					stripH = remH
				}
			}

			// Stack items horizontally inside strip
			currX := remX
			var cumVal float64
			for i := start; i < end; i++ {
				cumVal += values[i]
				var nextX int
				if i == end-1 {
					nextX = remX + remW
				} else {
					nextX = remX + int(math.Round((cumVal/rowSum)*float64(remW)))
					if nextX < currX {
						nextX = currX
					}
					if nextX > remX+remW {
						nextX = remX + remW
					}
				}
				itemW := nextX - currX
				rects[i] = TerminalRect{
					X: currX,
					Y: remY,
					W: itemW,
					H: stripH,
				}
				currX = nextX
			}

			remY += stripH
			remH -= stripH
			remVal -= rowSum
		}

		start = end
	}

	return rects
}

// worstVisualRatio evaluates Bruls et al. aspect ratio in visual space.
func worstVisualRatio(
	rowValues []float64,
	rowSum float64,
	remVal float64,
	totalVisualArea float64,
	sideLength float64,
) float64 {
	if rowSum <= 0 || sideLength <= 0 || remVal <= 0 || totalVisualArea <= 0 {
		return math.Inf(1)
	}

	// Visual strip thickness
	stripVisualArea := (rowSum / remVal) * totalVisualArea
	stripThickness := stripVisualArea / sideLength
	if stripThickness <= 0 {
		return math.Inf(1)
	}

	worst := 0.0
	for _, v := range rowValues {
		if v <= 0 {
			continue
		}
		itemVisualArea := (v / remVal) * totalVisualArea
		itemLength := itemVisualArea / stripThickness
		if itemLength <= 0 {
			continue
		}
		ratio := math.Max(stripThickness/itemLength, itemLength/stripThickness)
		if ratio > worst {
			worst = ratio
		}
	}
	return worst
}

// Layout provides compatibility with standard float Rect.
func Layout(
	root *tree.Node,
	rootCrumbs []int,
	area Rect,
	metric tree.Metric,
	opts *LayoutOptions,
) []Tile {
	maxDepth := uint32(3)
	if opts != nil && opts.MaxDepth > 0 {
		maxDepth = opts.MaxDepth
	}
	box := TerminalRect{
		X: int(area.X),
		Y: int(area.Y),
		W: int(area.W),
		H: int(area.H),
	}
	if box.W <= 0 {
		box.W = 80
	}
	if box.H <= 0 {
		box.H = 24
	}
	return LayoutTerminal(root, rootCrumbs, box, metric, maxDepth)
}

// Squarify provides compatibility with float slice areas.
func Squarify(values []float64, area Rect) []Rect {
	box := TerminalRect{
		X: int(area.X),
		Y: int(area.Y),
		W: int(area.W),
		H: int(area.H),
	}
	tBoxes := SquarifyTerminal(values, box)
	rects := make([]Rect, len(tBoxes))
	for i, b := range tBoxes {
		rects[i] = NewRect(float32(b.X), float32(b.Y), float32(b.W), float32(b.H))
	}
	return rects
}

// Hit finds the deepest tile containing terminal coordinate (x, y).
func Hit(tiles []Tile, x, y int) *Tile {
	for i := len(tiles) - 1; i >= 0; i-- {
		if tiles[i].Box.Contains(x, y) {
			return &tiles[i]
		}
	}
	return nil
}
