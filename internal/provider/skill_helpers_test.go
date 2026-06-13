package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWalkSkillDir_rejectsSymlink is a security regression test: a symlink
// inside source_dir must NOT be followed, otherwise walkSkillDir would read
// the symlink's target (a file outside source_dir, e.g. ~/.ssh/id_rsa) and
// upload its contents. The walk must error instead.
func TestWalkSkillDir_rejectsSymlink(t *testing.T) {
	// A "secret" living outside the skill dir.
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("TOPSECRET"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "leak.txt")); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}

	files, err := walkSkillDir(dir)
	if err == nil {
		t.Fatalf("expected error for symlink in source_dir, got nil (files=%v)", files)
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink, got: %v", err)
	}
}

// TestWalkSkillDir_regularFilesOK confirms the symlink rejection is specific
// to symlinks and does not break a normal directory of regular files.
func TestWalkSkillDir_regularFilesOK(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "ref.md"), []byte("ref"), 0o644); err != nil {
		t.Fatalf("write sub/ref.md: %v", err)
	}

	files, err := walkSkillDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := make([]string, 0, len(files))
	for _, f := range files {
		got = append(got, f.Path)
	}
	want := []string{"SKILL.md", "sub/ref.md"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("walked files = %v, want %v", got, want)
	}
}
