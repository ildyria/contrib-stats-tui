package gitstats

import (
	"testing"
	"time"
)

func TestIsCloneURL(t *testing.T) {
	cases := map[string]bool{
		"https://github.com/x/y.git": true,
		"git@github.com:x/y.git":     true,
		"server:org/repo.git":        true,
		"../sibling":                 false,
		"./local":                    false,
		"/abs/path":                  false,
		"~/home/repo":                false,
		"plainname":                  false,
	}
	for in, want := range cases {
		if got := IsCloneURL(in); got != want {
			t.Errorf("IsCloneURL(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestResolveReposRelativePaths(t *testing.T) {
	specs := ResolveRepos([]string{"repoA", "../repoB", "https://host/x.git"}, "/cfg/dir")
	if len(specs) != 3 {
		t.Fatalf("got %d specs, want 3", len(specs))
	}
	if specs[0].LocalPath != "/cfg/dir/repoA" {
		t.Errorf("repoA LocalPath = %q", specs[0].LocalPath)
	}
	if specs[1].LocalPath != "/cfg/repoB" {
		t.Errorf("repoB LocalPath = %q", specs[1].LocalPath)
	}
	if !specs[2].IsClone || specs[2].DisplayName != "x" {
		t.Errorf("clone spec = %+v", specs[2])
	}
}

// mkSummary builds a minimal one-contributor summary starting at first with the
// given weekly commit counts, for exercising MergeSummaries.
func mkSummary(repo, email string, first time.Time, commits int, weekly []float64) *Summary {
	weeks := len(weekly)
	if weeks == 0 {
		weeks = 1
	}
	s := &Summary{
		Repo:         repo,
		TotalCommits: commits,
		Additions:    commits * 10,
		Deletions:    commits * 2,
		FirstCommit:  first,
		LastCommit:   first.Add(time.Duration(weeks) * 7 * 24 * time.Hour),
		WeekCount:    weeks,
		Daily:        map[time.Time]int{DayKey(first): commits},
		WeeklyAdd:    make([]float64, weeks),
		WeeklyDel:    make([]float64, weeks),
		Contributors: []Contributor{{
			Name:      repo + "-dev",
			Email:     email,
			Commits:   commits,
			Additions: commits * 10,
			Deletions: commits * 2,
			FirstSeen: first,
			LastSeen:  first,
			Weekly:    append([]float64(nil), weekly...),
			WeeklyAdd: make([]float64, weeks),
			WeeklyDel: make([]float64, weeks),
		}},
	}
	s.Punch[int(first.Weekday())][first.Hour()] = commits
	return s
}

func TestMergeSummaries(t *testing.T) {
	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC) // a Monday
	// Repo A: 2 weeks starting week 0.
	a := mkSummary("A", "alice@example.com", base, 5, []float64{3, 2})
	// Repo B starts 4 weeks later; shares alice + adds bob.
	bStart := base.Add(4 * 7 * 24 * time.Hour)
	b := mkSummary("B", "bob@example.com", bStart, 4, []float64{1, 3})
	b.Contributors = append(b.Contributors, Contributor{
		Name: "alice", Email: "alice@example.com", Commits: 2,
		FirstSeen: bStart, LastSeen: bStart,
		Weekly: []float64{2, 0}, WeeklyAdd: []float64{0, 0}, WeeklyDel: []float64{0, 0},
	})
	b.TotalCommits += 2

	g := MergeSummaries([]*Summary{a, b}, "All")
	if g.TotalCommits != 11 {
		t.Errorf("TotalCommits = %d, want 11", g.TotalCommits)
	}
	// Global window spans week 0 (A start) .. week 5 (B second week) = 6 weeks.
	if g.WeekCount != 6 {
		t.Errorf("WeekCount = %d, want 6", g.WeekCount)
	}
	// alice is merged into one contributor across both repos.
	var alice *Contributor
	for i := range g.Contributors {
		if g.Contributors[i].Email == "alice@example.com" {
			alice = &g.Contributors[i]
		}
	}
	if alice == nil {
		t.Fatal("alice missing from merged contributors")
	}
	if alice.Commits != 7 {
		t.Errorf("alice.Commits = %d, want 7 (5 from A + 2 from B)", alice.Commits)
	}
	if len(g.Contributors) != 2 {
		t.Errorf("merged contributors = %d, want 2", len(g.Contributors))
	}
	// alice's weekly series realigned: weeks 0,1 from A (3,2) and week 4 from B (2).
	if len(alice.Weekly) != 6 {
		t.Fatalf("alice.Weekly len = %d, want 6", len(alice.Weekly))
	}
	if alice.Weekly[0] != 3 || alice.Weekly[1] != 2 || alice.Weekly[4] != 2 {
		t.Errorf("alice.Weekly realignment wrong: %v", alice.Weekly)
	}
}
