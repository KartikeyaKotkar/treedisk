package space

import (
	"os"
	"testing"
)

func TestGetSpaceInfo(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	info, err := GetSpaceInfo(wd)
	if err != nil {
		t.Fatalf("GetSpaceInfo failed: %v", err)
	}

	if info.Total == 0 {
		t.Errorf("expected total > 0, got %d", info.Total)
	}
	if info.Free > info.Total {
		t.Errorf("free (%d) > total (%d)", info.Free, info.Total)
	}
}

func TestVolumeRoot(t *testing.T) {
	root := VolumeRootFor("/")
	if root != "/" {
		t.Errorf("expected /, got %s", root)
	}
}
