package main

import (
	"github.com/KartikeyaKotkar/treedisk/pkg/scan"
	"github.com/KartikeyaKotkar/treedisk/pkg/space"
	"github.com/KartikeyaKotkar/treedisk/pkg/tree"
	"github.com/KartikeyaKotkar/treedisk/pkg/tui"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const usage = `treedisk — a treemap of what is using your disk

usage: treedisk [OPTIONS] [PATH]

arguments:
  PATH              directory to scan (default: the home directory)

The screen opens on a treemap of the root, largest first. Space marks the
selected tile, Enter opens it, c reviews the marked list, ? lists every key.

options:
  -a, --apparent-size   measure apparent length instead of allocated blocks
  -l, --follow-links    follow symlinks
  -H, --no-hidden       skip dotfiles and dot-directories
  -D, --disk            scan the whole disk the home directory is on
  -X, --cross-filesystems
                        also measure other disks, network shares and pseudo
                        filesystems mounted below PATH (off by default)
  -d, --depth N         how many levels to draw at once (1-6, default 3)
  -m, --mono            minimal monochrome terminal theme
      --no-sidebar      start with contents sidebar hidden
      --metric files    rank by file count instead of bytes
      --once            render a single frame and exit (benchmark / script mode)
  -h, --help            show this help
`

func main() {
	opts := scan.DefaultScanOptions()
	var targetPath string
	depth := uint32(3)
	disk := false
	once := false
	mono := false
	sidebar := true

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			fmt.Print(usage)
			os.Exit(0)
		case "-a", "--apparent-size":
			opts.ApparentSize = true
		case "-l", "--follow-links":
			opts.FollowLinks = true
		case "-H", "--no-hidden":
			opts.IncludeHidden = false
		case "-m", "--mono", "--monochrome":
			mono = true
		case "--no-sidebar":
			sidebar = false
		case "-x", "--one-filesystem":
			opts.OneFilesystem = true
		case "-X", "--cross-filesystems":
			opts.OneFilesystem = false
		case "-D", "--disk":
			disk = true
		case "--once":
			once = true
		case "-d", "--depth":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --depth requires a number")
				os.Exit(1)
			}
			i++
			val, err := strconv.Atoi(args[i])
			if err != nil || val < 1 || val > 6 {
				fmt.Fprintln(os.Stderr, "error: --depth must be 1 to 6")
				os.Exit(1)
			}
			depth = uint32(val)
		case "--metric":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --metric requires a value (bytes|files)")
				os.Exit(1)
			}
			i++
			val := strings.ToLower(args[i])
			switch val {
			case "files":
				opts.Metric = tree.Files
			case "bytes", "size":
				opts.Metric = tree.Bytes
			default:
				fmt.Fprintf(os.Stderr, "error: unknown metric %s; try bytes or files\n", val)
				os.Exit(1)
			}
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "unknown option %s\n\n%s", arg, usage)
				os.Exit(1)
			}
			if targetPath != "" {
				fmt.Fprintln(os.Stderr, "error: only one path can be scanned")
				os.Exit(1)
			}
			targetPath = arg
		}
	}

	if disk && targetPath != "" {
		fmt.Fprintln(os.Stderr, "error: --disk and PATH cannot be combined")
		os.Exit(1)
	}

	home, _ := os.UserHomeDir()
	if disk {
		if home != "" {
			targetPath = space.VolumeRootFor(home)
		} else {
			targetPath = "/"
		}
	} else if targetPath == "" {
		if home != "" {
			targetPath = home
		} else {
			targetPath = "."
		}
	}

	cleanRoot, err := filepath.Abs(targetPath)
	if err != nil {
		cleanRoot = targetPath
	}

	fi, err := os.Stat(cleanRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot read %s: %v\n", cleanRoot, err)
		os.Exit(1)
	}
	if !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "error: %s is not a directory\n", cleanRoot)
		os.Exit(1)
	}

	// Scan directory
	rootNode, err := scan.Scan(cleanRoot, opts, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error during scan: %v\n", err)
		os.Exit(1)
	}

	app := tui.NewApp(cleanRoot, rootNode, opts.Metric, depth)
	app.Monochrome = mono
	app.SidebarOpen = sidebar

	// Check if terminal is interactive
	isTerminal := isatty(int(os.Stdout.Fd()))
	if once || !isTerminal {
		cols := 100
		rows := 30
		if term, err := tui.OpenTerminal(); err == nil {
			if c, r, err := term.GetSize(); err == nil {
				cols, rows = c, r
			}
		}
		app.PrintOnce(cols, rows)
		return
	}

	term, err := tui.OpenTerminal()
	if err != nil {
		app.PrintOnce(100, 30)
		return
	}
	app.Term = term

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "runtime error: %v\n", err)
		os.Exit(1)
	}
}

func isatty(fd int) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&termios)),
	)
	return err == 0
}
