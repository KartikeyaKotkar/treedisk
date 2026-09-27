package size

import "testing"

func TestSizeFormatting(t *testing.T) {
	tests := []struct {
		bytes uint64
		want  string
	}{
		{0, "0 B"},
		{7, "7 B"},
		{12, "12 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{523 * 1024 * 1024, "523 MiB"},
		{1500 * 1024 * 1024, "1.5 GiB"},
	}

	for _, tt := range tests {
		got := HumanBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("HumanBytes(%d) = %q; want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestHumanBytesShort(t *testing.T) {
	if got := HumanBytesShort(1536); got != "1.5K" {
		t.Errorf("got %q, want 1.5K", got)
	}
	if got := HumanBytesShort(523 * 1024 * 1024); got != "523M" {
		t.Errorf("got %q, want 523M", got)
	}
}

func TestShareBar(t *testing.T) {
	bar := ShareBar(50, 100, 10)
	if bar != "▓▓▓▓▓░░░░░" {
		t.Errorf("ShareBar = %q", bar)
	}
}
