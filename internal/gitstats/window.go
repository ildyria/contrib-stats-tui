package gitstats

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Window constrains the commit history that is scanned. A zero Window (both
// fields zero) means the full history is considered.
type Window struct {
	// Months, when > 0, limits the scan to commits authored no earlier than
	// HEAD's date minus Months months.
	Months int `json:"months,omitempty"`
	// Commits, when > 0, limits the scan to the most recent N commits.
	Commits int `json:"commits,omitempty"`
}

// IsZero reports whether the window places no constraint on the history.
func (w Window) IsZero() bool {
	return w.Months == 0 && w.Commits == 0
}

// gitWindowArgs returns the extra `git log` / `git rev-list` arguments that
// implement the window. headTime is the author timestamp of the HEAD commit;
// it must have been fetched before calling this function (use headCommitTime).
// When headTime is zero the time-window constraint is silently skipped.
func gitWindowArgs(w Window, headTime time.Time) []string {
	if w.IsZero() {
		return nil
	}
	var args []string
	if w.Months > 0 && !headTime.IsZero() {
		since := headTime.AddDate(0, -w.Months, 0)
		// --after is a synonym for --since; it filters by author date and is
		// exclusive of the given date (i.e. commits ON that date are included).
		args = append(args, "--after="+since.Format("2006-01-02"))
	}
	if w.Commits > 0 {
		args = append(args, fmt.Sprintf("-n%d", w.Commits))
	}
	return args
}

// headCommitTime returns the author timestamp of the HEAD commit of a
// repository rooted at top. It is best-effort and returns a zero time on any
// error (e.g. empty repository).
func headCommitTime(top string) time.Time {
	out, err := exec.Command("git", "-C", top, "log", "-1", "--format=%at").Output()
	if err != nil {
		return time.Time{}
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(ts, 0)
}
