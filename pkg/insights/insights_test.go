package insights

import (
	"github.com/KartikeyaKotkar/treedisk/pkg/tree"
	"testing"
)

func TestWorthALook(t *testing.T) {
	root := tree.NewDirectory("home")
	cache := tree.NewDirectory(".cache")
	cache.Reclaim = tree.Regenerable
	cache.Children = append(cache.Children, tree.NewEntry("blob", tree.File, 100*1024*1024)) // 100 MiB

	root.Children = append(root.Children, cache)
	tree.Aggregate(root, tree.Bytes)

	candidates := WorthALook(root, 1000000, 10)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Finding.Kind != FindingReclaimable {
		t.Errorf("expected reclaimable finding, got %v", candidates[0].Finding.Kind)
	}
}
