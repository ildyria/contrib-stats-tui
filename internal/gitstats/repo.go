package gitstats

// repo.go resolves repository specifications (from the `repositories:` config
// list) into local paths that the collector can scan. A spec is either a local
// path (absolute, or relative to the config file) or a clone URL. Clone URLs
// are cloned into a per-user cache directory and refreshed with `git fetch` on
// later runs.

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RepoSpec is a resolved repository target. Raw is the original entry from the
// config (or command line); DisplayName is a short human-readable label shown
// in the UI; LocalPath is where the repository lives on disk (empty until a
// clone spec has been materialized); IsClone reports whether Raw is a clone URL
// rather than a local path.
type RepoSpec struct {
	Raw         string
	DisplayName string
	LocalPath   string
	IsClone     bool
}

// IsCloneURL reports whether s looks like a git clone URL rather than a local
// filesystem path. It recognizes scp-like syntax (git@host:org/repo), any
// scheme://… URL, and paths ending in .git.
func IsCloneURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.Contains(s, "://") {
		return true
	}
	if strings.HasPrefix(s, "git@") {
		return true
	}
	// scp-like "host:path" without a scheme, e.g. server:org/repo.git — treat a
	// trailing .git as a strong signal of a remote.
	if strings.HasSuffix(s, ".git") && !strings.HasPrefix(s, ".") &&
		!strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "~") {
		return true
	}
	return false
}

// IsGitRepo reports whether path is inside a git working tree.
func IsGitRepo(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

// cloneDisplayName derives a short label from a clone URL, using the last path
// segment with any trailing ".git" removed.
func cloneDisplayName(url string) string {
	s := strings.TrimSpace(url)
	s = strings.TrimSuffix(s, "/")
	// Strip scp-like "git@host:" or "host:" prefixes before taking the base.
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(s, ".git")
	if s == "" {
		return url
	}
	return s
}

// ResolveRepos classifies each raw entry as a local path or a clone URL and
// fills in the DisplayName. Relative local paths are resolved against configDir
// (the directory of the .contributors.yaml file); if configDir is empty they
// are resolved against the current working directory. Clone URLs are left with
// an empty LocalPath — call CloneOrUpdate to materialize them.
func ResolveRepos(raw []string, configDir string) []RepoSpec {
	specs := make([]RepoSpec, 0, len(raw))
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if IsCloneURL(r) {
			specs = append(specs, RepoSpec{
				Raw:         r,
				DisplayName: cloneDisplayName(r),
				IsClone:     true,
			})
			continue
		}
		path := r
		if !filepath.IsAbs(path) {
			base := configDir
			if base == "" {
				base, _ = os.Getwd()
			}
			path = filepath.Join(base, path)
		}
		specs = append(specs, RepoSpec{
			Raw:         r,
			DisplayName: filepath.Base(filepath.Clean(path)),
			LocalPath:   path,
		})
	}
	return specs
}

// cloneCacheDir returns the directory under which remote repositories are
// cloned, creating a stable per-URL subdirectory name.
func cloneCacheDir(url string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	sum := sha1.Sum([]byte(url))
	name := cloneDisplayName(url) + "-" + hex.EncodeToString(sum[:])[:12]
	return filepath.Join(base, "contributors", "clones", name), nil
}

// CloneOrUpdate ensures a clone spec is present on disk and reasonably fresh,
// returning the local path to scan. If the clone does not yet exist it is
// cloned; if it already exists a best-effort `git fetch` refreshes it (fetch
// errors are ignored so stale clones remain usable offline). The returned error
// is non-nil only when the repository could not be made available at all.
func CloneOrUpdate(spec RepoSpec) (string, error) {
	dir, err := cloneCacheDir(spec.Raw)
	if err != nil {
		return "", fmt.Errorf("resolving clone cache dir: %w", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		// Already cloned: refresh in the background best-effort.
		fetch := exec.Command("git", "-C", dir, "fetch", "--all", "--prune", "--quiet")
		_ = fetch.Run()
		return dir, nil
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", fmt.Errorf("creating clone cache dir: %w", err)
	}
	// Remove any partial leftover before cloning fresh.
	_ = os.RemoveAll(dir)
	clone := exec.Command("git", "clone", "--quiet", spec.Raw, dir)
	var stderr strings.Builder
	clone.Stderr = &stderr
	if err := clone.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("cloning %s: %s", spec.Raw, msg)
		}
		return "", fmt.Errorf("cloning %s: %w", spec.Raw, err)
	}
	return dir, nil
}
