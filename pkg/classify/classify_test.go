package classify

import (
	"github.com/KartikeyaKotkar/treedisk/pkg/tree"
	"testing"
)

func TestClassifyBasic(t *testing.T) {
	root := tree.NewDirectory("home")
	src := tree.NewDirectory("src")
	cache := tree.NewDirectory(".cache")
	git := tree.NewDirectory(".git")

	root.Children = append(root.Children, src, cache, git)
	tree.Aggregate(root, tree.Bytes)
	Classify(root)

	if src.Category != tree.Code {
		t.Errorf("expected src to be Code, got %v", src.Category)
	}
	if cache.Category != tree.Cache || cache.Reclaim != tree.Regenerable {
		t.Errorf("expected .cache to be Cache + Regenerable, got %v / %v", cache.Category, cache.Reclaim)
	}
	if git.Category != tree.Git {
		t.Errorf("expected .git to be Git, got %v", git.Category)
	}
}

func TestClassifySteamLibrary(t *testing.T) {
	root := tree.NewDirectory("data")
	lib := tree.NewDirectory("SteamLibrary")
	steamapps := tree.NewDirectory("steamapps")
	temp := tree.NewDirectory("temp")
	temp.Children = append(temp.Children, tree.NewEntry("partial", tree.File, 1))
	common := tree.NewDirectory("common")
	common.Children = append(common.Children, tree.NewEntry("game.pak", tree.File, 1000))

	steamapps.Children = append(steamapps.Children, temp, common)
	lib.Children = append(lib.Children, steamapps)
	root.Children = append(root.Children, lib)

	tree.Aggregate(root, tree.Bytes)
	Classify(root)

	if lib.Category != tree.Media {
		t.Errorf("SteamLibrary should be Media, got %v", lib.Category)
	}
}

func TestReclaimSiblingRules(t *testing.T) {
	project := tree.NewDirectory("my-rust-project")
	cargoToml := tree.NewEntry("Cargo.toml", tree.File, 100)
	targetDir := tree.NewDirectory("target")
	targetDir.Children = append(targetDir.Children, tree.NewEntry("binary", tree.File, 5000))

	project.Children = append(project.Children, cargoToml, targetDir)
	tree.Aggregate(project, tree.Bytes)
	Classify(project)

	if targetDir.Reclaim != tree.BuildOutput {
		t.Errorf("expected target beside Cargo.toml to be BuildOutput, got %v", targetDir.Reclaim)
	}
}
