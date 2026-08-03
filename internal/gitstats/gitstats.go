// Package gitstats extracts per-contributor statistics from a git repository.
//
// The package is organized into two layers connected by a channel-based
// pipeline:
//
//   - The collection layer (collector.go) runs git and parses its output into
//     a stream of Commit values.
//   - The aggregation layer (aggregator.go) consumes that stream and folds it
//     into a Summary.
//
// gitstats.go wires the two together and defines the shared types. Because the
// layers communicate through a Commit channel, commits are aggregated as they
// arrive instead of being buffered, keeping the memory footprint bounded by the
// number of contributors and weeks rather than the number of commits.
package gitstats

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Contributor holds the aggregated statistics for a single author.
type Contributor struct {
	Name      string
	Email     string
	Commits   int
	Additions int
	Deletions int
	FirstSeen time.Time
	LastSeen  time.Time
	// Weekly holds the number of commits per week bucket across the whole
	// history window, oldest first. Used to render an activity sparkline.
	Weekly []float64
	// WeeklyAdd and WeeklyDel hold the number of lines added/deleted per
	// week bucket, aligned with Weekly. Used for the "lines" activity graph.
	WeeklyAdd []float64
	WeeklyDel []float64
	// RetentionCommits and RetentionDays hold the contributor's retention
	// score: the number of lines they added, weighted by how long each line
	// survived. Lines still present at HEAD count with the maximum weight of
	// 1. RetentionCommits measures lifespan in commits; RetentionDays measures
	// it in days (a line removed on the day it was added counts as zero).
	// RetentionAdded is the number of tracked added lines, i.e. the maximum
	// either score could reach, used as the denominator for a retention rate.
	// RetentionAlive is the number of those added lines still present in the
	// current HEAD.
	RetentionCommits float64
	RetentionDays    float64
	RetentionAdded   int
	RetentionAlive   int
	// FilesTouched and DirsTouched measure a contributor's collaboration
	// breadth: the number of distinct files and directories in which they added
	// or removed lines (specialist vs. generalist). Derived from the retention
	// pass, so they are zero when that pass fails.
	FilesTouched int
	DirsTouched  int
}

// Summary aggregates repository-wide totals.
type Summary struct {
	Repo         string
	TotalCommits int
	Additions    int
	Deletions    int
	Contributors []Contributor
	FirstCommit  time.Time
	LastCommit   time.Time
	WeekCount    int
	// Daily maps a calendar day (local midnight) to the number of commits
	// authored on that day across the whole repository. Powers the calendar
	// heatmap view.
	Daily map[time.Time]int
	// Punch holds commit counts bucketed by [weekday][hour], where weekday
	// uses time.Weekday indexing (0=Sunday..6=Saturday) and hour is 0..23.
	// Powers the hour-of-week punch-card view.
	Punch [7][24]int
	// WeeklyAdd and WeeklyDel hold repository-wide lines added / deleted per
	// week bucket, oldest first and aligned with WeekCount. They power the
	// "lines changed over time" graph on the activity view.
	WeeklyAdd []float64
	WeeklyDel []float64
	// CommitSizeHist is a histogram of lines changed (additions + deletions)
	// per commit, bucketed by CommitSizeBuckets. CommitSizeHist[i] counts the
	// commits whose size falls in bucket i; there is one extra overflow bucket
	// at the end for commits larger than the last bound.
	CommitSizeHist []int
	// LargeCommits counts commits that changed more than LargeCommitThreshold
	// lines.
	LargeCommits int
	// MedianCommitSize is the median lines changed per commit, interpolated
	// from CommitSizeHist so it can be derived without buffering every commit.
	MedianCommitSize int
	// MedianLineLifetimeDays is the "time-to-legacy": the median age, in days,
	// at which a line that was later overwritten died. Derived from the
	// retention pass; zero when it fails or when no lines have been overwritten.
	MedianLineLifetimeDays int
	// MedianOnboardingDays is the median time, in days, from a contributor's
	// first commit to their first line that still survives in HEAD. Derived
	// from the retention pass; zero when it fails.
	MedianOnboardingDays int
}

// LargeCommitThreshold is the lines-changed cutoff (additions + deletions)
// above which a commit is considered "large" in the commit-size distribution.
const LargeCommitThreshold = 500

// CommitSizeBuckets holds the inclusive upper bounds (in lines changed) of the
// commit-size histogram buckets. A commit larger than the final bound falls
// into an overflow bucket, so a histogram has len(CommitSizeBuckets)+1 buckets.
var CommitSizeBuckets = []int{0, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000}

// commitSizeBucket returns the histogram bucket index for a commit that changed
// size lines. The returned index is len(CommitSizeBuckets) for the overflow
// bucket.
func commitSizeBucket(size int) int {
	for i, ub := range CommitSizeBuckets {
		if size <= ub {
			return i
		}
	}
	return len(CommitSizeBuckets)
}

// commitSizeBucketRange returns the [lo, hi] lines-changed range covered by
// histogram bucket i, used to interpolate the median.
func commitSizeBucketRange(i int) (lo, hi int) {
	n := len(CommitSizeBuckets)
	switch {
	case i <= 0:
		return 0, 0
	case i < n:
		return CommitSizeBuckets[i-1] + 1, CommitSizeBuckets[i]
	default:
		last := CommitSizeBuckets[n-1]
		return last + 1, last + 1
	}
}

// medianCommitSize estimates the median lines changed per commit from a
// histogram of total commits, interpolating within the bucket that contains the
// middle commit.
func medianCommitSize(hist []int, total int) int {
	if total == 0 || len(hist) == 0 {
		return 0
	}
	target := total / 2 // 0-based rank of the median commit
	cum := 0
	for i, cnt := range hist {
		if cnt == 0 {
			continue
		}
		if cum+cnt > target {
			lo, hi := commitSizeBucketRange(i)
			if hi <= lo {
				return lo
			}
			frac := (float64(target-cum) + 0.5) / float64(cnt)
			return lo + int(frac*float64(hi-lo)+0.5)
		}
		cum += cnt
	}
	return 0
}

// lineLifetimeBuckets holds the inclusive upper bounds (in days) of the
// line-lifetime histogram buckets used for the "time-to-legacy" metric. A line
// that lived longer than the final bound falls into an overflow bucket, so a
// histogram has len(lineLifetimeBuckets)+1 buckets.
var lineLifetimeBuckets = []int{0, 1, 7, 30, 90, 180, 365, 730, 1825}

// histBucket returns the histogram bucket index for value v given the inclusive
// upper bounds in edges. The returned index is len(edges) for the overflow
// bucket.
func histBucket(v int, edges []int) int {
	for i, ub := range edges {
		if v <= ub {
			return i
		}
	}
	return len(edges)
}

// histBucketRange returns the [lo, hi] range covered by bucket i of a histogram
// whose bucket edges are the inclusive upper bounds in edges.
func histBucketRange(i int, edges []int) (lo, hi int) {
	n := len(edges)
	switch {
	case i <= 0:
		return 0, edges[0]
	case i < n:
		return edges[i-1] + 1, edges[i]
	default:
		last := edges[n-1]
		return last + 1, last + 1
	}
}

// medianFromHist estimates the median of a bucketed distribution, interpolating
// within the bucket that contains the middle observation. edges holds the
// inclusive upper bounds of the buckets (hist has len(edges)+1 entries).
func medianFromHist(hist, edges []int, total int) int {
	if total == 0 || len(hist) == 0 {
		return 0
	}
	target := total / 2
	cum := 0
	for i, cnt := range hist {
		if cnt == 0 {
			continue
		}
		if cum+cnt > target {
			lo, hi := histBucketRange(i, edges)
			if hi <= lo {
				return lo
			}
			frac := (float64(target-cum) + 0.5) / float64(cnt)
			return lo + int(frac*float64(hi-lo)+0.5)
		}
		cum += cnt
	}
	return 0
}

// DayKey normalizes a timestamp to local midnight for use as a Daily key.
func DayKey(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// isDocPath reports whether a repository path refers to documentation: any
// Markdown file (*.md) or any file under a docs/ directory. It is used to honor
// the --exclude-docs option, which drops documentation changes from all stats.
func isDocPath(p string) bool {
	lp := strings.ToLower(strings.TrimSpace(p))
	if lp == "" {
		return false
	}
	if strings.HasSuffix(lp, ".md") {
		return true
	}
	return strings.HasPrefix(lp, "docs/") || strings.Contains(lp, "/docs/")
}

// NormalizeIgnore lowercases, trims, de-duplicates and sorts an ignore list so
// that equivalent lists produce the same value (used for cache keying).
func NormalizeIgnore(list []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(list))
	for _, v := range list {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// ignoreSet builds a lookup set from an ignore list of names/emails.
func ignoreSet(list []string) map[string]bool {
	norm := NormalizeIgnore(list)
	s := make(map[string]bool, len(norm))
	for _, v := range norm {
		s[v] = true
	}
	return s
}

// Commit is a single parsed commit produced by the collection layer and
// consumed by the aggregation layer. It carries only the fields needed to build
// a Summary; the additions and deletions are already summed across the commit's
// files.
type Commit struct {
	Name      string
	Email     string
	When      time.Time
	Additions int
	Deletions int
}

// Collect runs git in repoPath and returns aggregated contributor stats.
// Commits authored by anyone listed in ignore (matched case-insensitively
// against author name or email) are excluded from all statistics. When
// excludeDocs is true, changes to Markdown files and files under docs/ folders
// are ignored.
func Collect(repoPath string, ignore []string, excludeDocs bool) (*Summary, error) {
	return CollectWithProgress(repoPath, ignore, excludeDocs, nil, Window{}, nil)
}

// CollectWithProgress is like Collect but reports scanning progress via the
// optional callback, invoked with the number of commits processed so far and
// the total number of commits expected. Either argument may be zero when the
// total is unknown. identities, when non-empty, aggregates the commits of
// several git author identities (matching emails/usernames) into a single
// contributor labeled by the identity's display name.
//
// Collection and aggregation run concurrently: a goroutine streams parsed
// commits over a channel while the caller's goroutine aggregates them, so the
// whole commit history is never held in memory at once.
func CollectWithProgress(repoPath string, ignore []string, excludeDocs bool, identities []Identity, window Window, progress func(done, total int)) (*Summary, error) {
	top, err := repoRoot(repoPath)
	if err != nil {
		return nil, err
	}

	// Compute the git window flags once from the HEAD commit date so that both
	// git walks use the same cutoff.
	wArgs := gitWindowArgs(window, headCommitTime(top))

	total := countCommits(top, wArgs) // best-effort; 0 if it fails

	// Two independent full-history git walks feed the summary: the numstat
	// pass streamed into aggregate below, and the patch pass that
	// reconstructs line lifetimes for retention. They share no state until
	// the final merge, so the retention walk runs in its own goroutine to
	// overlap with the numstat walk instead of running strictly after it.
	// On multi-core machines this roughly halves the collection time.
	// Only the numstat pass reports progress; the retention pass runs
	// silently so the two do not fight over the progress counter.
	type retResult struct {
		byEmail map[string]retentionResult
		globals retentionGlobals
		err     error
	}
	retc := make(chan retResult, 1)
	go func() {
		byEmail, glob, rerr := computeRetention(top, total, excludeDocs, wArgs, nil)
		retc <- retResult{byEmail, glob, rerr}
	}()

	commits := make(chan Commit, 256)
	errc := make(chan error, 1)
	go func() { errc <- streamCommits(top, commits, excludeDocs, wArgs) }()

	resolver, err := newIdentityResolver(identities)
	if err != nil {
		return nil, err
	}
	summary, emailToDisplay := aggregate(top, commits, total, ignoreSet(ignore), resolver, progress)

	// streamCommits closes the channel before returning, so aggregate has
	// finished draining by the time we read the error.
	if err := <-errc; err != nil {
		return nil, err
	}
	if summary.TotalCommits == 0 {
		return nil, fmt.Errorf("no commits found in %s", top)
	}

	// Merge in the retention pass. It is supplementary, so a failure there
	// leaves the retention fields at zero rather than failing the whole
	// collection. The buffered channel lets its goroutine finish even on the
	// early-return paths above without leaking.
	if out := <-retc; out.err == nil {
		// The retention pass is keyed by raw author email; fold those results
		// onto the same identities used for aggregation (via each identity's
		// display email) so a contributor merged from several emails gets their
		// combined retention.
		byDisplay := make(map[string]retentionResult, len(out.byEmail))
		for email, r := range out.byEmail {
			disp := emailToDisplay[email]
			if disp == "" {
				disp = email
			}
			agg := byDisplay[disp]
			agg.commits += r.commits
			agg.days += r.days
			agg.added += r.added
			agg.alive += r.alive
			agg.files += r.files
			agg.dirs += r.dirs
			byDisplay[disp] = agg
		}
		for i := range summary.Contributors {
			if r, ok := byDisplay[summary.Contributors[i].Email]; ok {
				summary.Contributors[i].RetentionCommits = r.commits
				summary.Contributors[i].RetentionDays = r.days
				summary.Contributors[i].RetentionAdded = r.added
				summary.Contributors[i].RetentionAlive = r.alive
				summary.Contributors[i].FilesTouched = r.files
				summary.Contributors[i].DirsTouched = r.dirs
			}
		}
		summary.MedianLineLifetimeDays = out.globals.medianLifetimeDays
		summary.MedianOnboardingDays = out.globals.medianOnboardingDays
	}
	return summary, nil
}

// SortKey selects how contributors are ordered.
type SortKey int

const (
	// SortCommits orders by number of commits (descending).
	SortCommits SortKey = iota
	// SortChanges orders by total lines changed (additions + deletions).
	SortChanges
	// SortImpact orders by a balanced score that combines commit count and
	// lines changed, normalized so neither dimension dominates. It is the
	// default order.
	SortImpact
	// SortRecent orders by most recent commit date (descending).
	SortRecent
	// SortRetention orders by retention score (descending): lines added
	// weighted by how long they survived, measured in commits.
	SortRetention
	// SortAlive orders by the number of added lines still present in the
	// current HEAD (descending).
	SortAlive
	// SortActive orders by active span (descending): the number of days
	// between a contributor's first and last commit.
	SortActive
)

// impactScore returns a balanced, magnitude-normalized ranking score for a
// contributor. Each dimension (commits and lines changed) is scaled to the
// busiest contributor in the set, then combined via a geometric mean so that a
// contributor must score well on both to rank highly. This avoids the bias of
// ranking purely by commit count (which favors many tiny commits) or purely by
// lines changed (which favors large or generated-file changes).
func impactScore(c Contributor, maxCommits, maxChanges int) float64 {
	var nc, nl float64
	if maxCommits > 0 {
		nc = float64(c.Commits) / float64(maxCommits)
	}
	if maxChanges > 0 {
		nl = float64(c.Additions+c.Deletions) / float64(maxChanges)
	}
	return math.Sqrt(nc * nl)
}

// SortBy sorts contributors in place according to key.
func SortBy(cs []Contributor, key SortKey) {
	var maxCommits, maxChanges int
	if key == SortImpact {
		for _, c := range cs {
			if c.Commits > maxCommits {
				maxCommits = c.Commits
			}
			if ch := c.Additions + c.Deletions; ch > maxChanges {
				maxChanges = ch
			}
		}
	}
	sort.SliceStable(cs, func(i, j int) bool {
		switch key {
		case SortChanges:
			ci := cs[i].Additions + cs[i].Deletions
			cj := cs[j].Additions + cs[j].Deletions
			if ci != cj {
				return ci > cj
			}
			return cs[i].Commits > cs[j].Commits
		case SortImpact:
			si := impactScore(cs[i], maxCommits, maxChanges)
			sj := impactScore(cs[j], maxCommits, maxChanges)
			if si != sj {
				return si > sj
			}
			if cs[i].Commits != cs[j].Commits {
				return cs[i].Commits > cs[j].Commits
			}
			return cs[i].Additions+cs[i].Deletions > cs[j].Additions+cs[j].Deletions
		case SortRecent:
			if !cs[i].LastSeen.Equal(cs[j].LastSeen) {
				return cs[i].LastSeen.After(cs[j].LastSeen)
			}
			return cs[i].Commits > cs[j].Commits
		case SortRetention:
			if cs[i].RetentionCommits != cs[j].RetentionCommits {
				return cs[i].RetentionCommits > cs[j].RetentionCommits
			}
			if cs[i].RetentionDays != cs[j].RetentionDays {
				return cs[i].RetentionDays > cs[j].RetentionDays
			}
			return cs[i].Additions > cs[j].Additions
		case SortAlive:
			if cs[i].RetentionAlive != cs[j].RetentionAlive {
				return cs[i].RetentionAlive > cs[j].RetentionAlive
			}
			return cs[i].RetentionCommits > cs[j].RetentionCommits
		case SortActive:
			ai := cs[i].LastSeen.Sub(cs[i].FirstSeen)
			aj := cs[j].LastSeen.Sub(cs[j].FirstSeen)
			if ai != aj {
				return ai > aj
			}
			return cs[i].Commits > cs[j].Commits
		default:
			if cs[i].Commits != cs[j].Commits {
				return cs[i].Commits > cs[j].Commits
			}
			return cs[i].Additions+cs[i].Deletions > cs[j].Additions+cs[j].Deletions
		}
	})
}
