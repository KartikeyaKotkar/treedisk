package insights

import (
	"github.com/KartikeyaKotkar/treedisk/pkg/tree"
	"sort"
	"strings"
)

const (
	day       = 86400
	staleDays = 30
	minBytes  = 64 * 1024 * 1024 // 64 MiB
)

type FindingKind uint8

const (
	FindingReclaimable FindingKind = iota
	FindingWorktrees
	FindingStaleExperiments
)

type Finding struct {
	Kind       FindingKind
	Reclaim    tree.Reclaim
	Count      int
	OldestDays int64
}

type Candidate struct {
	Crumbs  []int
	Bytes   uint64
	Finding Finding
}

// WorthALook finds large reclaimable candidates, agent worktrees, and stale experiments.
func WorthALook(root *tree.Node, now int64, limit int) []Candidate {
	var found []Candidate
	var crumbs []int

	for idx, child := range root.Children {
		crumbs = append(crumbs, idx)
		visit(child, &crumbs, now, &found)
		crumbs = crumbs[:len(crumbs)-1]
	}

	sort.Slice(found, func(i, j int) bool {
		return found[i].Bytes > found[j].Bytes
	})

	if len(found) > limit {
		found = found[:limit]
	}
	return found
}

func visit(node *tree.Node, crumbs *[]int, now int64, found *[]Candidate) {
	if !node.IsDir() || node.Bytes < minBytes {
		return
	}

	if node.Reclaim != tree.ReclaimNone {
		*found = append(*found, Candidate{
			Crumbs: append([]int(nil), *crumbs...),
			Bytes:  node.Bytes,
			Finding: Finding{
				Kind:    FindingReclaimable,
				Reclaim: node.Reclaim,
			},
		})
		return
	}

	name := strings.ToLower(node.Name)
	if node.Category == tree.AgentScratch && name == "worktrees" {
		var trees []*tree.Node
		for _, child := range node.Children {
			if child.IsDir() {
				trees = append(trees, child)
			}
		}
		if len(trees) > 0 {
			oldest := now
			for _, t := range trees {
				if t.Modified > 0 && t.Modified < oldest {
					oldest = t.Modified
				}
			}
			days := (now - oldest) / day
			if days < 0 {
				days = 0
			}
			*found = append(*found, Candidate{
				Crumbs: append([]int(nil), *crumbs...),
				Bytes:  node.Bytes,
				Finding: Finding{
					Kind:       FindingWorktrees,
					Count:      len(trees),
					OldestDays: days,
				},
			})
			return
		}
	}

	for idx, child := range node.Children {
		*crumbs = append(*crumbs, idx)
		visit(child, crumbs, now, found)
		*crumbs = (*crumbs)[:len(*crumbs)-1]
	}
}
