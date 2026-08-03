package gitstats

// collector.go is the collection layer: it runs git and parses its output into
// a stream of Commit values. It performs no aggregation, keeping git-specific
// concerns isolated from the statistical folding done in aggregator.go.

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// recordSep is an ASCII record separator used between commit-header fields. It
// keeps parsing unambiguous even when author names contain unusual characters.
const recordSep = "\x1e"

// streamCommits runs `git log` in top, parses each commit, and sends it on out.
// It always closes out before returning and reports the first fatal error (if
// any). Running it in its own goroutine lets aggregation proceed concurrently.
// When excludeDocs is true, numstat lines for documentation files (Markdown or
// files under docs/) are skipped so their additions/deletions do not count.
func streamCommits(top string, out chan<- Commit, excludeDocs bool, extraArgs []string) (err error) {
	defer close(out)

	format := "%an" + recordSep + "%ae" + recordSep + "%at"
	args := []string{"-C", top, "log",
		"--no-merges",
		"--numstat",
		"--date=raw",
		"--pretty=format:C" + recordSep + format,
	}
	args = append(args, extraArgs...)
	cmd := exec.Command("git", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating git pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting git log: %w", err)
	}

	var cur Commit
	var have bool
	flush := func() {
		if have {
			out <- cur
		}
		cur = Commit{}
		have = false
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "C"+recordSep) {
			flush()
			fields := strings.Split(strings.TrimPrefix(line, "C"+recordSep), recordSep)
			if len(fields) < 3 {
				have = false
				continue
			}
			ts, _ := strconv.ParseInt(fields[2], 10, 64)
			cur = Commit{
				Name:  fields[0],
				Email: strings.ToLower(fields[1]),
				When:  time.Unix(ts, 0),
			}
			have = true
			continue
		}

		// numstat line: additions\tdeletions\tpath  (binary files show "-")
		if !have {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		if excludeDocs && isDocPath(parts[2]) {
			continue
		}
		add, _ := strconv.Atoi(parts[0]) // "-" for binary files -> 0
		del, _ := strconv.Atoi(parts[1])
		cur.Additions += add
		cur.Deletions += del
	}
	flush()

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning git output: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git log failed: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

// repoRoot resolves the top-level directory of the git repository containing
// path.
func repoRoot(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s is not a git repository", path)
	}
	return strings.TrimSpace(string(out)), nil
}

// countCommits returns the number of non-merge commits reachable from HEAD.
// It is best-effort and returns 0 on any error so callers can fall back to an
// indeterminate progress display.
func countCommits(top string, extraArgs []string) int {
	args := []string{"-C", top, "rev-list", "--no-merges", "--count"}
	args = append(args, extraArgs...)
	args = append(args, "HEAD")
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return n
}
