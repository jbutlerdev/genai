package tools

// Security regression tests for the go-git advisories reachable from
// tools/gitOperations.go (govulncheck traces on genai main):
//
//	GO-2026-6213 (GHSA-hc8v-wwc9-vgxm, HIGH) — worktree operations may follow
//	  symlinks. Traced: tools.RevertFile -> wt.Filesystem.Remove.
//	GO-2026-6214 (GHSA-qgq7-7hm3-q39j, MODERATE) — path traversal via crafted
//	  reference names may modify files outside the reference storage.
//	  Traced: tools.RevertFileWrapper -> git.PlainOpen -> dotgit.DotGit.Ref.
//
// These tests FAIL on go-git v5.19.1 (vulnerable) and PASS on v5.19.2 (fixed).
// The discriminator is observable filesystem state, not error strings:
// on the vulnerable version the escape actually happens (external file
// deleted / file written outside .git); on the fixed version it is refused.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// initRepo creates a fresh on-disk git repository.
func initRepo(t *testing.T, dir string) *git.Repository {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("git.PlainInit: %v", err)
	}
	return repo
}

// TestRevertFileRefusesSymlinkEscape covers GO-2026-6213 via the traced call
// chain tools.RevertFile -> repo.Worktree() -> wt.Filesystem.Remove.
//
// Setup: an external directory containing target.txt, and inside the repo a
// symlink "link" -> that external directory. RevertFile is asked to remove
// "link/target.txt". On the vulnerable version the kernel resolves the symlink
// and the EXTERNAL target is deleted (root escape). The fixed version must
// refuse: the external file must survive.
func TestRevertFileRefusesSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	target := filepath.Join(outside, "target.txt")
	if err := os.WriteFile(target, []byte("must-not-be-deletable-via-repo-symlink"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	repoDir := filepath.Join(base, "repo")
	repo := initRepo(t, repoDir)
	if err := os.Symlink(outside, filepath.Join(repoDir, "link")); err != nil {
		t.Fatalf("os.Symlink: %v", err)
	}

	err := RevertFile(repo, "link/target.txt")
	t.Logf("RevertFile(\"link/target.txt\") err=%v", err)

	if _, statErr := os.Stat(target); os.IsNotExist(statErr) {
		t.Fatalf("GO-2026-6213 REGRESSION: RevertFile followed the in-repo symlink and DELETED the external file %s (worktree filesystem escaped its root). err=%v", target, err)
	}
	t.Logf("external target survived (escape refused)")
}

// TestRevertFileWrapperRefusesSymlinkEscape covers the same advisory through
// the exported tool entrypoint RevertFileWrapper (which exercises the other
// traced edge: git.PlainOpen on attacker-influenced basePath).
func TestRevertFileWrapperRefusesSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	target := filepath.Join(outside, "target.txt")
	if err := os.WriteFile(target, []byte("must-survive"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	repoDir := filepath.Join(base, "repo")
	initRepo(t, repoDir)
	if err := os.Symlink(outside, filepath.Join(repoDir, "link")); err != nil {
		t.Fatalf("os.Symlink: %v", err)
	}

	res, err := RevertFileWrapper(map[string]any{
		"basePath": repoDir,
		"file":     "link/target.txt",
	})
	t.Logf("RevertFileWrapper err=%v result=%v", err, res)

	if _, statErr := os.Stat(target); os.IsNotExist(statErr) {
		t.Fatalf("GO-2026-6213 REGRESSION: RevertFileWrapper followed the in-repo symlink and DELETED the external file %s", target)
	}
	t.Logf("external target survived (escape refused)")
}

// TestSetRefRejectsTraversalRefName covers GO-2026-6214 at the dotgit storage
// layer reached from the traced chain (PlainOpen -> dotgit). A crafted
// reference name containing ../.. segments must not be able to write a loose
// ref file OUTSIDE the repository's ref storage (.git). The repo is placed
// deep enough that a bounded number of ".." lands deterministically inside
// the test's base dir instead of clamping at the filesystem root.
func TestSetRefRejectsTraversalRefName(t *testing.T) {
	base := t.TempDir()
	repoDir := filepath.Join(base, "r1", "r2", "r3", "r4", "repo")
	repo := initRepo(t, repoDir)

	// Loose refs resolve as <repoDir>/.git/<name>. From .git/refs/heads,
	// 8 levels up == base. So the crafted name writes base/escape6214 if
	// the traversal is honoured.
	const up = 8
	crafted := plumbing.ReferenceName("refs/heads/" + strings.Repeat("../", up) + "escape6214")
	escaped := filepath.Join(base, "escape6214")

	err := repo.Storer.SetReference(plumbing.NewHashReference(crafted, plumbing.ZeroHash))
	t.Logf("SetReference(crafted) err=%v", err)

	if _, statErr := os.Stat(escaped); statErr == nil {
		body, _ := os.ReadFile(escaped)
		t.Fatalf("GO-2026-6214 REGRESSION: SetRef with crafted name %q wrote file OUTSIDE ref storage at %s (content=%q)", string(crafted), escaped, string(body))
	}
	if err == nil {
		t.Logf("note: SetRef accepted the name but no file escaped (storage normalisation held)")
	}
	t.Logf("no file written outside ref storage (traversal refused)")
}
