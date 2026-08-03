package gitstats

import (
	"os"
	"testing"
)

func TestCollectSmoke(t *testing.T) {
	repo := os.Getenv("TEST_REPO")
	if repo == "" {
		t.Skip("TEST_REPO not set")
	}
	sum, err := Collect(repo, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("commits=%d add=%d del=%d contributors=%d weeks=%d days=%d",
		sum.TotalCommits, sum.Additions, sum.Deletions,
		len(sum.Contributors), sum.WeekCount, len(sum.Daily))
	for _, c := range sum.Contributors {
		t.Logf("  %-6s commits=%d +%d -%d weekly=%v",
			c.Name, c.Commits, c.Additions, c.Deletions, c.Weekly)
	}
	for d, n := range sum.Daily {
		t.Logf("  day %s => %d", d.Format("2006-01-02"), n)
	}
	for wd := 0; wd < 7; wd++ {
		for h := 0; h < 24; h++ {
			if sum.Punch[wd][h] > 0 {
				t.Logf("  punch wd=%d h=%d => %d", wd, h, sum.Punch[wd][h])
			}
		}
	}
}
