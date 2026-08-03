package gitstats

// aggregator.go is the aggregation layer: it folds a stream of Commit values
// into a Summary. It is pure computation and has no knowledge of git, which
// makes it easy to unit test and to feed from alternative sources (for example
// a cache).

import (
	"strings"
	"time"

	"github.com/samber/lo"
)

// weekSpanSecs is the number of seconds in a week, used to bucket commits into
// absolute (epoch-relative) week indices while streaming.
const weekSpanSecs = int64(7 * 24 * 3600)

// contribAccum accumulates a single contributor's statistics while commits are
// streamed. Weekly activity is kept in maps keyed by absolute week index so we
// never need to know the repository's time window up front; the maps are
// converted to dense slices once streaming completes. This keeps memory usage
// proportional to (contributors × active weeks) rather than to the number of
// commits.
type contribAccum struct {
	name       string
	email      string
	commits    int
	additions  int
	deletions  int
	firstSeen  time.Time
	lastSeen   time.Time
	weekCommit map[int64]float64
	weekAdd    map[int64]float64
	weekDel    map[int64]float64
}

// aggregate consumes commits from in until the channel is closed and returns
// the resulting Summary along with a map from each raw author email to the
// display email of the identity it was folded into (used to align the retention
// pass with aggregated identities). Commits whose author name or (lowercased)
// email is in the ignore set are skipped entirely, so they affect no totals,
// graphs, or heatmaps. When resolver is non-nil, commits matching a configured
// identity are aggregated under that identity's display name instead of
// per-email. progress, when non-nil, is called periodically with the number of
// commits processed and the expected total.
func aggregate(repo string, in <-chan Commit, total int, ignore map[string]bool, resolver *identityResolver, progress func(done, total int)) (*Summary, map[string]string) {
	sum := &Summary{Repo: repo, Daily: make(map[time.Time]int)}
	byKey := make(map[string]*contribAccum)
	emailToDisplay := make(map[string]string)
	var order []*contribAccum

	// Repository-wide weekly line churn and commit-size histogram, accumulated
	// while streaming so no per-commit buffer is retained.
	sumWeekAdd := make(map[int64]float64)
	sumWeekDel := make(map[int64]float64)
	hist := make([]int, len(CommitSizeBuckets)+1)
	largeCommits := 0

	var minWeek, maxWeek int64
	var haveWeek bool
	seen := 0

	for c := range in {
		seen++
		if progress != nil && seen%64 == 0 {
			progress(seen, total)
		}
		if len(ignore) > 0 &&
			(ignore[strings.ToLower(c.Name)] || ignore[c.Email]) {
			continue
		}

		sum.TotalCommits++
		sum.Additions += c.Additions
		sum.Deletions += c.Deletions
		if sum.FirstCommit.IsZero() || c.When.Before(sum.FirstCommit) {
			sum.FirstCommit = c.When
		}
		if c.When.After(sum.LastCommit) {
			sum.LastCommit = c.When
		}
		sum.Daily[DayKey(c.When)]++
		sum.Punch[int(c.When.Weekday())][c.When.Hour()]++

		size := c.Additions + c.Deletions
		hist[commitSizeBucket(size)]++
		if size > LargeCommitThreshold {
			largeCommits++
		}

		a := byKey[c.Email]
		if a == nil {
			// Map the commit onto its canonical identity so that several emails
			// or author names belonging to the same person fold into one
			// contributor. Unmatched authors keep their raw email as the key.
			canon, display, displayEmail, _ := resolver.resolve(c.Name, c.Email)
			emailToDisplay[c.Email] = displayEmail
			a = byKey[canon]
			if a == nil {
				a = &contribAccum{
					name:       display,
					email:      displayEmail,
					firstSeen:  c.When,
					lastSeen:   c.When,
					weekCommit: make(map[int64]float64),
					weekAdd:    make(map[int64]float64),
					weekDel:    make(map[int64]float64),
				}
				byKey[canon] = a
				order = append(order, a)
			}
			byKey[c.Email] = a
		}
		a.commits++
		a.additions += c.Additions
		a.deletions += c.Deletions
		if c.When.Before(a.firstSeen) {
			a.firstSeen = c.When
		}
		if c.When.After(a.lastSeen) {
			a.lastSeen = c.When
		}

		w := c.When.Unix() / weekSpanSecs
		a.weekCommit[w]++
		a.weekAdd[w] += float64(c.Additions)
		a.weekDel[w] += float64(c.Deletions)
		sumWeekAdd[w] += float64(c.Additions)
		sumWeekDel[w] += float64(c.Deletions)
		if !haveWeek {
			minWeek, maxWeek, haveWeek = w, w, true
		} else {
			if w < minWeek {
				minWeek = w
			}
			if w > maxWeek {
				maxWeek = w
			}
		}
	}
	if progress != nil {
		progress(seen, total)
	}

	if sum.TotalCommits == 0 {
		return sum, emailToDisplay
	}

	weeks := int(maxWeek-minWeek) + 1
	if weeks < 1 {
		weeks = 1
	}
	sum.WeekCount = weeks

	sum.Contributors = lo.Map(order, func(a *contribAccum, _ int) Contributor {
		c := Contributor{
			Name:      a.name,
			Email:     a.email,
			Commits:   a.commits,
			Additions: a.additions,
			Deletions: a.deletions,
			FirstSeen: a.firstSeen,
			LastSeen:  a.lastSeen,
			Weekly:    make([]float64, weeks),
			WeeklyAdd: make([]float64, weeks),
			WeeklyDel: make([]float64, weeks),
		}
		for w, v := range a.weekCommit {
			c.Weekly[int(w-minWeek)] = v
		}
		for w, v := range a.weekAdd {
			c.WeeklyAdd[int(w-minWeek)] = v
		}
		for w, v := range a.weekDel {
			c.WeeklyDel[int(w-minWeek)] = v
		}
		return c
	})
	SortBy(sum.Contributors, SortImpact)

	// Dense repository-wide weekly churn series aligned with WeekCount.
	sum.WeeklyAdd = make([]float64, weeks)
	sum.WeeklyDel = make([]float64, weeks)
	for w, v := range sumWeekAdd {
		sum.WeeklyAdd[int(w-minWeek)] = v
	}
	for w, v := range sumWeekDel {
		sum.WeeklyDel[int(w-minWeek)] = v
	}

	sum.CommitSizeHist = hist
	sum.LargeCommits = largeCommits
	sum.MedianCommitSize = medianCommitSize(hist, sum.TotalCommits)

	return sum, emailToDisplay
}
