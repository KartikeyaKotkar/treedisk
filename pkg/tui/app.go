package tui

import (
	"bytes"
	"disktree/pkg/export"
	"disktree/pkg/filter"
	"disktree/pkg/removal"
	"disktree/pkg/size"
	"disktree/pkg/space"
	"disktree/pkg/tree"
	"disktree/pkg/treemap"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type Cell struct {
	Char rune
	Fg   string
	Bg   string
	Bold bool
}

type ScreenBuffer struct {
	Width  int
	Height int
	Cells  [][]Cell
}

func NewScreenBuffer(w, h int) *ScreenBuffer {
	cells := make([][]Cell, h)
	for y := 0; y < h; y++ {
		cells[y] = make([]Cell, w)
		for x := 0; x < w; x++ {
			cells[y][x] = Cell{Char: ' ', Fg: Reset, Bg: Reset}
		}
	}
	return &ScreenBuffer{Width: w, Height: h, Cells: cells}
}

func (sb *ScreenBuffer) Set(x, y int, ch rune, fg, bg string, bold bool) {
	if x >= 0 && x < sb.Width && y >= 0 && y < sb.Height {
		sb.Cells[y][x] = Cell{Char: ch, Fg: fg, Bg: bg, Bold: bold}
	}
}

func (sb *ScreenBuffer) DrawString(x, y int, str string, fg, bg string, bold bool) {
	if y < 0 || y >= sb.Height {
		return
	}
	currX := x
	for _, ch := range str {
		if currX >= sb.Width {
			break
		}
		if currX >= 0 {
			sb.Cells[y][currX] = Cell{Char: ch, Fg: fg, Bg: bg, Bold: bold}
		}
		currX++
	}
}

func (sb *ScreenBuffer) Render() []byte {
	var buf bytes.Buffer
	buf.WriteString("\x1b[H") // move cursor home

	lastFg := ""
	lastBg := ""
	lastBold := false

	for y := 0; y < sb.Height; y++ {
		for x := 0; x < sb.Width; x++ {
			c := sb.Cells[y][x]

			// Attribute changes
			if c.Bold != lastBold || c.Fg != lastFg || c.Bg != lastBg {
				buf.WriteString(Reset)
				if c.Bold {
					buf.WriteString(Bold)
				}
				if c.Fg != "" && c.Fg != Reset {
					buf.WriteString(c.Fg)
				}
				if c.Bg != "" && c.Bg != Reset {
					buf.WriteString(c.Bg)
				}
				lastBold = c.Bold
				lastFg = c.Fg
				lastBg = c.Bg
			}
			buf.WriteRune(c.Char)
		}
		if y < sb.Height-1 {
			buf.WriteString("\r\n")
		}
	}
	buf.WriteString(Reset)
	return buf.Bytes()
}

type Mode uint8

const (
	ModeNormal Mode = iota
	ModeReview
	ModeHelp
	ModeSearch
)

// App is the interactive terminal treemap application.
type App struct {
	Term         *Terminal
	RootPath     string
	RootNode     *tree.Node
	CurrentNode  *tree.Node
	Crumbs       []int
	SelectedTile int
	Tiles        []treemap.Tile
	Marks        map[string]removal.Target
	Metric       tree.Metric
	Depth        uint32
	SearchQuery  string
	FilterResult *filter.Matches
	Mode         Mode
	SpaceInfo    space.SpaceInfo
	DeviceName   string
	StatusMsg    string
	Running      bool
}

func NewApp(rootPath string, rootNode *tree.Node, metric tree.Metric, depth uint32) *App {
	sp, _ := space.GetSpaceInfo(rootPath)
	dev := space.DeviceFor(rootPath)

	return &App{
		RootPath:     rootPath,
		RootNode:     rootNode,
		CurrentNode:  rootNode,
		Crumbs:       nil,
		SelectedTile: 0,
		Marks:        make(map[string]removal.Target),
		Metric:       metric,
		Depth:        depth,
		Mode:         ModeNormal,
		SpaceInfo:    sp,
		DeviceName:   dev,
		Running:      true,
	}
}

// ComputeLayout calculates tiles fitting terminal dimensions.
func (a *App) ComputeLayout(cols, rows int) {
	// Top header: 2 rows
	// Bottom footer: 4 rows
	mapH := rows - 6
	if mapH < 4 {
		mapH = 4
	}
	mapW := cols
	if mapW < 10 {
		mapW = 10
	}

	box := treemap.TerminalRect{
		X: 0,
		Y: 2,
		W: mapW,
		H: mapH,
	}

	a.Tiles = treemap.LayoutTerminal(a.CurrentNode, nil, box, a.Metric, a.Depth)

	if a.SelectedTile >= len(a.Tiles) {
		a.SelectedTile = len(a.Tiles) - 1
	}
	if a.SelectedTile < 0 && len(a.Tiles) > 0 {
		a.SelectedTile = 0
	}
}

// RenderFrame renders a single frame to the screen buffer.
func (a *App) RenderFrame(cols, rows int) []byte {
	a.ComputeLayout(cols, rows)
	sb := NewScreenBuffer(cols, rows)

	// Header Line 0: Breadcrumb trail
	trail := a.buildTrail()
	sb.DrawString(1, 0, "disktree", HighlightBorder, Reset, true)
	sb.DrawString(10, 0, "· "+trail, FgBrightWhite, Reset, false)

	// Header Line 1: Summary metrics
	metricStr := a.Metric.Label()
	summary := fmt.Sprintf("Total: %s (%s files) | Metric: [%s] | Depth: [%d]",
		size.HumanBytes(a.CurrentNode.Bytes),
		size.HumanCount(a.CurrentNode.Files),
		metricStr,
		a.Depth,
	)
	if a.FilterResult != nil {
		summary += fmt.Sprintf(" | Filter: %q (%d matches)", a.SearchQuery, a.FilterResult.Count)
	}
	sb.DrawString(1, 1, summary, FgYellow, Reset, false)

	// Draw treemap grid
	mapOffsetY := 2
	mapH := rows - 6

	for idx, tile := range a.Tiles {
		isSelected := (idx == a.SelectedTile)
		box := tile.Box

		x0 := box.X
		y0 := box.Y
		w := box.W
		h := box.H
		x1 := x0 + w
		y1 := y0 + h

		if x0 < 0 {
			x0 = 0
		}
		if x1 > cols {
			x1 = cols
		}
		if y0 < mapOffsetY {
			y0 = mapOffsetY
		}
		if y1 > mapOffsetY+mapH {
			y1 = mapOffsetY + mapH
		}

		if x1 <= x0 || y1 <= y0 {
			continue
		}

		// Resolve node
		var targetNode *tree.Node
		var targetPath string
		isOthers := (tile.Kind == treemap.TileOthers)

		if !isOthers {
			targetNode = a.CurrentNode.Resolve(tile.Crumbs)
			if targetNode != nil {
				targetPath = a.buildPath(tile.Crumbs)
			}
		}

		// Colors
		cat := tree.OtherCat
		reclaim := tree.ReclaimNone
		if targetNode != nil {
			cat = targetNode.Category
			reclaim = targetNode.Reclaim
		}

		fg := CategoryColor(cat)
		bg := CategoryBg(cat)
		if isSelected {
			bg = HighlightBg
		}

		_, isMarked := a.Marks[targetPath]

		// Structural Constraint 2:
		// If a box calculates to a width smaller than (len(label) + 2) or height < 2 rows,
		// omit internal borders and labels entirely to prevent text clipping.
		if !tile.ShowBorderAndText {
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					ch := ' '
					if reclaim != tree.ReclaimNone {
						ch = '░'
					}
					if isSelected {
						sb.Set(x, y, ch, HighlightBorder, HighlightBg, true)
					} else {
						sb.Set(x, y, ch, fg, bg, false)
					}
				}
			}
			continue
		}

		// Draw border & interior for containers meeting minimum dimensions
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				isBorder := (x == x0 || x == x1-1 || y == y0 || y == y1-1)
				if isBorder {
					var ch rune
					if isSelected {
						if x == x0 && y == y0 {
							ch = '╔'
						} else if x == x1-1 && y == y0 {
							ch = '╗'
						} else if x == x0 && y == y1-1 {
							ch = '╚'
						} else if x == x1-1 && y == y1-1 {
							ch = '╝'
						} else if y == y0 || y == y1-1 {
							ch = '═'
						} else {
							ch = '║'
						}
						sb.Set(x, y, ch, HighlightBorder, bg, true)
					} else {
						if x == x0 && y == y0 {
							ch = '┌'
						} else if x == x1-1 && y == y0 {
							ch = '┐'
						} else if x == x0 && y == y1-1 {
							ch = '└'
						} else if x == x1-1 && y == y1-1 {
							ch = '┘'
						} else if y == y0 || y == y1-1 {
							ch = '─'
						} else {
							ch = '│'
						}
						sb.Set(x, y, ch, fg, bg, false)
					}
				} else {
					ch := ' '
					if reclaim != tree.ReclaimNone {
						ch = '░' // hatch for reclaimable space
					}
					sb.Set(x, y, ch, fg, bg, false)
				}
			}
		}

		// Draw unclipped label & optional size
		interiorW := (x1 - x0) - 2
		title := tile.Label
		if isOthers {
			title = fmt.Sprintf("Others (%d)", tile.Count)
		} else if isMarked {
			title = "[X] " + title
		}

		titleFg := FgBrightWhite
		if isMarked {
			titleFg = FgBrightRed
		}

		// Draw label at row y0+1, col x0+1
		sb.DrawString(x0+1, y0+1, title, titleFg, bg, isSelected)

		// Size string
		if (y1-y0) >= 3 && targetNode != nil {
			var sizeStr string
			if a.Metric == tree.Files {
				sizeStr = size.HumanCount(targetNode.Files)
			} else {
				sizeStr = size.HumanBytesShort(targetNode.Bytes)
			}
			if utf8.RuneCountInString(sizeStr) <= interiorW {
				sb.DrawString(x0+1, y0+2, sizeStr, FgBrightYellow, bg, false)
			}
		}
	}

	// Bottom Footer
	footY := rows - 4

	// Footer 1: Volume free space bar
	volBar := ""
	if a.SpaceInfo.Total > 0 {
		bar := size.ShareBar(a.SpaceInfo.Used(), a.SpaceInfo.Total, 20)
		freeStr := size.HumanBytes(a.SpaceInfo.Available)
		totStr := size.HumanBytes(a.SpaceInfo.Total)
		usedPct := (float64(a.SpaceInfo.Used()) / float64(a.SpaceInfo.Total)) * 100.0

		volBar = fmt.Sprintf("Volume: [%s] %s free / %s (%.0f%% used)", bar, freeStr, totStr, usedPct)

		// If marked items exist, show projected recovery
		var markedBytes uint64
		for _, m := range a.Marks {
			markedBytes += m.Bytes
		}
		if markedBytes > 0 {
			proj := a.SpaceInfo.AfterRemoving(markedBytes)
			volBar += fmt.Sprintf(" -> %s free after marked", size.HumanBytes(proj.Available))
		}
	}
	sb.DrawString(1, footY, volBar, FgBrightCyan, Reset, false)

	// Footer 2: Current selection detail
	selDetail := "Selected: none"
	if a.SelectedTile >= 0 && a.SelectedTile < len(a.Tiles) {
		t := a.Tiles[a.SelectedTile]
		if t.Kind == treemap.TileOthers {
			selDetail = fmt.Sprintf("Selected: %d other smaller entries", t.Count)
		} else {
			n := a.CurrentNode.Resolve(t.Crumbs)
			if n != nil {
				p := a.buildPath(t.Crumbs)
				selDetail = fmt.Sprintf("Selected: %s | %s | %s files | %s",
					p, size.HumanBytes(n.Bytes), size.HumanCount(n.Files), n.Category.Label())
				if n.Reclaim != tree.ReclaimNone {
					selDetail += fmt.Sprintf(" [%s]", n.Reclaim.Label())
				}
				if _, ok := a.Marks[p]; ok {
					selDetail += " [MARKED FOR DELETION]"
				}
			}
		}
	}
	sb.DrawString(1, footY+1, selDetail, FgBrightWhite, Reset, true)

	// Footer 3: Status / Messages or Search input
	if a.Mode == ModeSearch {
		sb.DrawString(1, footY+2, "Search: "+a.SearchQuery+"█", HighlightBorder, Reset, true)
	} else if a.StatusMsg != "" {
		sb.DrawString(1, footY+2, a.StatusMsg, FgBrightRed, Reset, true)
	} else {
		// Key guide
		keys := "[Space] Mark  [Enter] Zoom  [Backspace] Up  [c] Review  [m] Metric  [/] Filter  [?] Help  [q] Quit"
		sb.DrawString(1, footY+2, keys, FgYellow, Reset, false)
	}

	// Render modal screens if active
	if a.Mode == ModeReview {
		a.drawReviewModal(sb, cols, rows)
	} else if a.Mode == ModeHelp {
		a.drawHelpModal(sb, cols, rows)
	}

	return sb.Render()
}

func (a *App) drawReviewModal(sb *ScreenBuffer, cols, rows int) {
	boxW := 60
	boxH := 16
	if boxW > cols-4 {
		boxW = cols - 4
	}
	if boxH > rows-4 {
		boxH = rows - 4
	}
	startX := (cols - boxW) / 2
	startY := (rows - boxH) / 2

	// Clear modal area
	for y := startY; y < startY+boxH; y++ {
		for x := startX; x < startX+boxW; x++ {
			isBorder := (x == startX || x == startX+boxW-1 || y == startY || y == startY+boxH-1)
			if isBorder {
				sb.Set(x, y, '#', HighlightBorder, BgBlack, true)
			} else {
				sb.Set(x, y, ' ', Reset, BgBlack, false)
			}
		}
	}

	sb.DrawString(startX+2, startY+1, "Review Marked Items for Deletion", HighlightBorder, BgBlack, true)

	var targets []removal.Target
	for _, m := range a.Marks {
		targets = append(targets, m)
	}
	plan := removal.BuildPlan(targets, a.RootPath)

	sb.DrawString(startX+2, startY+3, fmt.Sprintf("Marked Targets: %d", len(plan.Targets)), FgBrightWhite, BgBlack, false)
	sb.DrawString(startX+2, startY+4, fmt.Sprintf("Total Reclaimable: %s", size.HumanBytes(plan.Bytes())), FgBrightYellow, BgBlack, true)

	if len(plan.Covered) > 0 {
		sb.DrawString(startX+2, startY+5, fmt.Sprintf("Covered subpaths: %d", len(plan.Covered)), FgYellow, BgBlack, false)
	}
	if len(plan.Blocked) > 0 {
		sb.DrawString(startX+2, startY+6, fmt.Sprintf("Safety blocked paths: %d", len(plan.Blocked)), FgBrightRed, BgBlack, true)
	}

	sb.DrawString(startX+2, startY+8, "Actions:", FgBrightCyan, BgBlack, true)
	sb.DrawString(startX+4, startY+9, "[d] Confirm Permanent Delete (rm -rf)", FgBrightRed, BgBlack, true)
	sb.DrawString(startX+4, startY+10, "[t] Move to Trash (gio / trash-put)", FgBrightGreen, BgBlack, false)
	sb.DrawString(startX+4, startY+11, "[p] Export AI Agent Cleanup Prompt", FgBrightCyan, BgBlack, false)
	sb.DrawString(startX+4, startY+12, "[x] Clear All Marks", FgYellow, BgBlack, false)
	sb.DrawString(startX+4, startY+13, "[Esc] Return to Treemap", FgWhite, BgBlack, false)
}

func (a *App) drawHelpModal(sb *ScreenBuffer, cols, rows int) {
	boxW := 62
	boxH := 18
	if boxW > cols-4 {
		boxW = cols - 4
	}
	if boxH > rows-4 {
		boxH = rows - 4
	}
	startX := (cols - boxW) / 2
	startY := (rows - boxH) / 2

	for y := startY; y < startY+boxH; y++ {
		for x := startX; x < startX+boxW; x++ {
			isBorder := (x == startX || x == startX+boxW-1 || y == startY || y == startY+boxH-1)
			if isBorder {
				sb.Set(x, y, '#', HighlightBorder, BgBlack, true)
			} else {
				sb.Set(x, y, ' ', Reset, BgBlack, false)
			}
		}
	}

	sb.DrawString(startX+2, startY+1, "disktree Help & Keybindings", HighlightBorder, BgBlack, true)

	helpLines := []string{
		"Arrows / h j k l : Navigate between tiles",
		"Space            : Mark / unmark selected tile for removal",
		"Enter            : Zoom into directory",
		"Backspace / u    : Go up to parent directory",
		"c                : Review marked list and commit deletion",
		"m                : Toggle metric (Bytes / Files)",
		"[ and ]          : Decrease / increase drawing depth (1-6)",
		"/                : Search / filter by name",
		"?                : Open this help screen",
		"q / Esc          : Exit disktree",
	}

	for i, l := range helpLines {
		sb.DrawString(startX+2, startY+3+i, l, FgBrightWhite, BgBlack, false)
	}

	sb.DrawString(startX+2, startY+boxH-2, "Press [Esc] or [?] to close", HighlightBorder, BgBlack, false)
}

func (a *App) buildTrail() string {
	parts := []string{a.RootPath}
	curr := a.RootNode
	for _, c := range a.Crumbs {
		if c >= 0 && c < len(curr.Children) {
			curr = curr.Children[c]
			parts = append(parts, curr.Name)
		}
	}
	return strings.Join(parts, " > ")
}

func (a *App) buildPath(crumbs []int) string {
	relParts := []string{}
	curr := a.CurrentNode
	for _, c := range crumbs {
		if c >= 0 && c < len(curr.Children) {
			curr = curr.Children[c]
			relParts = append(relParts, curr.Name)
		}
	}

	base := a.RootPath
	currNode := a.RootNode
	for _, c := range a.Crumbs {
		if c >= 0 && c < len(currNode.Children) {
			currNode = currNode.Children[c]
			base = filepath.Join(base, currNode.Name)
		}
	}

	return filepath.Join(append([]string{base}, relParts...)...)
}

// HandleEvent processes keyboard input.
func (a *App) HandleEvent(ev Event) {
	a.StatusMsg = ""

	if a.Mode == ModeReview {
		switch ev.Type {
		case KeyEsc:
			a.Mode = ModeNormal
		case KeyChar:
			switch ev.Char {
			case 'd', 'D':
				a.executeDeletion(removal.Permanent)
				a.Mode = ModeNormal
			case 't', 'T':
				a.executeDeletion(removal.Trash)
				a.Mode = ModeNormal
			case 'x', 'X':
				a.Marks = make(map[string]removal.Target)
				a.StatusMsg = "Cleared all marked items"
				a.Mode = ModeNormal
			case 'p', 'P':
				a.exportPrompt()
			case 'q', 'Q':
				a.Mode = ModeNormal
			}
		}
		return
	}

	if a.Mode == ModeHelp {
		if ev.Type == KeyEsc || (ev.Type == KeyChar && (ev.Char == '?' || ev.Char == 'q')) {
			a.Mode = ModeNormal
		}
		return
	}

	if a.Mode == ModeSearch {
		switch ev.Type {
		case KeyEsc:
			a.Mode = ModeNormal
			a.SearchQuery = ""
			a.FilterResult = nil
		case KeyEnter:
			a.Mode = ModeNormal
		case KeyBackspace:
			if len(a.SearchQuery) > 0 {
				a.SearchQuery = a.SearchQuery[:len(a.SearchQuery)-1]
				a.updateFilter()
			}
		case KeyChar:
			a.SearchQuery += string(ev.Char)
			a.updateFilter()
		}
		return
	}

	// Normal Mode
	switch ev.Type {
	case KeyEsc:
		a.Running = false
	case KeyLeft:
		if a.SelectedTile > 0 {
			a.SelectedTile--
		}
	case KeyRight:
		if a.SelectedTile < len(a.Tiles)-1 {
			a.SelectedTile++
		}
	case KeyUp:
		if a.SelectedTile > 0 {
			a.SelectedTile--
		}
	case KeyDown:
		if a.SelectedTile < len(a.Tiles)-1 {
			a.SelectedTile++
		}
	case KeyEnter:
		a.zoomIn()
	case KeyBackspace:
		a.zoomOut()
	case KeySpace:
		a.toggleMark()
	case KeyChar:
		switch ev.Char {
		case 'q', 'Q':
			a.Running = false
		case 'h':
			if a.SelectedTile > 0 {
				a.SelectedTile--
			}
		case 'l':
			if a.SelectedTile < len(a.Tiles)-1 {
				a.SelectedTile++
			}
		case 'k':
			if a.SelectedTile > 0 {
				a.SelectedTile--
			}
		case 'j':
			if a.SelectedTile < len(a.Tiles)-1 {
				a.SelectedTile++
			}
		case 'u':
			a.zoomOut()
		case 'c', 'C':
			a.Mode = ModeReview
		case 'm', 'M':
			a.Metric = a.Metric.Toggled()
			tree.Aggregate(a.RootNode, a.Metric)
		case '[':
			if a.Depth > 1 {
				a.Depth--
			}
		case ']':
			if a.Depth < 6 {
				a.Depth++
			}
		case '/':
			a.Mode = ModeSearch
			a.SearchQuery = ""
		case '?':
			a.Mode = ModeHelp
		}
	}
}

func (a *App) zoomIn() {
	if a.SelectedTile < 0 || a.SelectedTile >= len(a.Tiles) {
		return
	}
	t := a.Tiles[a.SelectedTile]
	if t.Kind == treemap.TileOthers {
		return
	}
	targetNode := a.CurrentNode.Resolve(t.Crumbs)
	if targetNode != nil && targetNode.IsDir() && len(targetNode.Children) > 0 {
		a.Crumbs = append(a.Crumbs, t.Crumbs...)
		a.CurrentNode = targetNode
		a.SelectedTile = 0
	}
}

func (a *App) zoomOut() {
	if len(a.Crumbs) == 0 {
		return
	}
	a.Crumbs = a.Crumbs[:len(a.Crumbs)-1]
	a.CurrentNode = a.RootNode.Resolve(a.Crumbs)
	if a.CurrentNode == nil {
		a.CurrentNode = a.RootNode
		a.Crumbs = nil
	}
	a.SelectedTile = 0
}

func (a *App) toggleMark() {
	if a.SelectedTile < 0 || a.SelectedTile >= len(a.Tiles) {
		return
	}
	t := a.Tiles[a.SelectedTile]
	if t.Kind == treemap.TileOthers {
		return
	}
	targetNode := a.CurrentNode.Resolve(t.Crumbs)
	if targetNode == nil {
		return
	}
	path := a.buildPath(t.Crumbs)
	if _, ok := a.Marks[path]; ok {
		delete(a.Marks, path)
	} else {
		a.Marks[path] = removal.Target{
			Path:  path,
			Bytes: targetNode.Bytes,
			IsDir: targetNode.IsDir(),
		}
	}
}

func (a *App) updateFilter() {
	if a.SearchQuery == "" {
		a.FilterResult = nil
		return
	}
	a.FilterResult = filter.Filter(a.CurrentNode, nil, a.SearchQuery)
}

func (a *App) executeDeletion(mode removal.RemovalMode) {
	var targets []removal.Target
	for _, m := range a.Marks {
		targets = append(targets, m)
	}
	plan := removal.BuildPlan(targets, a.RootPath)
	if plan.IsEmpty() {
		a.StatusMsg = "No valid targets to delete"
		return
	}

	err := removal.Execute(plan, mode, nil)
	if err != nil {
		a.StatusMsg = "Removal error: " + err.Error()
	} else {
		a.StatusMsg = fmt.Sprintf("Successfully removed %d targets (%s freed)", len(plan.Targets), size.HumanBytes(plan.Bytes()))
		a.Marks = make(map[string]removal.Target)
		// Update space info
		if sp, err := space.GetSpaceInfo(a.RootPath); err == nil {
			a.SpaceInfo = sp
		}
	}
}

func (a *App) exportPrompt() {
	var targets []removal.Target
	for _, m := range a.Marks {
		targets = append(targets, m)
	}
	prompt := export.AgentPrompt(targets, a.RootPath, &a.SpaceInfo)
	outPath := filepath.Join(os.TempDir(), "disktree_agent_prompt.txt")
	if err := os.WriteFile(outPath, []byte(prompt), 0644); err == nil {
		a.StatusMsg = "Prompt saved to " + outPath
	} else {
		a.StatusMsg = "Failed to write prompt file"
	}
}

// Run executes the interactive loop.
func (a *App) Run() error {
	cols, rows, err := a.Term.GetSize()
	if err != nil {
		cols, rows = 80, 24
	}

	if err := a.Term.MakeRaw(); err != nil {
		return err
	}
	defer a.Term.Restore()

	for a.Running {
		frame := a.RenderFrame(cols, rows)
		os.Stdout.Write(frame)

		ev, err := a.Term.ReadEvent()
		if err != nil {
			break
		}
		a.HandleEvent(ev)

		// Check terminal resize
		if c, r, err := a.Term.GetSize(); err == nil {
			cols, rows = c, r
		}
	}
	return nil
}

// PrintOnce renders a single frame directly to stdout without interactive raw mode.
func (a *App) PrintOnce(cols, rows int) {
	frame := a.RenderFrame(cols, rows)
	os.Stdout.Write(frame)
	os.Stdout.WriteString("\n")
}
