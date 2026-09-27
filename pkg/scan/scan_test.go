package scan

import (
	"github.com/KartikeyaKotkar/treedisk/pkg/tree"
	"os"
	"path/filepath"
	"testing"
)

func TestScanDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disktree_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	subDir := filepath.Join(tempDir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(subDir, "hello.txt")
	content := []byte("hello disk tree scanner")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	opts := DefaultScanOptions()
	opts.ApparentSize = true // measure byte length for exact comparison
	opts.Metric = tree.Bytes

	root, err := Scan(tempDir, opts, nil)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if root.Files != 1 {
		t.Errorf("expected 1 file, got %d", root.Files)
	}
	if root.Bytes != uint64(len(content)) {
		t.Errorf("expected %d bytes, got %d", len(content), root.Bytes)
	}
}
