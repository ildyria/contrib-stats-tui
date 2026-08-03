package gitstats

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// weekOf returns the absolute week index for t, matching the formula used
// inside aggregate (Unix seconds divided by one week).
func weekOf(t time.Time) int64 {
	return t.Unix() / weekSpanSecs
}

// ---------------------------------------------------------------------------
// Ignore-set exclusion
// ---------------------------------------------------------------------------

func TestAggregateIgnoresExcludedAuthors(t *testing.T) {
	when := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	resolver := mustResolver(t, nil)

	// alice is in the ignore list; only bob's commits should be counted.
	sum := feed(t, resolver,
		Commit{Name: "alice", Email: "alice@example.com", When: when, Additions: 50, Deletions: 10},
		Commit{Name: "bob", Email: "bob@example.com", When: when.Add(time.Hour), Additions: 5, Deletions: 2},
	)

	// feed() passes no ignore set, so both authors appear.
	// Re-run with an explicit ignore set via aggregate directly.
	ch := make(chan Commit, 2)
	ch <- Commit{Name: "alice", Email: "alice@example.com", When: when, Additions: 50, Deletions: 10}
	ch <- Commit{Name: "bob", Email: "bob@example.com", When: when.Add(time.Hour), Additions: 5, Deletions: 2}
	close(ch)

	ignore := map[string]bool{"alice": true}
	got, _ := aggregate("repo", ch, 2, ignore, resolver, nil)

	if got.TotalCommits != 1 {
		t.Errorf("TotalCommits = %d, want 1 (alice excluded)", got.TotalCommits)
	}
	if got.Additions != 5 || got.Deletions != 2 {
		t.Errorf("Additions/Deletions = %d/%d, want 5/2", got.Additions, got.Deletions)
	}
	if len(got.Contributors) != 1 || got.Contributors[0].Name != "bob" {
		t.Errorf("Contributors = %v, want [bob]", got.Contributors)
	}

	// Ignored commits must not appear in the daily heatmap.
	for d, n := range got.Daily {
		_ = d
		if n != 1 {
			t.Errorf("Daily bucket: want 1, got %d", n)
		}
	}

	// The variable sum is used above just to prove feed works separately; keep
	// the linter happy.
	_ = sum
}

func TestAggregateIgnoreByEmail(t *testing.T) {
	when := time.Date(2024, 5, 1, 9, 0, 0, 0, time.UTC)
	resolver := mustResolver(t, nil)

	ch := make(chan Commit, 2)
	ch <- Commit{Name: "bot", Email: "bot@noreply.example.com", When: when, Additions: 100}
	ch <- Commit{Name: "human", Email: "human@example.com", When: when, Additions: 3}
	close(ch)

	ignore := map[string]bool{"bot@noreply.example.com": true}
	got, _ := aggregate("repo", ch, 2, ignore, resolver, nil)

	if got.TotalCommits != 1 {
		t.Errorf("TotalCommits = %d, want 1", got.TotalCommits)
	}
	if got.Additions != 3 {
		t.Errorf("Additions = %d, want 3", got.Additions)
	}
}

// ---------------------------------------------------------------------------
// Commit-size histogram
// ---------------------------------------------------------------------------

func TestAggregateCommitSizeHistogram(t *testing.T) {
	when := time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC)
	resolver := mustResolver(t, nil)

	// Three commits with known sizes:
	//   size 0  → bucket 0
	//   size 5  → bucket 1  (1..5)
	//   size 6  → bucket 2  (6..10)
	commits := []Commit{
		{Name: "dev", Email: "dev@x.com", When: when, Additions: 0, Deletions: 0},
		{Name: "dev", Email: "dev@x.com", When: when.Add(time.Hour), Additions: 3, Deletions: 2},
		{Name: "dev", Email: "dev@x.com", When: when.Add(2 * time.Hour), Additions: 4, Deletions: 2},
	}

	ch := make(chan Commit, len(commits))
	for _, c := range commits {
		ch <- c
	}
	close(ch)
	sum, _ := aggregate("repo", ch, len(commits), nil, resolver, nil)

	if len(sum.CommitSizeHist) != len(CommitSizeBuckets)+1 {
		t.Fatalf("CommitSizeHist len = %d, want %d", len(sum.CommitSizeHist), len(CommitSizeBuckets)+1)
	}
	// bucket 0 (size==0): 1 commit
	if sum.CommitSizeHist[0] != 1 {
		t.Errorf("bucket 0: got %d, want 1", sum.CommitSizeHist[0])
	}
	// bucket 1 (size 1..5): 1 commit (size 5)
	if sum.CommitSizeHist[1] != 1 {
		t.Errorf("bucket 1: got %d, want 1", sum.CommitSizeHist[1])
	}
	// bucket 2 (size 6..10): 1 commit (size 6)
	if sum.CommitSizeHist[2] != 1 {
		t.Errorf("bucket 2: got %d, want 1", sum.CommitSizeHist[2])
	}
	if sum.TotalCommits != 3 {
		t.Errorf("TotalCommits = %d, want 3", sum.TotalCommits)
	}
}

func TestAggregateLargeCommit(t *testing.T) {
	when := time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC)
	resolver := mustResolver(t, nil)

	ch := make(chan Commit, 2)
	ch <- Commit{Name: "dev", Email: "dev@x.com", When: when, Additions: 300, Deletions: 10}                 // size=310, small
	ch <- Commit{Name: "dev", Email: "dev@x.com", When: when.Add(time.Hour), Additions: 400, Deletions: 200} // size=600 > LargeCommitThreshold(500)
	close(ch)

	sum, _ := aggregate("repo", ch, 2, nil, resolver, nil)
	if sum.LargeCommits != 1 {
		t.Errorf("LargeCommits = %d, want 1", sum.LargeCommits)
	}
}

// ---------------------------------------------------------------------------
// Punch card (weekday × hour heatmap)
// ---------------------------------------------------------------------------

func TestAggregatePunchCard(t *testing.T) {
	// Monday 09:00 UTC
	when := time.Date(2024, 1, 8, 9, 0, 0, 0, time.UTC)
	if when.Weekday() != time.Monday {
		t.Fatalf("test date is not Monday: %v", when.Weekday())
	}

	resolver := mustResolver(t, nil)
	ch := make(chan Commit, 3)
	ch <- Commit{Name: "a", Email: "a@x.com", When: when, Additions: 1}
	ch <- Commit{Name: "a", Email: "a@x.com", When: when, Additions: 1}
	// Friday 14:00 UTC
	fri := time.Date(2024, 1, 12, 14, 0, 0, 0, time.UTC)
	ch <- Commit{Name: "b", Email: "b@x.com", When: fri, Additions: 1}
	close(ch)

	sum, _ := aggregate("repo", ch, 3, nil, resolver, nil)

	monday := int(time.Monday)
	friday := int(time.Friday)
	if sum.Punch[monday][9] != 2 {
		t.Errorf("Punch[Mon][9] = %d, want 2", sum.Punch[monday][9])
	}
	if sum.Punch[friday][14] != 1 {
		t.Errorf("Punch[Fri][14] = %d, want 1", sum.Punch[friday][14])
	}
	// All other slots should be zero.
	for d := 0; d < 7; d++ {
		for h := 0; h < 24; h++ {
			if d == monday && h == 9 {
				continue
			}
			if d == friday && h == 14 {
				continue
			}
			if sum.Punch[d][h] != 0 {
				t.Errorf("Punch[%d][%d] = %d, want 0", d, h, sum.Punch[d][h])
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Daily heatmap
// ---------------------------------------------------------------------------

func TestAggregateDailyHeatmap(t *testing.T) {
	resolver := mustResolver(t, nil)

	day1 := time.Date(2024, 4, 1, 8, 0, 0, 0, time.UTC)
	day2 := time.Date(2024, 4, 2, 18, 0, 0, 0, time.UTC)

	ch := make(chan Commit, 4)
	ch <- Commit{Name: "a", Email: "a@x.com", When: day1}
	ch <- Commit{Name: "a", Email: "a@x.com", When: day1.Add(time.Hour)} // same day
	ch <- Commit{Name: "b", Email: "b@x.com", When: day2}
	ch <- Commit{Name: "b", Email: "b@x.com", When: day2.Add(2 * time.Hour)} // same day
	close(ch)

	sum, _ := aggregate("repo", ch, 4, nil, resolver, nil)

	k1 := DayKey(day1)
	k2 := DayKey(day2)
	if sum.Daily[k1] != 2 {
		t.Errorf("Daily[day1] = %d, want 2", sum.Daily[k1])
	}
	if sum.Daily[k2] != 2 {
		t.Errorf("Daily[day2] = %d, want 2", sum.Daily[k2])
	}
	if len(sum.Daily) != 2 {
		t.Errorf("Daily has %d entries, want 2", len(sum.Daily))
	}
}

// ---------------------------------------------------------------------------
// Weekly series alignment
// ---------------------------------------------------------------------------

func TestAggregateWeeklySeries(t *testing.T) {
	resolver := mustResolver(t, nil)

	// Two commits exactly one week apart.
	w0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	w1 := w0.Add(7 * 24 * time.Hour)

	ch := make(chan Commit, 2)
	ch <- Commit{Name: "dev", Email: "dev@x.com", When: w0, Additions: 10, Deletions: 2}
	ch <- Commit{Name: "dev", Email: "dev@x.com", When: w1, Additions: 5, Deletions: 1}
	close(ch)

	sum, _ := aggregate("repo", ch, 2, nil, resolver, nil)

	if sum.WeekCount != 2 {
		t.Fatalf("WeekCount = %d, want 2", sum.WeekCount)
	}
	if len(sum.Contributors) != 1 {
		t.Fatalf("Contributors = %d, want 1", len(sum.Contributors))
	}
	c := sum.Contributors[0]
	if len(c.Weekly) != 2 {
		t.Fatalf("c.Weekly len = %d, want 2", len(c.Weekly))
	}
	if c.Weekly[0] != 1 || c.Weekly[1] != 1 {
		t.Errorf("c.Weekly = %v, want [1 1]", c.Weekly)
	}

	// Repository-wide WeeklyAdd/WeeklyDel should also be aligned.
	if len(sum.WeeklyAdd) != 2 || len(sum.WeeklyDel) != 2 {
		t.Fatalf("WeeklyAdd/Del len = %d/%d, want 2", len(sum.WeeklyAdd), len(sum.WeeklyDel))
	}
	if sum.WeeklyAdd[0] != 10 || sum.WeeklyAdd[1] != 5 {
		t.Errorf("WeeklyAdd = %v, want [10 5]", sum.WeeklyAdd)
	}
	if sum.WeeklyDel[0] != 2 || sum.WeeklyDel[1] != 1 {
		t.Errorf("WeeklyDel = %v, want [2 1]", sum.WeeklyDel)
	}
}

// ---------------------------------------------------------------------------
// Empty repository
// ---------------------------------------------------------------------------

func TestAggregateEmpty(t *testing.T) {
	resolver := mustResolver(t, nil)
	ch := make(chan Commit)
	close(ch)
	sum, emailToDisplay := aggregate("empty", ch, 0, nil, resolver, nil)
	if sum.TotalCommits != 0 {
		t.Errorf("TotalCommits = %d, want 0", sum.TotalCommits)
	}
	if len(emailToDisplay) != 0 {
		t.Errorf("emailToDisplay len = %d, want 0", len(emailToDisplay))
	}
}

// ---------------------------------------------------------------------------
// Totals
// ---------------------------------------------------------------------------

func TestAggregateTotals(t *testing.T) {
	resolver := mustResolver(t, nil)
	base := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	ch := make(chan Commit, 3)
	ch <- Commit{Name: "a", Email: "a@x.com", When: base, Additions: 10, Deletions: 1}
	ch <- Commit{Name: "b", Email: "b@x.com", When: base.Add(time.Hour), Additions: 20, Deletions: 2}
	ch <- Commit{Name: "a", Email: "a@x.com", When: base.Add(2 * time.Hour), Additions: 30, Deletions: 3}
	close(ch)

	sum, _ := aggregate("repo", ch, 3, nil, resolver, nil)

	if sum.TotalCommits != 3 {
		t.Errorf("TotalCommits = %d, want 3", sum.TotalCommits)
	}
	if sum.Additions != 60 {
		t.Errorf("Additions = %d, want 60", sum.Additions)
	}
	if sum.Deletions != 6 {
		t.Errorf("Deletions = %d, want 6", sum.Deletions)
	}
	if !sum.FirstCommit.Equal(base) {
		t.Errorf("FirstCommit = %v, want %v", sum.FirstCommit, base)
	}
	if !sum.LastCommit.Equal(base.Add(2 * time.Hour)) {
		t.Errorf("LastCommit = %v, want %v", sum.LastCommit, base.Add(2*time.Hour))
	}
}
