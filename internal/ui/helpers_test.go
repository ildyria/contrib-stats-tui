package ui

import (
	"testing"
)

// ---------------------------------------------------------------------------
// humanize
// ---------------------------------------------------------------------------

func TestHumanize(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1000000, "1,000,000"},
		// Negative numbers: the function returns early when n < 1000,
		// so negatives are returned as-is without separators.
		{-1234, "-1234"},
		{-999, "-999"},
	}
	for _, tc := range cases {
		got := humanize(tc.n)
		if got != tc.want {
			t.Errorf("humanize(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// truncate
// ---------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},         // shorter than limit
		{"hello", 5, "hello"},          // exactly the limit
		{"hello world", 8, "hello w…"}, // truncated with ellipsis
		{"hello", 1, "h"},              // n==1: no room for ellipsis
		{"hello", 0, ""},               // n==0: empty
		{"αβγδε", 3, "αβ…"},            // multi-byte runes
	}
	for _, tc := range cases {
		got := truncate(tc.s, tc.n)
		if got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// resample
// ---------------------------------------------------------------------------

func TestResample(t *testing.T) {
	// No resampling needed when input fits.
	data := []float64{1, 2, 3}
	got := resample(data, 5)
	if len(got) != 3 {
		t.Errorf("resample fits: got len %d, want 3", len(got))
	}
	for i, v := range data {
		if got[i] != v {
			t.Errorf("resample fits[%d] = %v, want %v", i, got[i], v)
		}
	}

	// Compress 6 values into 3 buckets.
	// Values: [1,2,3,4,5,6], n=3
	// bucket mapping: i*3/6 → 0,0,1,1,2,2
	// bucket 0: 1+2=3, bucket 1: 3+4=7, bucket 2: 5+6=11
	data6 := []float64{1, 2, 3, 4, 5, 6}
	got3 := resample(data6, 3)
	if len(got3) != 3 {
		t.Fatalf("resample 6→3: got len %d, want 3", len(got3))
	}
	if got3[0] != 3 || got3[1] != 7 || got3[2] != 11 {
		t.Errorf("resample 6→3: got %v, want [3 7 11]", got3)
	}

	// Empty input → empty output.
	if got := resample(nil, 3); len(got) != 0 {
		t.Errorf("resample nil: got %v", got)
	}
}
