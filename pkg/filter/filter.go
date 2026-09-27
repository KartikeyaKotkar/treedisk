package filter

import (
	"disktree/pkg/tree"
	"fmt"
	"strings"
)

type KeepKind uint8

const (
	KeepWhole KeepKind = iota
	KeepPartial
)

type Keep struct {
	Kind  KeepKind
	Bytes uint64
	Files uint64
}

// Matches holds the results of filtering a subtree by substring.
type Matches struct {
	Needle string
	Base   []int
	keep   map[string]Keep
	Count  int
	Bytes  uint64
	Files  uint64
}

func crumbsKey(crumbs []int) string {
	var sb strings.Builder
	for i, c := range crumbs {
		if i > 0 {
			sb.WriteByte('/')
		}
		fmt.Fprintf(&sb, "%d", c)
	}
	return sb.String()
}

func (m *Matches) KeepFor(crumbs []int) (Keep, bool) {
	if len(crumbs) < len(m.Base) {
		return Keep{Kind: KeepWhole}, true
	}
	for i, b := range m.Base {
		if crumbs[i] != b {
			return Keep{Kind: KeepWhole}, true
		}
	}

	for length := len(m.Base); length <= len(crumbs); length++ {
		key := crumbsKey(crumbs[:length])
		if k, ok := m.keep[key]; ok {
			if k.Kind == KeepWhole {
				return Keep{Kind: KeepWhole}, true
			}
			if length == len(crumbs) {
				return k, true
			}
		} else if length > len(m.Base) {
			return Keep{}, false
		}
	}

	return Keep{
		Kind:  KeepPartial,
		Bytes: m.Bytes,
		Files: m.Files,
	}, true
}

// Filter searches node for names containing needle.
func Filter(node *tree.Node, base []int, needle string) *Matches {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return nil
	}

	m := &Matches{
		Needle: needle,
		Base:   append([]int(nil), base...),
		keep:   make(map[string]Keep),
	}

	crumbs := append([]int(nil), base...)
	bytes, files := visit(node, &crumbs, m)
	m.Bytes = bytes
	m.Files = files

	if m.Count == 0 {
		return nil
	}
	return m
}

func visit(node *tree.Node, crumbs *[]int, m *Matches) (uint64, uint64) {
	var totalBytes, totalFiles uint64

	for idx, child := range node.Children {
		*crumbs = append(*crumbs, idx)
		nameMatches := strings.Contains(strings.ToLower(child.Name), m.Needle)

		if nameMatches {
			m.keep[crumbsKey(*crumbs)] = Keep{Kind: KeepWhole}
			m.Count++
			totalBytes += child.Bytes
			totalFiles += child.Files
		} else if child.IsDir() && len(child.Children) > 0 {
			subBytes, subFiles := visit(child, crumbs, m)
			if subBytes > 0 || subFiles > 0 {
				m.keep[crumbsKey(*crumbs)] = Keep{
					Kind:  KeepPartial,
					Bytes: subBytes,
					Files: subFiles,
				}
				totalBytes += subBytes
				totalFiles += subFiles
			}
		}
		*crumbs = (*crumbs)[:len(*crumbs)-1]
	}

	return totalBytes, totalFiles
}
