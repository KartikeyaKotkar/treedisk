package tui

import (
	"os"
	"syscall"
	"unsafe"
)

type termios struct {
	Iflag  uint32
	Oflag  uint32
	Cflag  uint32
	Lflag  uint32
	Line   uint8
	Cc     [32]uint8
	Ispeed uint32
	Ospeed uint32
}

type Terminal struct {
	fd       int
	origTerm termios
	isRaw    bool
}

func OpenTerminal() (*Terminal, error) {
	fd := int(os.Stdin.Fd())
	var orig termios
	if err := getTermios(fd, &orig); err != nil {
		return nil, err
	}
	return &Terminal{
		fd:       fd,
		origTerm: orig,
	}, nil
}

func (t *Terminal) MakeRaw() error {
	raw := t.origTerm
	// Disable echo, canonical mode, extended input processing, and interrupt signals
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.IEXTEN | syscall.ISIG
	// Disable software flow control and carriage return translation
	raw.Iflag &^= syscall.IXON | syscall.ICRNL | syscall.BRKINT | syscall.INPCK | syscall.ISTRIP
	// Disable output post-processing
	raw.Oflag &^= syscall.OPOST
	// 8-bit chars
	raw.Cflag |= syscall.CS8
	// Min bytes = 1, timeout = 0 (blocking read)
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0

	if err := setTermios(t.fd, &raw); err != nil {
		return err
	}
	t.isRaw = true

	// Enter alt screen and hide cursor
	os.Stdout.WriteString("\x1b[?1049h\x1b[?25l")
	return nil
}

func (t *Terminal) Restore() {
	if t.isRaw {
		// Show cursor and leave alt screen
		os.Stdout.WriteString("\x1b[?25h\x1b[?1049l")
		_ = setTermios(t.fd, &t.origTerm)
		t.isRaw = false
	}
}

type Winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func (t *Terminal) GetSize() (int, int, error) {
	var ws Winsize
	_, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(t.fd),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)),
	)
	if err != 0 {
		return 80, 24, err
	}
	cols := int(ws.Col)
	rows := int(ws.Row)
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	return cols, rows, nil
}

func getTermios(fd int, t *termios) error {
	_, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(t)),
	)
	if err != 0 {
		return err
	}
	return nil
}

func setTermios(fd int, t *termios) error {
	_, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(t)),
	)
	if err != 0 {
		return err
	}
	return nil
}

// Key represents a user input key.
type Key uint16

const (
	KeyNone Key = iota
	KeyEnter
	KeySpace
	KeyBackspace
	KeyTab
	KeyEsc
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyPgUp
	KeyPgDn
	KeyHome
	KeyEnd
	KeyChar
)

type Event struct {
	Type Key
	Char rune
}

func (t *Terminal) ReadEvent() (Event, error) {
	var buf [16]byte
	n, err := os.Stdin.Read(buf[:])
	if err != nil {
		return Event{Type: KeyNone}, err
	}
	if n == 0 {
		return Event{Type: KeyNone}, nil
	}

	b := buf[0]
	switch b {
	case '\r', '\n':
		return Event{Type: KeyEnter}, nil
	case ' ':
		return Event{Type: KeySpace, Char: ' '}, nil
	case 127, 8:
		return Event{Type: KeyBackspace}, nil
	case '\t':
		return Event{Type: KeyTab}, nil
	case 27: // Escape or escape sequence
		if n == 1 {
			return Event{Type: KeyEsc}, nil
		}
		if n >= 3 && buf[1] == '[' {
			switch buf[2] {
			case 'A':
				return Event{Type: KeyUp}, nil
			case 'B':
				return Event{Type: KeyDown}, nil
			case 'C':
				return Event{Type: KeyRight}, nil
			case 'D':
				return Event{Type: KeyLeft}, nil
			case 'H':
				return Event{Type: KeyHome}, nil
			case 'F':
				return Event{Type: KeyEnd}, nil
			case '5':
				if n >= 4 && buf[3] == '~' {
					return Event{Type: KeyPgUp}, nil
				}
			case '6':
				if n >= 4 && buf[3] == '~' {
					return Event{Type: KeyPgDn}, nil
				}
			}
		}
		return Event{Type: KeyEsc}, nil
	default:
		return Event{Type: KeyChar, Char: rune(b)}, nil
	}
}
