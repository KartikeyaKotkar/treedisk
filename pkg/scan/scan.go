package scan

import (
	"github.com/KartikeyaKotkar/treedisk/pkg/classify"
	"github.com/KartikeyaKotkar/treedisk/pkg/tree"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
)

// ScanOptions configures filesystem measurement and filtering.
type ScanOptions struct {
	ApparentSize   bool
	FollowLinks    bool
	IncludeHidden  bool
	OneFilesystem  bool
	MaxDepth       int // 0 means unlimited
	DedupHardlinks bool
	Metric         tree.Metric
}

func DefaultScanOptions() ScanOptions {
	return ScanOptions{
		ApparentSize:   false,
		FollowLinks:    false,
		IncludeHidden:  true,
		OneFilesystem:  true,
		MaxDepth:       0,
		DedupHardlinks: true,
		Metric:         tree.Bytes,
	}
}

// Progress tracks live scan counters.
type Progress struct {
	files     atomic.Uint64
	dirs      atomic.Uint64
	bytes     atomic.Uint64
	errors    atomic.Uint64
	finished  atomic.Bool
	cancelled atomic.Bool

	mu       sync.Mutex
	messages []string
}

func (p *Progress) Add(files, dirs, bytes uint64) {
	p.files.Add(files)
	p.dirs.Add(dirs)
	p.bytes.Add(bytes)
}

func (p *Progress) AddError(msg string) {
	p.errors.Add(1)
	p.mu.Lock()
	if len(p.messages) < 50 {
		p.messages = append(p.messages, msg)
	}
	p.mu.Unlock()
}

func (p *Progress) Cancel() {
	p.cancelled.Store(true)
}

func (p *Progress) IsCancelled() bool {
	return p.cancelled.Load()
}

type Snapshot struct {
	Files     uint64
	Dirs      uint64
	Bytes     uint64
	Errors    uint64
	Finished  bool
	Cancelled bool
	Messages  []string
}

func (p *Progress) Snapshot() Snapshot {
	p.mu.Lock()
	msgs := append([]string(nil), p.messages...)
	p.mu.Unlock()

	return Snapshot{
		Files:     p.files.Load(),
		Dirs:      p.dirs.Load(),
		Bytes:     p.bytes.Load(),
		Errors:    p.errors.Load(),
		Finished:  p.finished.Load(),
		Cancelled: p.cancelled.Load(),
		Messages:  msgs,
	}
}

// Scan scans root directory according to options, returning tree with aggregated values and classifications.
func Scan(rootPath string, opts ScanOptions, prog *Progress) (*tree.Node, error) {
	if prog == nil {
		prog = &Progress{}
	}

	cleanRoot := filepath.Clean(rootPath)
	var rootStat syscall.Stat_t
	var err error

	if opts.FollowLinks {
		err = syscall.Stat(cleanRoot, &rootStat)
	} else {
		err = syscall.Lstat(cleanRoot, &rootStat)
	}
	if err != nil {
		return nil, err
	}

	rootNode := tree.NewDirectory(filepath.Base(cleanRoot))
	rootNode.Dev = uint64(rootStat.Dev)
	rootNode.Ino = uint64(rootStat.Ino)
	rootNode.HasInode = true
	rootNode.Modified = rootStat.Mtim.Sec

	rootDev := uint64(rootStat.Dev)

	// Concurrency limiter for directory scanning
	concurrency := 16
	tokens := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	wg.Add(1)
	scanDir(cleanRoot, rootNode, rootDev, 0, opts, prog, tokens, &wg)
	wg.Wait()

	if prog.IsCancelled() {
		return rootNode, nil
	}

	// Hardlink deduplication & aggregation
	if opts.DedupHardlinks {
		seen := tree.NewSeen()
		tree.AggregateDeduped(rootNode, opts.Metric, seen)
	} else {
		tree.Aggregate(rootNode, opts.Metric)
	}

	// Classification
	classify.Classify(rootNode)

	prog.finished.Store(true)
	return rootNode, nil
}

func scanDir(
	dirPath string,
	node *tree.Node,
	rootDev uint64,
	currentDepth int,
	opts ScanOptions,
	prog *Progress,
	tokens chan struct{},
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	if prog.IsCancelled() {
		return
	}

	f, err := os.Open(dirPath)
	if err != nil {
		node.ReadError = true
		prog.AddError(dirPath)
		return
	}
	defer f.Close()

	// Read entries
	entries, err := f.Readdir(-1)
	if err != nil {
		node.ReadError = true
		prog.AddError(dirPath)
		return
	}

	type subDirTask struct {
		path string
		node *tree.Node
	}
	var subTasks []subDirTask

	var localFiles, localDirs, localBytes uint64
	localDirs = 1 // count this directory

	for _, entry := range entries {
		if prog.IsCancelled() {
			return
		}

		name := entry.Name()
		if !opts.IncludeHidden && len(name) > 0 && name[0] == '.' {
			continue
		}

		entryPath := filepath.Join(dirPath, name)
		var st syscall.Stat_t
		var statErr error
		if opts.FollowLinks {
			statErr = syscall.Stat(entryPath, &st)
		} else {
			statErr = syscall.Lstat(entryPath, &st)
		}

		if statErr != nil {
			prog.AddError(entryPath)
			continue
		}

		mode := st.Mode
		isDir := (mode & syscall.S_IFMT) == syscall.S_IFDIR
		isSym := (mode & syscall.S_IFMT) == syscall.S_IFLNK
		isReg := (mode & syscall.S_IFMT) == syscall.S_IFREG

		// Filesystem boundary check
		if opts.OneFilesystem && uint64(st.Dev) != rootDev {
			continue
		}

		bytes := uint64(st.Blocks * 512)
		if opts.ApparentSize {
			bytes = uint64(st.Size)
		}

		if isDir {
			subNode := tree.NewDirectory(name)
			subNode.Dev = uint64(st.Dev)
			subNode.Ino = uint64(st.Ino)
			subNode.HasInode = true
			subNode.Modified = st.Mtim.Sec
			node.Children = append(node.Children, subNode)

			if opts.MaxDepth == 0 || currentDepth+1 < opts.MaxDepth {
				subTasks = append(subTasks, subDirTask{path: entryPath, node: subNode})
			}
		} else {
			kind := tree.File
			if isSym {
				kind = tree.Symlink
			} else if !isReg {
				kind = tree.Other
			}

			child := tree.NewEntry(name, kind, bytes)
			child.Dev = uint64(st.Dev)
			child.Ino = uint64(st.Ino)
			child.HasInode = true
			child.Modified = st.Mtim.Sec

			node.Children = append(node.Children, child)
			localBytes += bytes
			if isReg {
				localFiles++
			}
		}
	}

	prog.Add(localFiles, localDirs, localBytes)

	// Launch subdirectory tasks
	for _, task := range subTasks {
		select {
		case tokens <- struct{}{}:
			// Run concurrently in goroutine
			wg.Add(1)
			go func(t subDirTask) {
				scanDir(t.path, t.node, rootDev, currentDepth+1, opts, prog, tokens, wg)
				<-tokens
			}(task)
		default:
			// Run inline to limit goroutines and keep stack usage bounded
			wg.Add(1)
			scanDir(task.path, task.node, rootDev, currentDepth+1, opts, prog, tokens, wg)
		}
	}
}
