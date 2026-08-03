package gitstats

// multi.go adds multi-repository support: it collects several repositories
// concurrently (cloning remote specs as needed) and merges their per-repo
// summaries into a single global summary. Each repository is scanned in its own
// goroutine, bounded by a small worker pool, so a batch of repositories is
// analyzed roughly in parallel rather than one after another.

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// RepoResult is the outcome of collecting a single repository. Summary is nil
// when Err is non-nil (the repository was skipped); Cached reports whether the
// summary came from the on-disk cache.
type RepoResult struct {
	Spec    RepoSpec
	Summary *Summary
	Cached  bool
	Err     error
}

// maxParallel bounds how many repositories are scanned at once. Each scan itself
// runs two concurrent git walks, so the pool is kept modest to avoid
// oversubscribing the machine.
func maxParallel() int {
	n := runtime.NumCPU() / 2
	if n < 2 {
		n = 2
	}
	if n > 8 {
		n = 8
	}
	return n
}

// CollectMulti resolves, (optionally) clones and scans every spec concurrently,
// then merges the successful summaries into a global one. It returns the
// per-repo summaries (in spec order, successful only), the merged global
// summary, and a per-spec result slice carrying any errors so callers can
// report the repositories that were skipped.
//
// progress, when non-nil, is called with the combined number of commits
// processed and expected across all repositories. identities, when non-empty,
// aggregates several git author identities into a single contributor across
// every repository.
func CollectMulti(specs []RepoSpec, useCache bool, ignore []string, excludeDocs bool, identities []Identity, window Window, progress func(done, total int)) ([]*Summary, *Summary, []RepoResult) {
	results := make([]RepoResult, len(specs))

	var mu sync.Mutex
	dones := make([]int, len(specs))
	totals := make([]int, len(specs))
	report := func() {
		if progress == nil {
			return
		}
		mu.Lock()
		d, t := 0, 0
		for i := range dones {
			d += dones[i]
			t += totals[i]
		}
		mu.Unlock()
		progress(d, t)
	}

	sem := make(chan struct{}, maxParallel())
	var wg sync.WaitGroup
	for i := range specs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			spec := specs[i]
			results[i].Spec = spec

			path := spec.LocalPath
			if spec.IsClone {
				p, err := CloneOrUpdate(spec)
				if err != nil {
					results[i].Err = err
					return
				}
				path = p
				results[i].Spec.LocalPath = p
			}
			if path == "" {
				results[i].Err = fmt.Errorf("no local path for %q", spec.Raw)
				return
			}

			sum, cached, err := CollectCached(path, useCache, ignore, excludeDocs, identities, window, func(done, total int) {
				mu.Lock()
				dones[i] = done
				totals[i] = total
				mu.Unlock()
				report()
			})
			if err != nil {
				results[i].Err = err
				return
			}
			// Prefer the configured display name over the on-disk path so the
			// UI shows a stable, friendly label.
			if spec.DisplayName != "" {
				sum.Repo = spec.DisplayName
			}
			results[i].Summary = sum
			results[i].Cached = cached
		}(i)
	}
	wg.Wait()

	repos := make([]*Summary, 0, len(results))
	for i := range results {
		if results[i].Summary != nil {
			repos = append(repos, results[i].Summary)
		}
	}
	global := MergeSummaries(repos, "All repositories")
	return repos, global, results
}

// weekBase returns the absolute (epoch-relative) week index of a summary's
// first commit, which is the week that bucket 0 of its Weekly slices maps to.
func weekBase(first time.Time) int64 {
	if first.IsZero() {
		return 0
	}
	return first.Unix() / weekSpanSecs
}

// MergeSummaries folds several per-repo summaries into a single aggregate.
// Totals, the punch-card matrix, the daily map and the commit-size histogram
// are summed; contributors are merged by email; and the weekly churn series are
// realigned to absolute calendar weeks (using each summary's FirstCommit) so the
// combined series spans the full history of all repositories.
//
// The median line-lifetime and onboarding metrics cannot be merged exactly
// because the underlying histograms are not retained on the Summary; they are
// approximated with a commit-weighted average of the per-repo medians.
func MergeSummaries(sums []*Summary, name string) *Summary {
	valid := make([]*Summary, 0, len(sums))
	for _, s := range sums {
		if s != nil && s.TotalCommits > 0 {
			valid = append(valid, s)
		}
	}
	g := &Summary{Repo: name, Daily: make(map[time.Time]int)}
	if len(valid) == 0 {
		return g
	}

	// Global week window across all repositories.
	var minWeek, maxWeek int64
	haveWeek := false
	for _, s := range valid {
		base := weekBase(s.FirstCommit)
		end := base
		if n := len(s.WeeklyAdd); n > 0 {
			end = base + int64(n) - 1
		}
		if !haveWeek {
			minWeek, maxWeek, haveWeek = base, end, true
			continue
		}
		if base < minWeek {
			minWeek = base
		}
		if end > maxWeek {
			maxWeek = end
		}
	}
	weeks := int(maxWeek-minWeek) + 1
	if weeks < 1 {
		weeks = 1
	}
	g.WeekCount = weeks
	g.WeeklyAdd = make([]float64, weeks)
	g.WeeklyDel = make([]float64, weeks)
	g.CommitSizeHist = make([]int, len(CommitSizeBuckets)+1)

	byEmail := make(map[string]*Contributor)
	var order []*Contributor

	var lifeWeighted, onboardWeighted float64
	var lifeWeight, onboardWeight int

	addWeekly := func(dst []float64, src []float64, off int) {
		for i, v := range src {
			j := off + i
			if j >= 0 && j < len(dst) {
				dst[j] += v
			}
		}
	}

	for _, s := range valid {
		g.TotalCommits += s.TotalCommits
		g.Additions += s.Additions
		g.Deletions += s.Deletions
		g.LargeCommits += s.LargeCommits
		if g.FirstCommit.IsZero() || (!s.FirstCommit.IsZero() && s.FirstCommit.Before(g.FirstCommit)) {
			g.FirstCommit = s.FirstCommit
		}
		if s.LastCommit.After(g.LastCommit) {
			g.LastCommit = s.LastCommit
		}
		for d, n := range s.Daily {
			g.Daily[d] += n
		}
		for d := 0; d < 7; d++ {
			for h := 0; h < 24; h++ {
				g.Punch[d][h] += s.Punch[d][h]
			}
		}
		for i := 0; i < len(s.CommitSizeHist) && i < len(g.CommitSizeHist); i++ {
			g.CommitSizeHist[i] += s.CommitSizeHist[i]
		}

		off := int(weekBase(s.FirstCommit) - minWeek)
		addWeekly(g.WeeklyAdd, s.WeeklyAdd, off)
		addWeekly(g.WeeklyDel, s.WeeklyDel, off)

		if s.MedianLineLifetimeDays > 0 {
			lifeWeighted += float64(s.MedianLineLifetimeDays) * float64(s.TotalCommits)
			lifeWeight += s.TotalCommits
		}
		if s.MedianOnboardingDays > 0 {
			onboardWeighted += float64(s.MedianOnboardingDays) * float64(s.TotalCommits)
			onboardWeight += s.TotalCommits
		}

		for i := range s.Contributors {
			c := &s.Contributors[i]
			gc := byEmail[c.Email]
			if gc == nil {
				nc := Contributor{
					Name:      c.Name,
					Email:     c.Email,
					FirstSeen: c.FirstSeen,
					LastSeen:  c.LastSeen,
					Weekly:    make([]float64, weeks),
					WeeklyAdd: make([]float64, weeks),
					WeeklyDel: make([]float64, weeks),
				}
				byEmail[c.Email] = &nc
				order = append(order, &nc)
				gc = &nc
			}
			gc.Commits += c.Commits
			gc.Additions += c.Additions
			gc.Deletions += c.Deletions
			if !c.FirstSeen.IsZero() && (gc.FirstSeen.IsZero() || c.FirstSeen.Before(gc.FirstSeen)) {
				gc.FirstSeen = c.FirstSeen
			}
			if c.LastSeen.After(gc.LastSeen) {
				gc.LastSeen = c.LastSeen
			}
			gc.RetentionCommits += c.RetentionCommits
			gc.RetentionDays += c.RetentionDays
			gc.RetentionAdded += c.RetentionAdded
			gc.RetentionAlive += c.RetentionAlive
			gc.FilesTouched += c.FilesTouched
			gc.DirsTouched += c.DirsTouched
			addWeekly(gc.Weekly, c.Weekly, off)
			addWeekly(gc.WeeklyAdd, c.WeeklyAdd, off)
			addWeekly(gc.WeeklyDel, c.WeeklyDel, off)
		}
	}

	g.Contributors = make([]Contributor, len(order))
	for i, c := range order {
		g.Contributors[i] = *c
	}
	SortBy(g.Contributors, SortImpact)

	g.MedianCommitSize = medianCommitSize(g.CommitSizeHist, g.TotalCommits)
	if lifeWeight > 0 {
		g.MedianLineLifetimeDays = int(lifeWeighted/float64(lifeWeight) + 0.5)
	}
	if onboardWeight > 0 {
		g.MedianOnboardingDays = int(onboardWeighted/float64(onboardWeight) + 0.5)
	}
	return g
}
