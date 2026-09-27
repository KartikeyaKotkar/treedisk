package tree

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// NodeKind indicates what a node represents on disk.
type NodeKind uint8

const (
	Directory NodeKind = iota
	File
	Symlink
	Other
)

func (k NodeKind) IsDir() bool {
	return k == Directory
}

// Metric determines how a node's weight is measured.
type Metric uint8

const (
	Bytes Metric = iota
	Files
)

func (m Metric) Label() string {
	switch m {
	case Files:
		return "files"
	default:
		return "size"
	}
}

func (m Metric) Toggled() Metric {
	if m == Bytes {
		return Files
	}
	return Bytes
}

// Category represents the kind of data for visual coloring.
type Category uint8

const (
	OtherCat Category = iota
	Code
	AgentScratch
	Toolchain
	Synced
	Git
	Media
	Documents
	Cache
)

var Legend = [8]Category{
	Code,
	AgentScratch,
	Toolchain,
	Synced,
	Git,
	Media,
	Documents,
	Cache,
}

func (c Category) Label() string {
	switch c {
	case Code:
		return "Code"
	case AgentScratch:
		return "Agent scratch"
	case Toolchain:
		return "Toolchains"
	case Synced:
		return "Synced"
	case Git:
		return "Git"
	case Media:
		return "Media"
	case Documents:
		return "Documents"
	case Cache:
		return "Cache"
	default:
		return "Other"
	}
}

// Reclaim explains why space can be reclaimed.
type Reclaim uint8

const (
	ReclaimNone Reclaim = iota
	Regenerable
	SyncHistory
	PackageStore
	BuildOutput
	Reinstallable
	SandboxLayers
	Snapshots
	Trash
	Temporary
)

func (r Reclaim) Label() string {
	switch r {
	case Regenerable:
		return "regenerable"
	case SyncHistory:
		return "sync history"
	case PackageStore:
		return "package store"
	case BuildOutput:
		return "build output"
	case Reinstallable:
		return "reinstallable"
	case SandboxLayers:
		return "sandbox layers"
	case Snapshots:
		return "snapshots"
	case Trash:
		return "trash"
	case Temporary:
		return "temporary"
	default:
		return ""
	}
}

// Node represents an entry in the scanned disk tree.
type Node struct {
	Name      string
	Kind      NodeKind
	Bytes     uint64 // Subtree total
	OwnBytes  uint64 // Direct leaf entries size
	Files     uint64 // Subtree files count
	OwnFiles  uint64 // Direct leaf files count
	Dirs      uint64 // Subtree dirs count
	Dev       uint64 // Device ID for hardlink dedup
	Ino       uint64 // Inode for hardlink dedup
	HasInode  bool
	ReadError bool
	Modified  int64 // Newest write time (Unix timestamp in seconds)
	Category  Category
	Reclaim   Reclaim
	Children  []*Node
}

// NewDirectory creates an empty directory node.
func NewDirectory(name string) *Node {
	return &Node{
		Name:     name,
		Kind:     Directory,
		Dirs:     1,
		Category: OtherCat,
		Reclaim:  ReclaimNone,
	}
}

// NewEntry creates a leaf entry node.
func NewEntry(name string, kind NodeKind, bytes uint64) *Node {
	files := uint64(0)
	if kind == File {
		files = 1
	}
	return &Node{
		Name:     name,
		Kind:     kind,
		Bytes:    bytes,
		OwnBytes: bytes,
		Files:    files,
		OwnFiles: files,
		Dirs:     0,
		Category: OtherCat,
		Reclaim:  ReclaimNone,
	}
}

func (n *Node) IsDir() bool {
	return n.Kind.IsDir()
}

func (n *Node) Value(m Metric) uint64 {
	if m == Files {
		return n.Files
	}
	return n.Bytes
}

func (n *Node) ChildNamed(name string) *Node {
	for _, child := range n.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}

func (n *Node) Child(index int) *Node {
	if index >= 0 && index < len(n.Children) {
		return n.Children[index]
	}
	return nil
}

func (n *Node) Resolve(crumbs []int) *Node {
	curr := n
	for _, idx := range crumbs {
		if idx < 0 || idx >= len(curr.Children) {
			return nil
		}
		curr = curr.Children[idx]
	}
	return curr
}

func (n *Node) ResolveChain(crumbs []int) []*Node {
	chain := []*Node{n}
	curr := n
	for _, idx := range crumbs {
		if idx < 0 || idx >= len(curr.Children) {
			break
		}
		curr = curr.Children[idx]
		chain = append(chain, curr)
	}
	return chain
}

func (n *Node) LargestChild() int {
	if len(n.Children) == 0 {
		return -1
	}
	return 0
}

func (n *Node) Depth() uint32 {
	maxChild := uint32(0)
	for _, child := range n.Children {
		d := child.Depth()
		if d+1 > maxChild {
			maxChild = d + 1
		}
	}
	return maxChild
}

// Find performs BFS for first node whose name contains needle (case-insensitive).
func (n *Node) Find(needle string) ([]int, *Node) {
	needle = strings.ToLower(needle)
	if needle == "" {
		return nil, nil
	}

	type queueItem struct {
		crumbs []int
		node   *Node
	}
	queue := []queueItem{{crumbs: nil, node: n}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for idx, child := range curr.node.Children {
			if strings.Contains(strings.ToLower(child.Name), needle) {
				found := make([]int, len(curr.crumbs)+1)
				copy(found, curr.crumbs)
				found[len(curr.crumbs)] = idx
				return found, child
			}
			if child.IsDir() && len(child.Children) > 0 {
				nextCrumbs := make([]int, len(curr.crumbs)+1)
				copy(nextCrumbs, curr.crumbs)
				nextCrumbs[len(curr.crumbs)] = idx
				queue = append(queue, queueItem{crumbs: nextCrumbs, node: child})
			}
		}
	}
	return nil, nil
}

// Aggregate recomputes Bytes, Files, Dirs, OwnBytes, OwnFiles and sorts children largest-first.
func Aggregate(node *Node, metric Metric) {
	aggregateAt(node, metric, nil)
}

// AggregateDeduped charges a hardlinked file once across the tree.
func AggregateDeduped(node *Node, metric Metric, seen *Seen) {
	aggregateAt(node, metric, seen)
}

func aggregateAt(node *Node, metric Metric, seen *Seen) {
	if !node.IsDir() {
		if seen != nil && node.HasInode {
			if seen.HasSeen(node.Dev, node.Ino) {
				node.Bytes = 0
				node.OwnBytes = 0
				node.Files = 0
				node.OwnFiles = 0
			}
		}
		return
	}

	var bytes, files, dirs uint64
	var ownBytes, ownFiles uint64
	dirs = 1 // count self
	maxMod := node.Modified

	for _, child := range node.Children {
		aggregateAt(child, metric, seen)
		bytes += child.Bytes
		files += child.Files
		dirs += child.Dirs
		if !child.IsDir() {
			ownBytes += child.Bytes
			ownFiles += child.Files
		}
		if child.Modified > maxMod {
			maxMod = child.Modified
		}
	}

	node.Bytes = bytes
	node.Files = files
	node.Dirs = dirs
	node.OwnBytes = ownBytes
	node.OwnFiles = ownFiles
	node.Modified = maxMod

	// Sort children descending by metric
	sort.Slice(node.Children, func(i, j int) bool {
		vi := node.Children[i].Value(metric)
		vj := node.Children[j].Value(metric)
		if vi != vj {
			return vi > vj
		}
		return node.Children[i].Name < node.Children[j].Name
	})
}

// Seen tracks device+inode for hardlink deduplication.
// Uses a 64M bitmap for fast inode hits on primary volume, plus sharded map for others.
type Seen struct {
	mu     sync.Mutex
	hasDev bool
	dev    uint64
	bits   []uint64 // 1<<26 bits = 64M files = 8MB
	shards [64]struct {
		sync.Mutex
		m map[inodeKey]struct{}
	}
}

type inodeKey struct {
	dev uint64
	ino uint64
}

const seenBits = 1 << 26

func NewSeen() *Seen {
	s := &Seen{
		bits: make([]uint64, seenBits/64),
	}
	for i := range s.shards {
		s.shards[i].m = make(map[inodeKey]struct{})
	}
	return s
}

func (s *Seen) HasSeen(dev, ino uint64) bool {
	s.mu.Lock()
	if !s.hasDev {
		s.dev = dev
		s.hasDev = true
	}
	firstDev := s.dev
	s.mu.Unlock()

	if dev == firstDev && ino < seenBits {
		wordIdx := ino / 64
		bitMask := uint64(1) << (ino % 64)
		wordPtr := &s.bits[wordIdx]
		for {
			oldVal := atomic.LoadUint64(wordPtr)
			if oldVal&bitMask != 0 {
				return true // already seen
			}
			newVal := oldVal | bitMask
			if atomic.CompareAndSwapUint64(wordPtr, oldVal, newVal) {
				return false // first time seen
			}
		}
	}

	// Fallback to sharded map
	shardIdx := (dev ^ ino) % 64
	shard := &s.shards[shardIdx]
	shard.Lock()
	defer shard.Unlock()
	key := inodeKey{dev: dev, ino: ino}
	if _, ok := shard.m[key]; ok {
		return true
	}
	shard.m[key] = struct{}{}
	return false
}

// PathOf constructs path representation from node chain.
func PathOf(chain []*Node) string {
	if len(chain) == 0 {
		return ""
	}
	names := make([]string, len(chain))
	for i, n := range chain {
		names[i] = n.Name
	}
	return strings.Join(names, "/")
}
