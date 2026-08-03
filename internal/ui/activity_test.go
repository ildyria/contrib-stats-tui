package ui

import (
	"math"
	"testing"

	"github.com/ildyria/contrib-stats-tui/internal/gitstats"
)

// ---------------------------------------------------------------------------
// giniCoefficient
// ---------------------------------------------------------------------------

func TestGiniCoefficient(t *testing.T) {
	cases := []struct {
		desc string
		vals []int
		want float64
		eps  float64
	}{
		{"empty", nil, 0, 0},
		{"single", []int{100}, 0, 0},
		{"all equal", []int{50, 50, 50}, 0, 1e-9},
		{"all zero", []int{0, 0, 0}, 0, 0},
		{"total inequality n=2", []int{0, 100}, 0.5, 1e-9},
		// [10, 10, 80]: weighted = 1*10+2*10+3*80=270, sum=100, n=3
		// g = 2*270/(3*100) - 4/3 = 1.8 - 1.333... ≈ 0.467
		{"skewed n=3", []int{10, 10, 80}, 0.4667, 0.0001},
	}
	for _, tc := range cases {
		got := giniCoefficient(tc.vals)
		if math.Abs(got-tc.want) > tc.eps {
			t.Errorf("giniCoefficient(%v) [%s] = %.6f, want %.6f (±%.0e)",
				tc.vals, tc.desc, got, tc.want, tc.eps)
		}
	}
}

func TestGiniCoefficientNonNegative(t *testing.T) {
	// The formula can produce negative values for very even distributions;
	// the implementation clamps to 0.
	got := giniCoefficient([]int{1, 1, 1, 1, 1})
	if got < 0 {
		t.Errorf("giniCoefficient should never be negative; got %v", got)
	}
}

// ---------------------------------------------------------------------------
// punchStatsOf
// ---------------------------------------------------------------------------

func TestPunchStatsOf(t *testing.T) {
	s := &gitstats.Summary{}
	// Put the busiest slot on Wednesday (wd=3) at hour 14.
	s.Punch[3][14] = 50
	// Secondary slot on Friday (wd=5) at hour 9.
	s.Punch[5][9] = 30

	st := punchStatsOf(s)

	if st.bestD != 3 || st.bestH != 14 {
		t.Errorf("bestD/bestH = %d/%d, want 3/14", st.bestD, st.bestH)
	}
	if st.slotCount != 50 {
		t.Errorf("slotCount = %d, want 50", st.slotCount)
	}
	// Best hour overall: hour 14 has 50, hour 9 has 30 → hour 14.
	if st.bestHour != 14 {
		t.Errorf("bestHour = %d, want 14", st.bestHour)
	}
	if st.bestHourCount != 50 {
		t.Errorf("bestHourCount = %d, want 50", st.bestHourCount)
	}
	// Busiest day: Wednesday has 50, Friday has 30 → Wednesday (3).
	if st.bestDay != 3 {
		t.Errorf("bestDay = %d, want 3", st.bestDay)
	}
	if st.bestDayCount != 50 {
		t.Errorf("bestDayCount = %d, want 50", st.bestDayCount)
	}
}

func TestPunchStatsOfAllZero(t *testing.T) {
	s := &gitstats.Summary{}
	st := punchStatsOf(s)
	if st.slotCount != 0 || st.bestHourCount != 0 || st.bestDayCount != 0 {
		t.Errorf("all-zero punch card should produce zero stats: %+v", st)
	}
}

// ---------------------------------------------------------------------------
// workPatternOf
// ---------------------------------------------------------------------------

func TestWorkPatternOfWeekdayInHours(t *testing.T) {
	s := &gitstats.Summary{}
	// Monday (1) at 09:00: not weekend, not after-hours.
	s.Punch[1][9] = 10
	we, ah := workPatternOf(s)
	if we != 0 {
		t.Errorf("weekend = %v, want 0", we)
	}
	if ah != 0 {
		t.Errorf("afterHours = %v, want 0", ah)
	}
}

func TestWorkPatternOfWeekend(t *testing.T) {
	s := &gitstats.Summary{}
	// Saturday (6) at 10:00: weekend, not after-hours.
	s.Punch[6][10] = 10
	we, ah := workPatternOf(s)
	if we != 1.0 {
		t.Errorf("weekend = %v, want 1.0", we)
	}
	if ah != 0 {
		t.Errorf("afterHours = %v, want 0", ah)
	}
}

func TestWorkPatternOfAfterHours(t *testing.T) {
	s := &gitstats.Summary{}
	// Monday (1) at 20:00: not weekend, after-hours (h >= 18).
	s.Punch[1][20] = 10
	we, ah := workPatternOf(s)
	if we != 0 {
		t.Errorf("weekend = %v, want 0", we)
	}
	if ah != 1.0 {
		t.Errorf("afterHours = %v, want 1.0", ah)
	}
}

func TestWorkPatternOfEarlyMorning(t *testing.T) {
	s := &gitstats.Summary{}
	// Wednesday (3) at 05:00: not weekend, after-hours (h < 8).
	s.Punch[3][5] = 8
	we, ah := workPatternOf(s)
	if we != 0 {
		t.Errorf("weekend = %v, want 0", we)
	}
	if ah != 1.0 {
		t.Errorf("afterHours = %v, want 1.0 (early morning counts)", ah)
	}
}

func TestWorkPatternOfMixed(t *testing.T) {
	s := &gitstats.Summary{}
	// 5 commits Saturday in-hours + 5 commits Monday in-hours.
	s.Punch[6][10] = 5 // Saturday
	s.Punch[1][10] = 5 // Monday
	we, ah := workPatternOf(s)
	if math.Abs(we-0.5) > 1e-9 {
		t.Errorf("weekend = %v, want 0.5", we)
	}
	if ah != 0 {
		t.Errorf("afterHours = %v, want 0", ah)
	}
}

func TestWorkPatternOfEmpty(t *testing.T) {
	s := &gitstats.Summary{}
	we, ah := workPatternOf(s)
	if we != 0 || ah != 0 {
		t.Errorf("empty summary: got (%v, %v), want (0, 0)", we, ah)
	}
}

// ---------------------------------------------------------------------------
// humanizeDays
// ---------------------------------------------------------------------------

func TestHumanizeDays(t *testing.T) {
	cases := []struct {
		d    int
		want string
	}{
		{0, "0 days"},
		{-5, "0 days"},
		{1, "1 day"},
		{2, "2 days"},
		{59, "59 days"},
		{60, "2.0 months"}, // 60/30.44 ≈ 1.97 → "2.0 months"
		{91, "3.0 months"}, // 91/30.44 ≈ 2.99
		{364, "12.0 months"},
		{365, "1.0 years"},
		{730, "2.0 years"},
	}
	for _, tc := range cases {
		got := humanizeDays(tc.d)
		if got != tc.want {
			t.Errorf("humanizeDays(%d) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// sizeBucketLabel
// ---------------------------------------------------------------------------

func TestSizeBucketLabel(t *testing.T) {
	// CommitSizeBuckets = {0,5,10,25,50,100,250,500,1000,2500,5000}
	cases := []struct {
		i    int
		want string
	}{
		{0, "0"},
		{1, "1–5"},
		{2, "6–10"},
		{3, "11–25"},
		{4, "26–50"},
		{5, "51–100"},
		{6, "101–250"},
		{7, "251–500"},
		{8, "501–1,000"},
		{9, "1,001–2,500"},
		{10, "2,501–5,000"},
		{11, "5,001+"}, // overflow
	}
	for _, tc := range cases {
		got := sizeBucketLabel(tc.i)
		if got != tc.want {
			t.Errorf("sizeBucketLabel(%d) = %q, want %q", tc.i, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// level (heatmap intensity)
// ---------------------------------------------------------------------------

func TestLevel(t *testing.T) {
	// punchScale.cuts = {0.02, 0.10, 0.30, 0.50, 0.75}
	// heatColors has 7 entries (indices 0..6).
	th := punchScale

	cases := []struct {
		count, maxCount, want int
	}{
		{0, 100, 0},   // zero → always 0
		{-1, 100, 0},  // negative → 0
		{1, 100, 1},   // ratio 0.01 ≤ 0.02 → lvl=1
		{3, 100, 2},   // ratio 0.03 > 0.02 → lvl=2
		{15, 100, 3},  // ratio 0.15 > 0.10 → lvl=3
		{40, 100, 4},  // ratio 0.40 > 0.30 → lvl=4
		{55, 100, 5},  // ratio 0.55 > 0.50 → lvl=5
		{80, 100, 6},  // ratio 0.80 > 0.75 → lvl=6
		{100, 100, 6}, // ratio 1.0 → max level
	}
	for _, tc := range cases {
		got := level(tc.count, tc.maxCount, th)
		if got != tc.want {
			t.Errorf("level(%d, %d, punchScale) = %d, want %d",
				tc.count, tc.maxCount, got, tc.want)
		}
	}
}

func TestLevelCalendarScale(t *testing.T) {
	// calendarScale.cuts = {0.03, 0.08, 0.20, 0.35, 0.50}
	th := calendarScale

	// count=0 → 0
	if got := level(0, 10, th); got != 0 {
		t.Errorf("level(0, 10, calendarScale) = %d, want 0", got)
	}
	// count = maxCount → highest level (6, clamped to len(heatColors)-1 = 6)
	got := level(10, 10, th)
	if got != 6 {
		t.Errorf("level(maxCount, maxCount, calendarScale) = %d, want 6", got)
	}
}
