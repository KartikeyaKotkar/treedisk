package removal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Target represents one marked path for removal.
type Target struct {
	Path   string
	Bytes  uint64
	IsDir  bool
	Hidden bool
}

// RemovalMode indicates how removal should be carried out.
type RemovalMode uint8

const (
	Permanent RemovalMode = iota
	Trash
)

func (m RemovalMode) Label() string {
	if m == Trash {
		return "Move to trash"
	}
	return "Delete permanently"
}

func (m RemovalMode) Detail() string {
	if m == Trash {
		return "recoverable until trash is emptied"
	}
	return "rm -rf: unrecoverable"
}

// Blocked represents a target that will not be touched.
type Blocked struct {
	Path   string
	Reason string
}

// Plan represents the actions a removal will execute.
type Plan struct {
	Targets []Target
	Covered []Target
	Blocked []Blocked
	Root    string
}

func (p Plan) Bytes() uint64 {
	var total uint64
	for _, t := range p.Targets {
		total += t.Bytes
	}
	return total
}

func (p Plan) IsEmpty() bool {
	return len(p.Targets) == 0
}

// BuildPlan validates targets against root, eliminates covered subtargets, and blocks dangerous paths.
func BuildPlan(targets []Target, root string) Plan {
	cleanRoot := filepath.Clean(root)
	plan := Plan{Root: cleanRoot}

	// Safety: check dangerous roots
	criticalRoots := []string{"/", "/home", "/root", "/etc", "/usr", "/bin", "/lib", "/var", "/dev", "/sys", "/proc"}

	for _, t := range targets {
		cleanPath := filepath.Clean(t.Path)

		// Never delete critical system directories
		isCrit := false
		for _, crit := range criticalRoots {
			if cleanPath == crit {
				plan.Blocked = append(plan.Blocked, Blocked{
					Path:   cleanPath,
					Reason: "refusing to remove critical system path",
				})
				isCrit = true
				break
			}
		}
		if isCrit {
			continue
		}

		// Never delete scanned root directly
		if cleanPath == cleanRoot {
			plan.Blocked = append(plan.Blocked, Blocked{
				Path:   cleanPath,
				Reason: "refusing to remove the scanned root itself",
			})
			continue
		}

		// Must be inside cleanRoot
		rel, err := filepath.Rel(cleanRoot, cleanPath)
		if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
			plan.Blocked = append(plan.Blocked, Blocked{
				Path:   cleanPath,
				Reason: "path is outside the scanned root",
			})
			continue
		}

		plan.Targets = append(plan.Targets, t)
	}

	// Deduplicate: if target A is inside target B, target A is covered by B
	var deduplicated []Target
	for i, t := range plan.Targets {
		covered := false
		for j, other := range plan.Targets {
			if i != j && other.IsDir {
				rel, err := filepath.Rel(other.Path, t.Path)
				if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
					covered = true
					break
				}
			}
		}
		if covered {
			plan.Covered = append(plan.Covered, t)
		} else {
			deduplicated = append(deduplicated, t)
		}
	}
	plan.Targets = deduplicated

	return plan
}

// Execute performs the removal plan.
func Execute(p Plan, mode RemovalMode, onProgress func(Target, error)) error {
	for _, target := range p.Targets {
		var err error
		if mode == Trash {
			err = moveToTrash(target.Path)
			if err != nil {
				// Fallback to permanent if trash unavailable? Better to report error
			}
		} else {
			err = os.RemoveAll(target.Path)
		}
		if onProgress != nil {
			onProgress(target, err)
		}
	}
	return nil
}

func moveToTrash(path string) error {
	if cmdPath, err := exec.LookPath("gio"); err == nil {
		cmd := exec.Command(cmdPath, "trash", path)
		if out, err := cmd.CombinedOutput(); err == nil {
			return nil
		} else {
			return fmt.Errorf("gio trash failed: %v (%s)", err, string(out))
		}
	}
	if cmdPath, err := exec.LookPath("trash-put"); err == nil {
		cmd := exec.Command(cmdPath, path)
		if out, err := cmd.CombinedOutput(); err == nil {
			return nil
		} else {
			return fmt.Errorf("trash-put failed: %v (%s)", err, string(out))
		}
	}
	return errors.New("no trash utility found (installed gio or trash-cli needed)")
}
