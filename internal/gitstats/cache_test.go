package gitstats

import (
	"os"
	"testing"
)

func TestCacheRoundTrip(t *testing.T) {
	repo := os.Getenv("TEST_REPO")
	if repo == "" {
		t.Skip("TEST_REPO not set")
	}
	// Isolate the cache directory so we don't touch the real one.
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	sum1, cached1, err := CollectCached(repo, true, nil, false, nil, Window{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cached1 {
		t.Fatalf("first call should not be cached")
	}

	sum2, cached2, err := CollectCached(repo, true, nil, false, nil, Window{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cached2 {
		t.Fatalf("second call should be served from cache")
	}
	if sum2.TotalCommits != sum1.TotalCommits ||
		sum2.Additions != sum1.Additions ||
		sum2.Deletions != sum1.Deletions ||
		len(sum2.Contributors) != len(sum1.Contributors) {
		t.Fatalf("cached summary differs: %+v vs %+v", sum2, sum1)
	}
	if len(sum2.Daily) != len(sum1.Daily) {
		t.Fatalf("cached daily differs: %d vs %d", len(sum2.Daily), len(sum1.Daily))
	}
	if sum2.Punch != sum1.Punch {
		t.Fatalf("cached punch differs")
	}
	t.Logf("ok: commits=%d contributors=%d cachedSecond=%v",
		sum2.TotalCommits, len(sum2.Contributors), cached2)

	// A different build identity must invalidate the cache.
	orig := Version
	Version = orig + "-changed"
	defer func() { Version = orig }()
	_, cached3, err := CollectCached(repo, true, nil, false, nil, Window{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cached3 {
		t.Fatalf("cache should be discarded when the build ID changes")
	}
}
