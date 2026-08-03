package gitstats

import (
	"testing"
	"time"
)

// feed streams the given commits through aggregate with the supplied identity
// resolver and returns the resulting summary.
func feed(t *testing.T, resolver *identityResolver, commits ...Commit) *Summary {
	t.Helper()
	ch := make(chan Commit, len(commits))
	for _, c := range commits {
		ch <- c
	}
	close(ch)
	sum, _ := aggregate("repo", ch, len(commits), nil, resolver, nil)
	return sum
}

// mustResolver builds a resolver and fails the test if any pattern is invalid.
func mustResolver(t *testing.T, ids []Identity) *identityResolver {
	t.Helper()
	r, err := newIdentityResolver(ids)
	if err != nil {
		t.Fatalf("newIdentityResolver: %v", err)
	}
	return r
}

func TestIdentityAggregationByEmail(t *testing.T) {
	when := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	resolver := mustResolver(t, []Identity{{
		DisplayName: "Jane Doe",
		Emails:      []string{"jane@example.com", "jane.doe@work.example"},
	}})

	sum := feed(t, resolver,
		Commit{Name: "jane", Email: "jane@example.com", When: when, Additions: 10},
		Commit{Name: "Jane D", Email: "jane.doe@work.example", When: when.Add(time.Hour), Additions: 5},
		Commit{Name: "Bob", Email: "bob@example.com", When: when, Additions: 1},
	)

	if len(sum.Contributors) != 2 {
		t.Fatalf("got %d contributors, want 2 (Jane merged + Bob)", len(sum.Contributors))
	}
	var jane *Contributor
	for i := range sum.Contributors {
		if sum.Contributors[i].Name == "Jane Doe" {
			jane = &sum.Contributors[i]
		}
	}
	if jane == nil {
		t.Fatalf("merged Jane not found; contributors: %+v", sum.Contributors)
	}
	if jane.Commits != 2 || jane.Additions != 15 {
		t.Errorf("merged Jane = %d commits / %d additions, want 2 / 15", jane.Commits, jane.Additions)
	}
}

func TestIdentityAggregationByUsername(t *testing.T) {
	when := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	resolver := mustResolver(t, []Identity{{
		DisplayName: "Jane Doe",
		Emails:      []string{"jane@example.com"},
		Usernames:   []string{"jdoe"},
	}})

	sum := feed(t, resolver,
		Commit{Name: "jane", Email: "jane@example.com", When: when, Additions: 3},
		// Matched by username even though the email is different.
		Commit{Name: "jdoe", Email: "other@example.com", When: when.Add(time.Hour), Additions: 4},
	)

	if len(sum.Contributors) != 1 {
		t.Fatalf("got %d contributors, want 1 merged", len(sum.Contributors))
	}
	c := sum.Contributors[0]
	if c.Name != "Jane Doe" || c.Commits != 2 || c.Additions != 7 {
		t.Errorf("merged = %+v, want Jane Doe / 2 commits / 7 additions", c)
	}
	// The canonical key stays the identity's first email so cross-repo merges
	// fold the same person together.
	if c.Email != "jane@example.com" {
		t.Errorf("canonical email = %q, want %q", c.Email, "jane@example.com")
	}
}

func TestNoIdentitiesKeepsPerEmail(t *testing.T) {
	when := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	sum := feed(t, mustResolver(t, nil),
		Commit{Name: "jane", Email: "jane@example.com", When: when},
		Commit{Name: "Jane D", Email: "jane.doe@work.example", When: when},
	)
	if len(sum.Contributors) != 2 {
		t.Fatalf("got %d contributors, want 2 (no aggregation)", len(sum.Contributors))
	}
}

func TestIdentityRegexBots(t *testing.T) {
	when := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	resolver := mustResolver(t, []Identity{{
		DisplayName: "Bots",
		Emails:      []string{`.*@users\.noreply\.github\.com`},
		Usernames:   []string{`.*\[bot\]`, `dependabot`},
	}})

	sum := feed(t, resolver,
		Commit{Name: "dependabot[bot]", Email: "49699333+dependabot[bot]@users.noreply.github.com", When: when, Additions: 1},
		Commit{Name: "renovate[bot]", Email: "renovate@whatever.io", When: when.Add(time.Hour), Additions: 2},
		// Matched by the email noreply pattern even though the name is not a bot.
		Commit{Name: "Some User", Email: "someuser@users.noreply.github.com", When: when.Add(2 * time.Hour), Additions: 4},
		// A regular human, untouched.
		Commit{Name: "Alice", Email: "alice@example.com", When: when, Additions: 8},
	)

	if len(sum.Contributors) != 2 {
		t.Fatalf("got %d contributors, want 2 (Bots + Alice); %+v", len(sum.Contributors), sum.Contributors)
	}
	var bots *Contributor
	for i := range sum.Contributors {
		if sum.Contributors[i].Name == "Bots" {
			bots = &sum.Contributors[i]
		}
	}
	if bots == nil {
		t.Fatalf("merged Bots not found; contributors: %+v", sum.Contributors)
	}
	if bots.Commits != 3 || bots.Additions != 7 {
		t.Errorf("merged Bots = %d commits / %d additions, want 3 / 7", bots.Commits, bots.Additions)
	}
}

func TestIdentityDisplayEmail(t *testing.T) {
	when := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	resolver := mustResolver(t, []Identity{{
		DisplayName:  "Bots",
		DisplayEmail: "bots@example.com",
		Emails:       []string{`.*@users\.noreply\.github\.com`},
		Usernames:    []string{`.*\[bot\]`},
	}})

	sum := feed(t, resolver,
		Commit{Name: "dependabot[bot]", Email: "dependabot@whatever.io", When: when, Additions: 1},
		Commit{Name: "Some User", Email: "someuser@users.noreply.github.com", When: when.Add(time.Hour), Additions: 2},
	)

	if len(sum.Contributors) != 1 {
		t.Fatalf("got %d contributors, want 1 merged", len(sum.Contributors))
	}
	c := sum.Contributors[0]
	if c.Name != "Bots" || c.Email != "bots@example.com" {
		t.Errorf("merged = %q <%s>, want Bots <bots@example.com>", c.Name, c.Email)
	}
	if c.Commits != 2 || c.Additions != 3 {
		t.Errorf("merged Bots = %d commits / %d additions, want 2 / 3", c.Commits, c.Additions)
	}
}

func TestValidateIdentitiesBadRegex(t *testing.T) {
	err := ValidateIdentities([]Identity{{
		DisplayName: "Broken",
		Emails:      []string{"([unterminated"},
	}})
	if err == nil {
		t.Fatal("expected an error for an invalid regular expression")
	}
}
