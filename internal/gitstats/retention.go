package gitstats

// retention.go computes a per-contributor "retention" score: the number of
// lines a contributor added, weighted by how long those lines survived in the
// codebase. Lines that are still present in the current HEAD count with the
// maximum weight; lines that were later removed count in proportion to how long
// they lived before being deleted.
//
// Two ponderations are produced:
//
//   - by-commit: a line's lifespan is measured in commits (how many commits it
//     survived, relative to how many it could have survived).
//   - by-day: a line's lifespan is measured in days. A line that is removed on
//     the same day it was added does not count (its survival is zero).
//
// The score is a pure per-contributor quantity (it does not slide with time),
// so it serves as a lasting "contribution" measure rather than an activity
// series.
//
// Line lifetimes are reconstructed by replaying the first-parent history
// oldest-first and applying each commit's unified diff (with zero context) to a
// per-file array that records, for every current line, the index of the commit
// that introduced it. When lines are removed, the difference between the current
// commit and their birth commit gives their lifespan. Using --first-parent
// keeps the reconstruction self-consistent on branchy histories; the trade-off
// is that squashed/rebased workflows attribute lines precisely while old
// merge-commit workflows attribute merged lines to the commit that merged them.

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// retentionResult holds a single contributor's accumulated retention score.
type retentionResult struct {
	commits float64 // retention-weighted added lines, by-commit ponderation
	days    float64 // retention-weighted added lines, by-day ponderation
	added   int     // total tracked added lines (the denominator for a rate)
	alive   int     // added lines still present in the current HEAD
	files   int     // distinct files the contributor added/removed lines in
	dirs    int     // distinct directories those files live in
}

// retentionGlobals holds repository-wide metrics derived from the line-lifetime
// reconstruction: the median age at which overwritten lines died and the median
// time contributors take to land their first surviving change.
type retentionGlobals struct {
	medianLifetimeDays   int
	medianOnboardingDays int
}

// hunkRe matches a unified-diff hunk header, e.g. "@@ -12,3 +12,5 @@". With
// --unified=0 the counts are exactly the number of removed/added lines.
var hunkRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// retSection tracks the state of a single file's diff within one commit.
type retSection struct {
	oldPath, newPath, key string
	renameFrom, renameTo  string
	isNew, isDelete       bool
	isBinary, isCopy      bool
	prepared              bool
	skip                  bool
	delta                 int // running (added-removed) offset across hunks
}

// retTracker reconstructs line lifetimes while streaming a first-parent patch
// log. Per-commit slices are indexed by commit index (0 == oldest); the death
// and survival aggregates are indexed by a line's birth commit.
type retTracker struct {
	files   map[string][]int32
	email   []string // author email per commit index
	dayNum  []int64  // day number (unix/86400) per commit index
	died    []int64  // sum of (death-birth) commit-distances, by birth index
	diedDay []int64  // sum of survived days, by birth index
	alive   []int64  // lines still alive at HEAD, by birth index
	added   []int64  // lines added, by birth (commit) index
	cur     int
	sec     *retSection
	// lifeHist buckets the age in days of every line that was overwritten (the
	// "time-to-legacy" distribution); lifeCount is the total number of deaths.
	lifeHist  []int
	lifeCount int
	// touched maps an author email to the set of file paths in which they added
	// or removed lines, powering the collaboration-breadth metric.
	touched map[string]map[string]struct{}
	// excludeDocs drops documentation files (Markdown or docs/ paths) from the
	// line-lifetime reconstruction when set.
	excludeDocs bool
}

// computeRetention replays repository history and returns per-email retention
// scores. It is best-effort: any git failure is surfaced as an error so callers
// can choose to ignore it and continue without retention data.
func computeRetention(top string, total int, excludeDocs bool, extraArgs []string, progress func(done, total int)) (map[string]retentionResult, retentionGlobals, error) {
	format := "C" + recordSep + "%an" + recordSep + "%ae" + recordSep + "%at"
	args := []string{"-C", top, "log",
		"--first-parent", "--reverse", "-p", "--unified=0",
		"--no-color", "-M", "--format=" + format,
	}
	args = append(args, extraArgs...)
	cmd := exec.Command("git", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, retentionGlobals{}, fmt.Errorf("creating git pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, retentionGlobals{}, fmt.Errorf("starting git log -p: %w", err)
	}

	t := &retTracker{
		files:       make(map[string][]int32),
		cur:         -1,
		lifeHist:    make([]int, len(lineLifetimeBuckets)+1),
		touched:     make(map[string]map[string]struct{}),
		excludeDocs: excludeDocs,
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)
	seen := 0
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "C"+recordSep) {
			t.closeSection()
			fields := strings.Split(strings.TrimPrefix(line, "C"+recordSep), recordSep)
			if len(fields) < 3 {
				continue
			}
			ts, _ := strconv.ParseInt(fields[2], 10, 64)
			t.onCommit(strings.ToLower(fields[1]), ts/86400)
			seen++
			if progress != nil && seen%64 == 0 {
				progress(seen, total)
			}
			continue
		}
		if strings.HasPrefix(line, "diff --git ") {
			t.closeSection()
			t.sec = &retSection{}
			continue
		}
		if t.sec == nil || t.cur < 0 {
			continue
		}
		t.line(line)
	}
	t.closeSection()

	if err := sc.Err(); err != nil {
		return nil, retentionGlobals{}, fmt.Errorf("scanning git output: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return nil, retentionGlobals{}, fmt.Errorf("git log -p failed: %s", strings.TrimSpace(stderr.String()))
	}
	if progress != nil {
		progress(seen, total)
	}
	res, glob := t.finalize()
	return res, glob, nil
}

// onCommit advances to a new commit, extending the per-commit slices.
func (t *retTracker) onCommit(email string, dayNum int64) {
	t.cur++
	t.email = append(t.email, email)
	t.dayNum = append(t.dayNum, dayNum)
	t.died = append(t.died, 0)
	t.diedDay = append(t.diedDay, 0)
	t.alive = append(t.alive, 0)
	t.added = append(t.added, 0)
}

// line processes one line of diff output within the current file section.
func (t *retTracker) line(line string) {
	s := t.sec
	if s.prepared {
		// Inside the hunk body: only new hunk headers matter; with
		// --unified=0 there are no context lines to track.
		if strings.HasPrefix(line, "@@ ") {
			t.applyHunk(line)
		}
		return
	}
	switch {
	case strings.HasPrefix(line, "@@ "):
		t.prepareSection()
		if !s.skip {
			t.applyHunk(line)
		}
	case strings.HasPrefix(line, "new file mode"):
		s.isNew = true
	case strings.HasPrefix(line, "deleted file mode"):
		s.isDelete = true
	case strings.HasPrefix(line, "rename from "):
		s.renameFrom = cleanRenamePath(line[len("rename from "):])
	case strings.HasPrefix(line, "rename to "):
		s.renameTo = cleanRenamePath(line[len("rename to "):])
	case strings.HasPrefix(line, "copy to "):
		s.isCopy = true
		s.renameTo = cleanRenamePath(line[len("copy to "):])
	case strings.HasPrefix(line, "Binary files "),
		strings.HasPrefix(line, "GIT binary patch"):
		s.isBinary = true
	case strings.HasPrefix(line, "--- "):
		s.oldPath = cleanDiffPath(line[4:])
	case strings.HasPrefix(line, "+++ "):
		s.newPath = cleanDiffPath(line[4:])
	}
}

// prepareSection resolves the file section's target path once all of its header
// lines have been seen (right before the first hunk, or at section close for
// hunkless sections such as pure renames).
func (t *retTracker) prepareSection() {
	s := t.sec
	if s.prepared {
		return
	}
	s.prepared = true
	if s.isBinary {
		s.skip = true
		return
	}
	if s.oldPath == "" && s.renameFrom != "" {
		s.oldPath = s.renameFrom
	}
	if s.newPath == "" && s.renameTo != "" {
		s.newPath = s.renameTo
	}
	switch {
	case s.isDelete:
		if s.oldPath == "" {
			s.skip = true
			return
		}
		s.key = s.oldPath
	case s.isNew || s.isCopy:
		if s.newPath == "" {
			s.skip = true
			return
		}
		s.key = s.newPath
		if _, ok := t.files[s.key]; !ok {
			t.files[s.key] = nil
		}
	default:
		// A rename (with or without edits) moves the ownership array.
		if s.oldPath != "" && s.newPath != "" && s.oldPath != s.newPath {
			if arr, ok := t.files[s.oldPath]; ok {
				t.files[s.newPath] = arr
				delete(t.files, s.oldPath)
			}
		}
		switch {
		case s.newPath != "":
			s.key = s.newPath
		case s.oldPath != "":
			s.key = s.oldPath
		default:
			s.skip = true
		}
	}
	// Honor --exclude-docs: documentation files never contribute to retention.
	if t.excludeDocs && s.key != "" && isDocPath(s.key) {
		s.skip = true
	}
}

// closeSection finalizes the current file section, dropping the ownership array
// for deleted files.
func (t *retTracker) closeSection() {
	if t.sec == nil {
		return
	}
	s := t.sec
	if !s.prepared {
		t.prepareSection()
	}
	if !s.skip && s.isDelete && s.key != "" {
		delete(t.files, s.key)
	}
	t.sec = nil
}

// applyHunk mutates the current file's ownership array according to one hunk
// header, recording the death of removed lines and the birth of added ones.
func (t *retTracker) applyHunk(line string) {
	s := t.sec
	if s.skip || s.key == "" {
		return
	}
	m := hunkRe.FindStringSubmatch(line)
	if m == nil {
		return
	}
	// The author of the current commit touched this file (added or removed
	// lines in it), feeding the collaboration-breadth metric.
	if e := t.email[t.cur]; e != "" {
		set := t.touched[e]
		if set == nil {
			set = make(map[string]struct{})
			t.touched[e] = set
		}
		set[s.key] = struct{}{}
	}
	oldStart, _ := strconv.Atoi(m[1])
	delCount := 1
	if m[2] != "" {
		delCount, _ = strconv.Atoi(m[2])
	}
	addCount := 1
	if m[4] != "" {
		addCount, _ = strconv.Atoi(m[4])
	}

	arr := t.files[s.key]
	var idx int
	if delCount == 0 {
		idx = oldStart + s.delta // insertion after oldStart lines
	} else {
		idx = oldStart - 1 + s.delta
	}
	if idx < 0 {
		idx = 0
	}
	if idx > len(arr) {
		idx = len(arr)
	}
	end := idx + delCount
	if end > len(arr) {
		end = len(arr)
	}
	for k := idx; k < end; k++ {
		t.recordDeath(int(arr[k]))
	}
	arr = append(arr[:idx], arr[end:]...)
	if addCount > 0 {
		arr = insertInt32(arr, idx, t.cur, addCount)
		t.added[t.cur] += int64(addCount)
	}
	t.files[s.key] = arr
	s.delta += addCount - delCount
}

// recordDeath books the lifespan of a line born at birth and removed at the
// current commit.
func (t *retTracker) recordDeath(birth int) {
	if birth < 0 || birth > t.cur {
		return
	}
	t.died[birth] += int64(t.cur - birth)
	age := t.dayNum[t.cur] - t.dayNum[birth]
	if age < 0 {
		age = 0
	}
	if age > 0 {
		t.diedDay[birth] += age
	}
	t.lifeHist[histBucket(int(age), lineLifetimeBuckets)]++
	t.lifeCount++
}

// finalize converts the accumulated per-birth statistics into per-email scores.
// Surviving lines (still present at HEAD) contribute the maximum weight of 1;
// removed lines contribute their survived fraction. The normalization
// denominator (how long a line born at a given commit could possibly have
// survived) depends only on the birth commit, so all lines of the same cohort
// share it.
func (t *retTracker) finalize() (map[string]retentionResult, retentionGlobals) {
	res := make(map[string]retentionResult)
	if t.cur < 0 {
		return res, retentionGlobals{}
	}
	for _, arr := range t.files {
		for _, b := range arr {
			bi := int(b)
			if bi >= 0 && bi <= t.cur {
				t.alive[bi]++
			}
		}
	}
	head := t.cur
	headDay := t.dayNum[head]
	for b := 0; b <= head; b++ {
		if t.added[b] == 0 && t.alive[b] == 0 {
			continue
		}
		maxC := head - b
		maxD := headDay - t.dayNum[b]
		var rc, rd float64
		if maxC > 0 {
			rc = float64(t.died[b]) / float64(maxC)
		}
		if maxD > 0 {
			rd = float64(t.diedDay[b]) / float64(maxD)
		}
		rc += float64(t.alive[b])
		rd += float64(t.alive[b])
		e := t.email[b]
		r := res[e]
		r.commits += rc
		r.days += rd
		r.added += int(t.added[b])
		r.alive += int(t.alive[b])
		res[e] = r
	}

	// Collaboration breadth: distinct files and directories per contributor.
	for e, set := range t.touched {
		dirs := make(map[string]struct{}, len(set))
		for f := range set {
			dirs[dirOf(f)] = struct{}{}
		}
		r := res[e]
		r.files = len(set)
		r.dirs = len(dirs)
		res[e] = r
	}

	glob := retentionGlobals{
		medianLifetimeDays:   medianFromHist(t.lifeHist, lineLifetimeBuckets, t.lifeCount),
		medianOnboardingDays: t.onboardingMedian(),
	}
	return res, glob
}

// onboardingMedian returns the median number of days between a contributor's
// first commit and the first line they authored that still survives in HEAD.
// It relies on t.alive being populated (call after the survival scan in
// finalize).
func (t *retTracker) onboardingMedian() int {
	firstCommitDay := make(map[string]int64)
	for i := 0; i <= t.cur; i++ {
		e := t.email[i]
		if d, ok := firstCommitDay[e]; !ok || t.dayNum[i] < d {
			firstCommitDay[e] = t.dayNum[i]
		}
	}
	firstAliveDay := make(map[string]int64)
	for b := 0; b <= t.cur; b++ {
		if t.alive[b] <= 0 {
			continue
		}
		e := t.email[b]
		if d, ok := firstAliveDay[e]; !ok || t.dayNum[b] < d {
			firstAliveDay[e] = t.dayNum[b]
		}
	}
	deltas := make([]int, 0, len(firstAliveDay))
	for e, ad := range firstAliveDay {
		delta := int(ad - firstCommitDay[e])
		if delta < 0 {
			delta = 0
		}
		deltas = append(deltas, delta)
	}
	if len(deltas) == 0 {
		return 0
	}
	sort.Ints(deltas)
	n := len(deltas)
	if n%2 == 1 {
		return deltas[n/2]
	}
	return (deltas[n/2-1] + deltas[n/2]) / 2
}

// dirOf returns the directory portion of a slash-separated git path, or "." for
// a top-level file.
func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return "."
}

// insertInt32 inserts n copies of val into s at index i.
func insertInt32(s []int32, i, val, n int) []int32 {
	if i > len(s) {
		i = len(s)
	}
	s = append(s, make([]int32, n)...)
	copy(s[i+n:], s[i:])
	for k := 0; k < n; k++ {
		s[i+k] = int32(val)
	}
	return s
}

// cleanDiffPath normalizes a path taken from a "---"/"+++" diff header,
// stripping the a//b/ prefix and returning "" for /dev/null.
func cleanDiffPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "/dev/null" {
		return ""
	}
	if strings.HasPrefix(p, "a/") || strings.HasPrefix(p, "b/") {
		p = p[2:]
	}
	return unquotePath(p)
}

// cleanRenamePath normalizes a path taken from a "rename from/to" header (which
// carries no a//b/ prefix).
func cleanRenamePath(p string) string {
	return unquotePath(strings.TrimSpace(p))
}

// unquotePath decodes a git-quoted path (used when it contains unusual bytes).
func unquotePath(p string) string {
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		if uq, err := strconv.Unquote(p); err == nil {
			return uq
		}
	}
	return p
}
