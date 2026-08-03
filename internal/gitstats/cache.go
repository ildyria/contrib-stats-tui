package gitstats

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

// Version identifies the build of the binary. It defaults to "dev" and can be
// overridden at build time, e.g.:
//
//	go build -ldflags "-X github.com/ildyria/contrib-stats-tui/internal/gitstats.Version=$(git rev-parse HEAD)"
var Version = "dev"

// BuildID returns a stable fingerprint of the current binary build. Caches are
// tied to this value so that any change to the binary (new commit, rebuild, or
// different Go toolchain) transparently invalidates previously written caches,
// removing the need to version the on-disk cache format.
func BuildID() string {
	parts := []string{Version, runtime.Version()}
	if bi, ok := debug.ReadBuildInfo(); ok {
		parts = append(parts, bi.Main.Version)
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				parts = append(parts, "rev="+s.Value)
			case "vcs.modified":
				parts = append(parts, "modified="+s.Value)
			}
		}
	}
	// Executable size + modtime guarantees rebuilds invalidate the cache even
	// when no VCS build info is embedded (e.g. plain `go build`).
	if exe, err := os.Executable(); err == nil {
		if fi, err := os.Stat(exe); err == nil {
			parts = append(parts, fmt.Sprintf("%d-%d", fi.Size(), fi.ModTime().UnixNano()))
		}
	}
	sum := sha1.Sum([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

type cacheFile struct {
	Build       string     `json:"build"`
	Repo        string     `json:"repo"`
	Head        string     `json:"head"`
	Ignore      []string   `json:"ignore"`
	ExcludeDocs bool       `json:"exclude_docs"`
	Identities  []Identity `json:"identities,omitempty"`
	Window      Window     `json:"window,omitempty"`
	Summary     *Summary   `json:"summary"`
}

// CollectCached returns statistics for repoPath, using an on-disk cache keyed
// by the repository's current HEAD commit. When useCache is true and a cache
// exists whose recorded HEAD matches the repository's current HEAD, the cached
// result is returned without rescanning. Otherwise the repository is scanned
// and the result is written back to the cache.
//
// The ignore list (names/emails whose commits are excluded), the excludeDocs
// flag and the identity aggregation list are part of the cache key, so changing
// any of them transparently invalidates a stale cache.
//
// The second return value reports whether the result came from the cache.
func CollectCached(repoPath string, useCache bool, ignore []string, excludeDocs bool, identities []Identity, window Window, progress func(done, total int)) (*Summary, bool, error) {
	top, err := repoRoot(repoPath)
	if err != nil {
		return nil, false, err
	}
	head := HeadCommit(top)
	path := cachePath(top)
	build := BuildID()
	normIgnore := NormalizeIgnore(ignore)
	normIdentities := NormalizeIdentities(identities)

	if useCache && head != "" && path != "" {
		if cf, err := loadCache(path); err == nil &&
			cf.Build == build && cf.Head == head && cf.Summary != nil &&
			cf.ExcludeDocs == excludeDocs &&
			cf.Window == window &&
			sameStrings(cf.Ignore, normIgnore) &&
			sameIdentities(cf.Identities, normIdentities) {
			return cf.Summary, true, nil
		}
	}

	sum, err := CollectWithProgress(repoPath, ignore, excludeDocs, identities, window, progress)
	if err != nil {
		return nil, false, err
	}

	if useCache && head != "" && path != "" {
		_ = saveCache(path, &cacheFile{
			Build:       build,
			Repo:        top,
			Head:        head,
			Ignore:      normIgnore,
			ExcludeDocs: excludeDocs,
			Identities:  normIdentities,
			Window:      window,
			Summary:     sum,
		})
	}
	return sum, false, nil
}

// sameStrings reports whether two string slices are element-wise equal.
func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sameIdentities reports whether two normalized identity lists are equal. Both
// are expected to have been produced by NormalizeIdentities so their order and
// contents are canonical.
func sameIdentities(a, b []Identity) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].DisplayName != b[i].DisplayName ||
			a[i].DisplayEmail != b[i].DisplayEmail ||
			!sameStrings(a[i].Emails, b[i].Emails) ||
			!sameStrings(a[i].Usernames, b[i].Usernames) {
			return false
		}
	}
	return true
}

// HeadCommit returns the full SHA of the repository's current HEAD, or "" if it
// cannot be determined (e.g. an empty repository).
func HeadCommit(top string) string {
	cmd := exec.Command("git", "-C", top, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// cachePath returns the cache file path for the given repository root, or ""
// if a cache directory cannot be resolved.
func cachePath(top string) string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	sum := sha1.Sum([]byte(top))
	name := hex.EncodeToString(sum[:]) + ".json"
	return filepath.Join(dir, "contributors", name)
}

func loadCache(path string) (*cacheFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("decoding cache: %w", err)
	}
	return &cf, nil
}

func saveCache(path string, cf *cacheFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(cf)
	if err != nil {
		return err
	}
	// Write atomically to avoid corrupt caches on interruption.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
