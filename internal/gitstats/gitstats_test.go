package gitstats

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// commitSizeBucket
// ---------------------------------------------------------------------------

func TestCommitSizeBucket(t *testing.T) {
	// CommitSizeBuckets = {0,5,10,25,50,100,250,500,1000,2500,5000}
	// bucket index 0..10 are within bounds; index 11 is the overflow bucket.
	cases := []struct {
		size   int
		bucket int
	}{
		{0, 0},
		{1, 1},
		{5, 1},
		{6, 2},
		{10, 2},
		{11, 3},
		{25, 3},
		{26, 4},
		{50, 4},
		{51, 5},
		{100, 5},
		{101, 6},
		{250, 6},
		{251, 7},
		{500, 7},
		{501, 8},
		{1000, 8},
		{1001, 9},
		{2500, 9},
		{2501, 10},
		{5000, 10},
		{5001, 11}, // overflow
		{10000, 11},
	}
	for _, tc := range cases {
		got := commitSizeBucket(tc.size)
		if got != tc.bucket {
			t.Errorf("commitSizeBucket(%d) = %d, want %d", tc.size, got, tc.bucket)
		}
	}
}

// ---------------------------------------------------------------------------
// commitSizeBucketRange
// ---------------------------------------------------------------------------

func TestCommitSizeBucketRange(t *testing.T) {
	// CommitSizeBuckets = {0,5,10,25,50,100,250,500,1000,2500,5000}
	cases := []struct {
		i, wantLo, wantHi int
	}{
		{0, 0, 0},        // i<=0: (0,0)
		{1, 1, 5},        // (0+1, 5)
		{2, 6, 10},       // (5+1, 10)
		{3, 11, 25},      // (10+1, 25)
		{10, 2501, 5000}, // last in-bounds bucket
		{11, 5001, 5001}, // overflow: (5000+1, 5000+1)
		{99, 5001, 5001}, // large overflow index also returns overflow range
	}
	for _, tc := range cases {
		lo, hi := commitSizeBucketRange(tc.i)
		if lo != tc.wantLo || hi != tc.wantHi {
			t.Errorf("commitSizeBucketRange(%d) = [%d, %d], want [%d, %d]",
				tc.i, lo, hi, tc.wantLo, tc.wantHi)
		}
	}
}

// ---------------------------------------------------------------------------
// medianCommitSize
// ---------------------------------------------------------------------------

func TestMedianCommitSize(t *testing.T) {
	// Edge cases
	if got := medianCommitSize(nil, 0); got != 0 {
		t.Errorf("nil hist: got %d, want 0", got)
	}
	if got := medianCommitSize([]int{}, 5); got != 0 {
		t.Errorf("empty hist: got %d, want 0", got)
	}
	if got := medianCommitSize([]int{5}, 0); got != 0 {
		t.Errorf("total=0: got %d, want 0", got)
	}

	nBuckets := len(CommitSizeBuckets) + 1 // 12 buckets

	// All commits in bucket 0 (size==0) → median = 0
	hist := make([]int, nBuckets)
	hist[0] = 10
	if got := medianCommitSize(hist, 10); got != 0 {
		t.Errorf("all size-0: got %d, want 0", got)
	}

	// All commits in bucket 1 (1..5) → median within [1, 5]
	hist = make([]int, nBuckets)
	hist[1] = 10
	got := medianCommitSize(hist, 10)
	if got < 1 || got > 5 {
		t.Errorf("all in [1,5]: got %d, want in [1,5]", got)
	}

	// Equal split between bucket 0 (size==0) and bucket 1 (size 1..5)
	// target = 10/2 = 5; bucket 0 has 5 → cum reaches 5, not > 5;
	// bucket 1 has 5 → 5+5=10 > 5 → median falls in [1,5]
	hist = make([]int, nBuckets)
	hist[0] = 5
	hist[1] = 5
	got = medianCommitSize(hist, 10)
	if got < 1 || got > 5 {
		t.Errorf("5×0, 5×[1-5]: got %d, want in [1,5]", got)
	}
}

// ---------------------------------------------------------------------------
// histBucket
// ---------------------------------------------------------------------------

func TestHistBucket(t *testing.T) {
	edges := []int{1, 7, 30, 90}
	cases := []struct {
		v, want int
	}{
		{0, 0},
		{1, 0},
		{2, 1},
		{7, 1},
		{8, 2},
		{30, 2},
		{31, 3},
		{90, 3},
		{91, 4}, // overflow
		{999, 4},
	}
	for _, tc := range cases {
		got := histBucket(tc.v, edges)
		if got != tc.want {
			t.Errorf("histBucket(%d, %v) = %d, want %d", tc.v, edges, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// histBucketRange
// ---------------------------------------------------------------------------

func TestHistBucketRange(t *testing.T) {
	// lineLifetimeBuckets = {0, 1, 7, 30, 90, 180, 365, 730, 1825}
	edges := lineLifetimeBuckets
	cases := []struct {
		i, wantLo, wantHi int
	}{
		{0, 0, 0},       // i<=0 → (0, edges[0]) = (0, 0)
		{1, 1, 1},       // (edges[0]+1, edges[1]) = (1, 1)
		{2, 2, 7},       // (1+1, 7)
		{3, 8, 30},      // (7+1, 30)
		{4, 31, 90},     // (30+1, 90)
		{8, 731, 1825},  // (730+1, 1825)
		{9, 1826, 1826}, // overflow
	}
	for _, tc := range cases {
		lo, hi := histBucketRange(tc.i, edges)
		if lo != tc.wantLo || hi != tc.wantHi {
			t.Errorf("histBucketRange(%d, lineLifetimeBuckets) = [%d, %d], want [%d, %d]",
				tc.i, lo, hi, tc.wantLo, tc.wantHi)
		}
	}
}

// ---------------------------------------------------------------------------
// medianFromHist
// ---------------------------------------------------------------------------

func TestMedianFromHist(t *testing.T) {
	edges := lineLifetimeBuckets // {0,1,7,30,90,180,365,730,1825}

	// Zero total
	if got := medianFromHist(nil, edges, 0); got != 0 {
		t.Errorf("nil/0: got %d, want 0", got)
	}

	// All in bucket 0 (value == 0) → returns 0
	hist := make([]int, len(edges)+1)
	hist[0] = 10
	if got := medianFromHist(hist, edges, 10); got != 0 {
		t.Errorf("all in [0,0]: got %d, want 0", got)
	}

	// All in bucket 1 (value == 1) → returns 1
	hist = make([]int, len(edges)+1)
	hist[1] = 5
	if got := medianFromHist(hist, edges, 5); got != 1 {
		t.Errorf("all in [1,1]: got %d, want 1", got)
	}

	// All in bucket 2 (values 2..7) → median within [2, 7]
	hist = make([]int, len(edges)+1)
	hist[2] = 10
	got := medianFromHist(hist, edges, 10)
	if got < 2 || got > 7 {
		t.Errorf("all in [2,7]: got %d, want in [2,7]", got)
	}
}

// ---------------------------------------------------------------------------
// DayKey
// ---------------------------------------------------------------------------

func TestDayKey(t *testing.T) {
	loc := time.UTC

	// Afternoon timestamp → midnight of same day.
	t1 := time.Date(2024, 3, 15, 14, 30, 45, 999, loc)
	got := DayKey(t1)
	want := time.Date(2024, 3, 15, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Errorf("DayKey(%v) = %v, want %v", t1, got, want)
	}

	// Midnight maps to itself.
	t2 := time.Date(2024, 3, 15, 0, 0, 0, 0, loc)
	if !DayKey(t2).Equal(t2) {
		t.Errorf("DayKey at midnight should be unchanged; got %v", DayKey(t2))
	}

	// Two timestamps on the same day share the same key.
	a := DayKey(time.Date(2024, 6, 1, 8, 0, 0, 0, loc))
	b := DayKey(time.Date(2024, 6, 1, 23, 59, 59, 0, loc))
	if !a.Equal(b) {
		t.Errorf("same-day timestamps should share key: %v vs %v", a, b)
	}
}

// ---------------------------------------------------------------------------
// isDocPath
// ---------------------------------------------------------------------------

func TestIsDocPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"README.md", true},
		{"CHANGELOG.MD", true},   // case-insensitive
		{"docs/guide.txt", true}, // docs/ prefix
		{"docs/api.md", true},
		{"src/docs/notes.go", true}, // /docs/ embedded
		{"src/main.go", false},
		{"internal/pkg/foo.go", false},
		{"src/notdocs/main.go", false}, // "notdocs" ≠ "/docs/"
		{"", false},
		{"  ", false}, // whitespace only
	}
	for _, tc := range cases {
		got := isDocPath(tc.path)
		if got != tc.want {
			t.Errorf("isDocPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// NormalizeIgnore
// ---------------------------------------------------------------------------

func TestNormalizeIgnore(t *testing.T) {
	// Deduplication, trimming, lowercasing, and sorting.
	got := NormalizeIgnore([]string{"Bob", "  alice  ", "BOB", "", "carol", "carol"})
	want := []string{"alice", "bob", "carol"}
	if len(got) != len(want) {
		t.Fatalf("NormalizeIgnore len = %d, want %d; got %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("NormalizeIgnore[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	// Empty input → empty output.
	if out := NormalizeIgnore(nil); len(out) != 0 {
		t.Errorf("nil input: got %v", out)
	}
	if out := NormalizeIgnore([]string{"", "  "}); len(out) != 0 {
		t.Errorf("blank-only input: got %v", out)
	}
}

// ---------------------------------------------------------------------------
// SortBy
// ---------------------------------------------------------------------------

func TestSortBy(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	make3 := func() []Contributor {
		return []Contributor{
			{
				Name: "alice", Commits: 5, Additions: 100, Deletions: 10,
				FirstSeen: t1, LastSeen: t2,
				RetentionAlive: 80, RetentionCommits: 40,
			},
			{
				Name: "bob", Commits: 10, Additions: 50, Deletions: 5,
				FirstSeen: t1, LastSeen: t1,
				RetentionAlive: 30, RetentionCommits: 20,
			},
			{
				Name: "carol", Commits: 1, Additions: 200, Deletions: 100,
				FirstSeen: t1, LastSeen: t1,
				RetentionAlive: 200, RetentionCommits: 100,
			},
		}
	}

	names := func(cs []Contributor) []string {
		out := make([]string, len(cs))
		for i, c := range cs {
			out[i] = c.Name
		}
		return out
	}

	// SortCommits: bob(10) > alice(5) > carol(1)
	cs := make3()
	SortBy(cs, SortCommits)
	if cs[0].Name != "bob" || cs[1].Name != "alice" || cs[2].Name != "carol" {
		t.Errorf("SortCommits: got %v", names(cs))
	}

	// SortChanges: carol(300) > alice(110) > bob(55)
	cs = make3()
	SortBy(cs, SortChanges)
	if cs[0].Name != "carol" || cs[1].Name != "alice" || cs[2].Name != "bob" {
		t.Errorf("SortChanges: got %v", names(cs))
	}

	// SortRecent: alice (LastSeen=t2) first; then tie-break by commits
	cs = make3()
	SortBy(cs, SortRecent)
	if cs[0].Name != "alice" {
		t.Errorf("SortRecent: expected alice first, got %v", names(cs))
	}

	// SortActive: alice has largest span (t1→t2); bob and carol share t1→t1
	cs = make3()
	SortBy(cs, SortActive)
	if cs[0].Name != "alice" {
		t.Errorf("SortActive: expected alice first, got %v", names(cs))
	}

	// SortAlive: carol(200) > alice(80) > bob(30)
	cs = make3()
	SortBy(cs, SortAlive)
	if cs[0].Name != "carol" || cs[1].Name != "alice" || cs[2].Name != "bob" {
		t.Errorf("SortAlive: got %v", names(cs))
	}

	// SortRetention: carol(100) > alice(40) > bob(20)
	cs = make3()
	SortBy(cs, SortRetention)
	if cs[0].Name != "carol" || cs[1].Name != "alice" || cs[2].Name != "bob" {
		t.Errorf("SortRetention: got %v", names(cs))
	}

	// SortImpact (default): geometric mean of normalised commits × changes.
	// carol: commits=1/10, changes=300/300 → sqrt(0.1×1.0) ≈ 0.316
	// bob:   commits=10/10, changes=55/300 → sqrt(1.0×0.183) ≈ 0.428
	// alice: commits=5/10, changes=110/300 → sqrt(0.5×0.367) ≈ 0.428
	// bob and alice are close; carol should be last.
	cs = make3()
	SortBy(cs, SortImpact)
	if cs[len(cs)-1].Name != "carol" {
		t.Errorf("SortImpact: expected carol last (few commits), got %v", names(cs))
	}
}

// ---------------------------------------------------------------------------
// Window
// ---------------------------------------------------------------------------

func TestWindowIsZero(t *testing.T) {
	if !(Window{}).IsZero() {
		t.Error("zero Window should be IsZero")
	}
	if (Window{Months: 1}).IsZero() {
		t.Error("Window{Months:1} should not be IsZero")
	}
	if (Window{Commits: 10}).IsZero() {
		t.Error("Window{Commits:10} should not be IsZero")
	}
	if (Window{Months: 3, Commits: 50}).IsZero() {
		t.Error("Window{Months:3,Commits:50} should not be IsZero")
	}
}

func TestGitWindowArgs(t *testing.T) {
	headTime := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	// Zero window → no args.
	args := gitWindowArgs(Window{}, headTime)
	if len(args) != 0 {
		t.Errorf("zero window: got %v, want none", args)
	}

	// Months only, non-zero headTime → one --after flag.
	args = gitWindowArgs(Window{Months: 3}, headTime)
	if len(args) != 1 || args[0] == "" {
		t.Errorf("Months window: got %v", args)
	}
	// Since date = headTime - 3 months = 2024-03-01.
	if args[0] != "--after=2024-03-01" {
		t.Errorf("Months window --after: got %q, want --after=2024-03-01", args[0])
	}

	// Commits only → one -nN flag.
	args = gitWindowArgs(Window{Commits: 100}, headTime)
	if len(args) != 1 || args[0] != "-n100" {
		t.Errorf("Commits window: got %v, want [-n100]", args)
	}

	// Both Months and Commits.
	args = gitWindowArgs(Window{Months: 3, Commits: 100}, headTime)
	if len(args) != 2 {
		t.Errorf("both: got %v (len %d), want 2 args", args, len(args))
	}

	// Zero headTime with Months set → --after is skipped, window not zero
	// so non-nil slice but no --after entry.
	args = gitWindowArgs(Window{Months: 3}, time.Time{})
	if len(args) != 0 {
		t.Errorf("zero headTime + Months: got %v, want empty", args)
	}
}
