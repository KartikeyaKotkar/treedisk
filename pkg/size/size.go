package size

import (
	"fmt"
	"math"
	"strings"
)

var byteUnits = [...]string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
var byteUnitsShort = [...]string{"B", "K", "M", "G", "T", "P"}

// HumanBytes returns binary units with 1 decimal below 10: "1.4 GiB", "523 MiB", "12 B".
func HumanBytes(bytes uint64) string {
	val, unit := scale(bytes)
	var text string
	if unit == 0 {
		text = fmt.Sprintf("%.0f", val)
	} else if val < 9.95 {
		text = fmt.Sprintf("%.1f", val)
	} else {
		text = fmt.Sprintf("%.0f", val)
	}
	return fmt.Sprintf("%s %s", text, byteUnits[unit])
}

// HumanBytesShort returns compact units: "1.4G", "523M", "12B".
func HumanBytesShort(bytes uint64) string {
	val, unit := scale(bytes)
	var text string
	if unit == 0 {
		text = fmt.Sprintf("%.0f", val)
	} else if val < 9.95 {
		text = fmt.Sprintf("%.1f", val)
	} else {
		text = fmt.Sprintf("%.0f", val)
	}
	return fmt.Sprintf("%s%s", text, byteUnitsShort[unit])
}

func scale(bytes uint64) (float64, int) {
	val := float64(bytes)
	unit := 0
	for val >= 1024.0 && unit+1 < len(byteUnits) {
		val /= 1024.0
		unit++
	}
	return val, unit
}

// HumanCount formats counts: "1.2k", "3.4M", "812".
func HumanCount(count uint64) string {
	if count < 10_000 {
		return fmt.Sprintf("%d", count)
	}
	val := float64(count)
	if val < 1_000_000.0 {
		return fmt.Sprintf("%.1fk", val/1_000.0)
	} else if val < 1_000_000_000.0 {
		return fmt.Sprintf("%.1fM", val/1_000_000.0)
	}
	return fmt.Sprintf("%.1fG", val/1_000_000_000.0)
}

// Share returns percentage of total (0..100).
func Share(part, total uint64) float64 {
	if total == 0 {
		return 0.0
	}
	return (float64(part) / float64(total)) * 100.0
}

// ShareBar returns a visual meter like "▓▓▓░░".
func ShareBar(part, total uint64, width int) string {
	if width <= 0 {
		return ""
	}
	if total == 0 {
		return strings.Repeat("░", width)
	}
	filled := int(math.Round((float64(part) / float64(total)) * float64(width)))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("▓", filled) + strings.Repeat("░", width-filled)
}
