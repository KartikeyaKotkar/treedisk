package filter

import (
	"github.com/KartikeyaKotkar/treedisk/pkg/tree"
	"testing"
)

func TestFilter(t *testing.T) {
	root := tree.NewDirectory("root")
	sub1 := tree.NewDirectory("sub1")
	f1 := tree.NewEntry("match_me.txt", tree.File, 500)
	f2 := tree.NewEntry("ignore.txt", tree.File, 1000)

	sub1.Children = append(sub1.Children, f1, f2)
	root.Children = append(root.Children, sub1)
	tree.Aggregate(root, tree.Bytes)

	m := Filter(root, nil, "match_me")
	if m == nil {
		t.Fatal("expected filter matches, got nil")
	}

	if m.Bytes != 500 {
		t.Errorf("expected 500 bytes matched, got %d", m.Bytes)
	}
	if m.Count != 1 {
		t.Errorf("expected 1 match, got %d", m.Count)
	}
}
