package ui

import (
	"fmt"
	"strings"
)

// resample compresses data into n buckets by summing, leaving it unchanged when
// it already fits.
func resample(data []float64, n int) []float64 {
	if len(data) <= n {
		return data
	}
	out := make([]float64, n)
	for i, v := range data {
		bucket := i * n / len(data)
		if bucket >= n {
			bucket = n - 1
		}
		out[bucket] += v
	}
	return out
}

// humanize formats an integer with thousands separators.
func humanize(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s
	}
	var out []byte
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	for i, ch := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, ch)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// truncate shortens s to at most n runes, appending an ellipsis when cut.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
