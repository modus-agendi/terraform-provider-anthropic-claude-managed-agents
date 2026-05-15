package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/andasv/terraform-provider-anthropic-claude-managed-agents/internal/client"
)

// Provider-side skill upload limits. These mirror the client-side constants
// in internal/client/skill.go so plan-time errors fire before any network
// call. Duplicating the constant — rather than importing it — keeps the
// helper file's dependency surface narrow.
const (
	maxSkillUploadBytesProvider = 30 * 1024 * 1024
	skillEntrypointFile         = "SKILL.md"
)

// excludedSkillBasenames are filesystem cruft that should never be part of a
// skill upload. Kept intentionally minimal — anything more elaborate (e.g.
// arbitrary .gitignore parsing) is deferred per spec §11.4.
var excludedSkillBasenames = map[string]struct{}{
	".DS_Store": {},
}

// excludedSkillDirNames are directory names whose entire contents are
// skipped. Top-level matches only; we don't walk into them.
var excludedSkillDirNames = map[string]struct{}{
	".git":        {},
	"__pycache__": {},
}

// walkSkillDir walks a local directory and returns the deterministic file
// list ready to upload. Implements spec Appendix A.
//
// The slice is sorted ascendingly by relative POSIX path so the content hash
// is stable across filesystems with different walk orders. SKILL.md must be
// present at the top level — that's enforced by validateSkillFilesProvider
// after the walk, not here, so callers can produce a clearer error when the
// file is missing.
func walkSkillDir(dir string) ([]client.SkillFile, error) {
	if dir == "" {
		return nil, fmt.Errorf("source_dir is empty")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("stat source_dir %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source_dir %q is not a directory", dir)
	}

	var files []client.SkillFile
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip whole subtrees we don't care about.
			if path == dir {
				return nil
			}
			if _, skip := excludedSkillDirNames[d.Name()]; skip {
				return filepath.SkipDir
			}
			return nil
		}
		if _, skip := excludedSkillBasenames[d.Name()]; skip {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("rel %q: %w", path, err)
		}
		rel = filepath.ToSlash(rel)
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %q: %w", path, err)
		}
		files = append(files, client.SkillFile{Path: rel, Content: b})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// canonicalSkillHash computes a deterministic sha256 over the walked files
// plus the rotation counter. The output is the bare hex digest (no `sha256:`
// prefix) so callers can present it however they want.
//
// Hash input shape:
//
//	for each file in sorted order:
//	  path bytes + NUL + content bytes + NUL
//	then: "|rotation:N"
//
// The NUL separators prevent the trivial "file_a"+"x" / "file_a x" collision.
func canonicalSkillHash(files []client.SkillFile, rotation int64) string {
	h := sha256.New()
	for _, f := range files {
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write(f.Content)
		h.Write([]byte{0})
	}
	fmt.Fprintf(h, "|rotation:%d", rotation)
	return hex.EncodeToString(h.Sum(nil))
}

// validateSkillFilesProvider enforces the three plan-time invariants we want
// to surface as Terraform errors rather than letting the API reject them
// with a 4xx that's harder to read.
func validateSkillFilesProvider(files []client.SkillFile) error {
	if len(files) == 0 {
		return fmt.Errorf("source_dir is empty; at least one file is required")
	}
	var (
		total          int
		foundEntryFile bool
	)
	for _, f := range files {
		total += len(f.Content)
		if f.Path == skillEntrypointFile {
			foundEntryFile = true
		}
	}
	if !foundEntryFile {
		return fmt.Errorf("SKILL.md is required at the root of source_dir")
	}
	if total > maxSkillUploadBytesProvider {
		return fmt.Errorf("source_dir total size %d bytes exceeds 30 MB limit", total)
	}
	return nil
}

// resolveSkillSourceDir expands relative paths the way Terraform expects:
// relative to the working directory. Symlinks are followed by filepath.WalkDir
// implicitly. Strips a trailing slash so the displayed path stays clean.
func resolveSkillSourceDir(in string) string {
	if in == "" {
		return in
	}
	clean := filepath.Clean(in)
	return strings.TrimRight(clean, string(os.PathSeparator))
}
