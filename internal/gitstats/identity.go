package gitstats

// identity.go adds user aggregation: several git author identities (different
// emails and/or author names) that belong to the same person can be merged into
// a single contributor. Matching is driven by an explicit list of identities,
// typically loaded from the config file's `users:` section. Each email/username
// entry is a regular expression (matched case-insensitively), which makes it
// easy to fold in bots and generated committers, e.g. `.*\[bot\]` or
// `.*@users\.noreply\.github\.com`.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Identity describes a person (or bot) who commits under several git author
// identities. All commits whose author email matches one of Emails or whose
// author name matches one of Usernames are aggregated into a single contributor
// labeled DisplayName. Emails and Usernames are regular expressions matched
// case-insensitively and unanchored (so `bot` matches anywhere in the value).
// DisplayEmail, when set, is the email shown for the aggregated contributor;
// otherwise the first email pattern is used (which may be a raw regular
// expression), so it is handy for bots or multi-address people.
type Identity struct {
	DisplayName  string   `json:"display_name"`
	DisplayEmail string   `json:"display_email,omitempty"`
	Emails       []string `json:"emails"`
	Usernames    []string `json:"usernames"`
}

// NormalizeIdentities trims the match patterns, drops empty entries, sorts every
// list and the identities themselves so that equivalent configurations produce
// an identical value. Patterns keep their original case (they are matched
// case-insensitively at resolve time) so regular-expression semantics are
// preserved. It is used both to build a resolver and to key the on-disk cache.
func NormalizeIdentities(ids []Identity) []Identity {
	out := make([]Identity, 0, len(ids))
	for _, id := range ids {
		name := strings.TrimSpace(id.DisplayName)
		if name == "" {
			continue
		}
		n := Identity{
			DisplayName:  name,
			DisplayEmail: strings.TrimSpace(id.DisplayEmail),
			Emails:       normalizeList(id.Emails),
			Usernames:    normalizeList(id.Usernames),
		}
		if len(n.Emails) == 0 && len(n.Usernames) == 0 {
			continue
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].DisplayName) < strings.ToLower(out[j].DisplayName)
	})
	return out
}

// normalizeList trims, de-duplicates and sorts a list of match patterns.
func normalizeList(list []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(list))
	for _, v := range list {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

// ValidateIdentities reports the first invalid regular expression in the
// identity list, or nil when every pattern compiles. It lets the CLI surface a
// clear error before scanning begins.
func ValidateIdentities(ids []Identity) error {
	_, err := newIdentityResolver(ids)
	return err
}

// identityMatcher holds the compiled email/name patterns for a single identity
// together with its canonical grouping key and display name.
type identityMatcher struct {
	emailRes     []*regexp.Regexp
	nameRes      []*regexp.Regexp
	canon        string
	display      string
	displayEmail string
}

// identityResolver maps a commit's author name/email onto a canonical grouping
// key and display name. It is built once per collection from a list of
// identities and consulted for every commit.
type identityResolver struct {
	matchers []identityMatcher
}

// newIdentityResolver builds a resolver from a list of identities, compiling the
// email/username patterns as case-insensitive regular expressions. A nil or
// empty list yields a nil resolver, which resolve treats as "no aggregation".
// An error is returned when any pattern fails to compile.
func newIdentityResolver(ids []Identity) (*identityResolver, error) {
	ids = NormalizeIdentities(ids)
	if len(ids) == 0 {
		return nil, nil
	}
	compile := func(patterns []string) ([]*regexp.Regexp, error) {
		res := make([]*regexp.Regexp, 0, len(patterns))
		for _, p := range patterns {
			re, err := regexp.Compile("(?i)" + p)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern %q: %w", p, err)
			}
			res = append(res, re)
		}
		return res, nil
	}
	r := &identityResolver{}
	for _, id := range ids {
		emailRes, err := compile(id.Emails)
		if err != nil {
			return nil, fmt.Errorf("users %q: %w", id.DisplayName, err)
		}
		nameRes, err := compile(id.Usernames)
		if err != nil {
			return nil, fmt.Errorf("users %q: %w", id.DisplayName, err)
		}
		// The canonical grouping key is the identity's first email pattern so it
		// stays stable across repositories (letting MergeSummaries fold the same
		// person together). Identities defined only by username fall back to a
		// synthetic, still-stable key derived from the display name.
		canon := ""
		if len(id.Emails) > 0 {
			canon = id.Emails[0]
		} else {
			canon = "id:" + strings.ToLower(id.DisplayName)
		}
		// The email shown for the contributor: the configured DisplayEmail when
		// present, otherwise the canonical key (the first email pattern, or the
		// synthetic username-only key).
		displayEmail := id.DisplayEmail
		if displayEmail == "" {
			displayEmail = canon
		}
		r.matchers = append(r.matchers, identityMatcher{
			emailRes:     emailRes,
			nameRes:      nameRes,
			canon:        canon,
			display:      id.DisplayName,
			displayEmail: displayEmail,
		})
	}
	return r, nil
}

// resolve returns the canonical grouping key, display name and display email for
// a commit's author name/email. matched reports whether an identity matched;
// when it does not, the raw email (as both the key and the display email) and
// name are returned so unmatched authors keep the default per-email behavior.
// Identities are tested in normalized order, so the first match wins.
func (r *identityResolver) resolve(name, email string) (canon, display, displayEmail string, matched bool) {
	if r == nil {
		return email, name, email, false
	}
	for i := range r.matchers {
		m := &r.matchers[i]
		for _, re := range m.emailRes {
			if re.MatchString(email) {
				return m.canon, m.display, m.displayEmail, true
			}
		}
		for _, re := range m.nameRes {
			if re.MatchString(name) {
				return m.canon, m.display, m.displayEmail, true
			}
		}
	}
	return email, name, email, false
}
