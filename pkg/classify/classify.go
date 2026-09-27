package classify

import (
	"github.com/KartikeyaKotkar/treedisk/pkg/tree"
	"strings"
)

// CategoryOfName returns the category a directory name announces on its own.
func CategoryOfName(name string) (tree.Category, bool) {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "onedrive - ") || strings.HasPrefix(lower, "dropbox (") {
		return tree.Synced, true
	}

	switch lower {
	case "src", "code", "projects", "repos", "dev", "work",
		"workspace", "workspaces", "github.com", "gitlab.com",
		"sites", "development":
		return tree.Code, true

	case ".codex", ".claude", ".herdr", ".pi", ".cursor", ".aider",
		".gemini", ".continue", ".windsurf", ".microsandbox", ".omp",
		".agents", ".openai", "tries", "worktrees", "experiments",
		"scratch", "playground":
		return tree.AgentScratch, true

	case ".cargo", ".rustup", ".local", ".npm", ".pnpm-store", "pnpm",
		".bun", ".deno", "go", ".gradle", ".m2", ".platformio",
		"mise", ".mise", ".pyenv", ".nvm", ".gem", "gem", ".rbenv",
		".espressif", ".arduino15", ".config", ".vscode", ".zig",
		".rye", ".conda", "anaconda3", "miniconda3", ".opam",
		".ghcup", ".stack", ".julia", ".dotnet", ".android",
		".sdkman", ".volta", ".yarn", ".java", ".nuget",
		"xcode", "coresimulator":
		return tree.Toolchain, true

	case "sync", "dropbox", "nextcloud", "google drive", "onedrive",
		"pclouddrive", "mega", ".stversions",
		"mobile documents", "cloudstorage", "iclouddrive":
		return tree.Synced, true

	case ".git":
		return tree.Git, true

	case "pictures", "photos", "music", "videos", "movies", "steam",
		"steamlibrary", "steamapps", "emulation",
		"models", ".ollama", ".lmstudio", "games", "wineprefix":
		return tree.Media, true

	case "documents", "desktop", "downloads", "books", "notes",
		"obsidian", "public", "templates":
		return tree.Documents, true

	case ".cache", "cache", "caches", ".ccache", ".sccache", "_cacache",
		"__pycache__", "node_modules", "trash", ".trash", "tmp",
		".tmp", "deriveddata", "ios devicesupport",
		"watchos devicesupport", "temp", "$recycle.bin", "npm-cache",
		"v3-cache", "inetcache", "d3dscache", "dxcache", "glcache", "crashdumps":
		return tree.Cache, true
	}

	return tree.OtherCat, false
}

// ReclaimOf judges whether a directory's space can be had back.
func ReclaimOf(name string, parent tree.Category, hasSibling func(string) bool) (tree.Reclaim, bool) {
	lower := strings.ToLower(name)
	switch lower {
	case ".cache", "cache", "caches", ".ccache", ".sccache", "_cacache",
		"npm-cache", "v3-cache", "inetcache", "d3dscache", "dxcache",
		"glcache", "ios devicesupport", "watchos devicesupport":
		return tree.Regenerable, true

	case ".stversions":
		return tree.SyncHistory, true

	case ".pnpm-store", "pnpm":
		return tree.PackageStore, true

	case "__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache",
		".next", ".turbo", ".parcel-cache", "deriveddata":
		return tree.BuildOutput, true

	case "logs":
		if hasSibling("Application Support") {
			return tree.Temporary, true
		}

	case "target":
		if hasSibling("Cargo.toml") {
			return tree.BuildOutput, true
		}

	case "node_modules":
		if hasSibling("package.json") {
			return tree.Reinstallable, true
		}

	case "layers":
		if parent == tree.AgentScratch {
			return tree.SandboxLayers, true
		}

	case "snapshots":
		if parent == tree.AgentScratch {
			return tree.Snapshots, true
		}

	case "trash", ".trash", "$recycle.bin":
		return tree.Trash, true

	case "tmp", ".tmp", "temp", "crashdumps":
		return tree.Temporary, true
	}

	return tree.ReclaimNone, false
}

// Classify assigns category and reclaim attributes to every node in the tree.
func Classify(root *tree.Node) {
	root.Category = tree.OtherCat
	root.Reclaim = tree.ReclaimNone

	siblings := root.Children
	hasSibling := func(wanted string) bool {
		for _, s := range siblings {
			if s.Name == wanted {
				return true
			}
		}
		return false
	}

	for i := range root.Children {
		child := root.Children[i]
		cat, ok := CategoryOfName(child.Name)
		if !ok {
			if IsGitStore(child) {
				cat = tree.Git
			} else if domCat, domOk := DominantChildCategory(child); domOk {
				cat = domCat
			} else {
				cat = tree.OtherCat
			}
		}

		reclaim := tree.ReclaimNone
		if child.IsDir() {
			if r, rok := ReclaimOf(child.Name, tree.OtherCat, hasSibling); rok {
				reclaim = r
			}
		}

		classifyBelow(child, cat, reclaim)
	}
}

func classifyBelow(node *tree.Node, cat tree.Category, reclaim tree.Reclaim) {
	node.Category = cat
	node.Reclaim = reclaim

	hasDir := false
	for _, child := range node.Children {
		if child.IsDir() {
			hasDir = true
			break
		}
	}

	if !hasDir {
		for _, child := range node.Children {
			child.Category = cat
			child.Reclaim = reclaim
		}
		return
	}

	hasSibling := func(wanted string) bool {
		for _, s := range node.Children {
			if s.Name == wanted {
				return true
			}
		}
		return false
	}

	for _, child := range node.Children {
		if !child.IsDir() {
			child.Category = cat
			child.Reclaim = reclaim
			continue
		}

		childCat := cat
		if c, ok := CategoryOfName(child.Name); ok {
			childCat = c
		} else if IsGitStore(child) {
			childCat = tree.Git
		}

		childReclaim := reclaim
		if childReclaim == tree.ReclaimNone {
			if r, ok := ReclaimOf(child.Name, cat, hasSibling); ok {
				childReclaim = r
			}
		}

		classifyBelow(child, childCat, childReclaim)
	}
}

// DominantChildCategory returns the category of the largest recognisable child down a few levels.
func DominantChildCategory(node *tree.Node) (tree.Category, bool) {
	curr := node
	for step := 0; step < 3; step++ {
		for _, child := range curr.Children {
			if child.IsDir() {
				if c, ok := CategoryOfName(child.Name); ok {
					return c, true
				}
				if IsGitStore(child) {
					return tree.Git, true
				}
			}
		}
		// Descend into first dir child (already sorted largest first)
		var nextDir *tree.Node
		for _, child := range curr.Children {
			if child.IsDir() {
				nextDir = child
				break
			}
		}
		if nextDir == nil {
			break
		}
		curr = nextDir
	}
	return tree.OtherCat, false
}

// IsGitStore checks if node looks like a git store (has objects, refs, HEAD).
func IsGitStore(node *tree.Node) bool {
	if !node.IsDir() {
		return false
	}
	hasObjects, hasRefs, hasHead := false, false, false
	for _, child := range node.Children {
		switch child.Name {
		case "objects":
			hasObjects = true
		case "refs":
			hasRefs = true
		case "HEAD":
			hasHead = true
		}
	}
	return hasObjects && hasRefs && hasHead
}
