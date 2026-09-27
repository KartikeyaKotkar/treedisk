package space

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// SpaceInfo holds a volume's capacity in bytes.
type SpaceInfo struct {
	Total     uint64
	Free      uint64 // Free blocks including root reserve
	Available uint64 // Free blocks available to unprivileged user
}

func (s SpaceInfo) Used() uint64 {
	if s.Total > s.Free {
		return s.Total - s.Free
	}
	return 0
}

func (s SpaceInfo) UsedFraction() float32 {
	if s.Total == 0 {
		return 0
	}
	return float32(float64(s.Used()) / float64(s.Total))
}

func (s SpaceInfo) AfterRemoving(bytes uint64) SpaceInfo {
	avail := s.Available + bytes
	if avail > s.Total {
		avail = s.Total
	}
	free := s.Free + bytes
	if free > s.Total {
		free = s.Total
	}
	return SpaceInfo{
		Total:     s.Total,
		Free:      free,
		Available: avail,
	}
}

// GetSpaceInfo reads capacity of the filesystem containing path.
func GetSpaceInfo(path string) (SpaceInfo, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return SpaceInfo{}, err
	}

	bsize := uint64(stat.Bsize)
	if stat.Frsize > 0 {
		bsize = uint64(stat.Frsize)
	}

	return SpaceInfo{
		Total:     stat.Blocks * bsize,
		Free:      stat.Bfree * bsize,
		Available: stat.Bavail * bsize,
	}, nil
}

// DeviceFor returns the mounted device for path from /proc/self/mounts.
func DeviceFor(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	f, err := os.Open("/proc/self/mounts")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	bestMount := ""
	bestDev := ""

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		dev := fields[0]
		mount := fields[1]

		// Handle octal escapes like \040 for spaces
		mount = strings.ReplaceAll(mount, "\\040", " ")

		if strings.HasPrefix(abs, mount) {
			if len(mount) > len(bestMount) {
				bestMount = mount
				bestDev = dev
			}
		}
	}
	return bestDev
}

// VolumeRootFor returns the mount point for path.
func VolumeRootFor(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	f, err := os.Open("/proc/self/mounts")
	if err != nil {
		return "/"
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	bestMount := "/"

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		mount := strings.ReplaceAll(fields[1], "\\040", " ")
		if strings.HasPrefix(abs, mount) && len(mount) > len(bestMount) {
			bestMount = mount
		}
	}
	return bestMount
}
