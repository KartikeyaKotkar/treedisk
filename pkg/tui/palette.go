package tui

import "disktree/pkg/tree"

// ANSI Color definitions
const (
	Reset       = "\x1b[0m"
	Bold        = "\x1b[1m"
	Dim         = "\x1b[2m"
	Underline   = "\x1b[4m"
	Invert      = "\x1b[7m"

	// Basic colors
	FgBlack   = "\x1b[30m"
	FgRed     = "\x1b[31m"
	FgGreen   = "\x1b[32m"
	FgYellow  = "\x1b[33m"
	FgBlue    = "\x1b[34m"
	FgMagenta = "\x1b[35m"
	FgCyan    = "\x1b[36m"
	FgWhite   = "\x1b[37m"

	// Bright colors
	FgBrightRed     = "\x1b[91m"
	FgBrightGreen   = "\x1b[92m"
	FgBrightYellow  = "\x1b[93m"
	FgBrightBlue    = "\x1b[94m"
	FgBrightMagenta = "\x1b[95m"
	FgBrightCyan    = "\x1b[96m"
	FgBrightWhite   = "\x1b[97m"

	// Backgrounds
	BgBlack   = "\x1b[40m"
	BgRed     = "\x1b[41m"
	BgGreen   = "\x1b[42m"
	BgYellow  = "\x1b[43m"
	BgBlue    = "\x1b[44m"
	BgMagenta = "\x1b[45m"
	BgCyan    = "\x1b[46m"
	BgWhite   = "\x1b[47m"

	// Highlight amber / gold for selection
	HighlightBorder = "\x1b[38;5;214m" + Bold
	HighlightBg     = "\x1b[48;5;236m"
)

// CategoryColor returns ANSI escape code for tile foreground according to its kind.
func CategoryColor(cat tree.Category) string {
	switch cat {
	case tree.Code:
		return "\x1b[38;5;75m" // Blue
	case tree.AgentScratch:
		return "\x1b[38;5;208m" // Orange
	case tree.Toolchain:
		return "\x1b[38;5;78m" // Teal / Green
	case tree.Synced:
		return "\x1b[38;5;44m" // Cyan
	case tree.Git:
		return "\x1b[38;5;197m" // Coral / Red
	case tree.Media:
		return "\x1b[38;5;176m" // Violet / Purple
	case tree.Documents:
		return "\x1b[38;5;222m" // Light Gold / Sand
	case tree.Cache:
		return "\x1b[38;5;214m" // Amber / Yellow
	default:
		return "\x1b[38;5;246m" // Muted Gray
	}
}

// CategoryBg returns subtle dark background tint for tile.
func CategoryBg(cat tree.Category) string {
	switch cat {
	case tree.Code:
		return "\x1b[48;5;235m"
	case tree.AgentScratch:
		return "\x1b[48;5;52m"
	case tree.Toolchain:
		return "\x1b[48;5;236m"
	case tree.Synced:
		return "\x1b[48;5;235m"
	case tree.Git:
		return "\x1b[48;5;53m"
	case tree.Media:
		return "\x1b[48;5;236m"
	case tree.Documents:
		return "\x1b[48;5;235m"
	case tree.Cache:
		return "\x1b[48;5;58m"
	default:
		return "\x1b[48;5;234m"
	}
}
