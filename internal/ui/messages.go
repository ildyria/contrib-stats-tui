package ui

import "github.com/ildyria/contrib-stats-tui/internal/gitstats"

// scanProgressMsg reports scanning progress from the background collector.
type scanProgressMsg struct{ done, total int }

// scanDoneMsg reports completion of the background collector. repos holds the
// per-repository summaries (successful only), global is their merged aggregate,
// and results carries every spec's outcome (including errors for skipped
// repositories). cached is true only in single-repo mode when that repo's
// summary came from the on-disk cache.
type scanDoneMsg struct {
	repos   []*gitstats.Summary
	global  *gitstats.Summary
	results []gitstats.RepoResult
	cached  bool
	err     error
}

// cheekyTickMsg advances the rotating "flavor" line shown while scanning.
type cheekyTickMsg struct{}

// cheekyLines are tongue-in-cheek status messages shown one at a time under the
// progress bar to make the wait feel busier than it really is.
var cheekyLines = []string{
	"Walking the commit graph…",
	"Parsing diff hunks…",
	"Aggregating per-author changes…",
	"Resolving author identities…",
	"Computing line retention…",
	"Tracing line ownership through history…",
	"Building the contribution calendar…",
	"Bucketing commits by weekday and hour…",
	"Following renames across commits…",
	"Reconciling merge commits…",
	"Normalizing author emails…",
	"Tallying additions and deletions…",
	"Indexing commit timestamps…",
	"Folding history into weekly buckets…",
	"Scoring impact per contributor…",
	"Hunting down dead lines…",
	"Counting the fallen lines of code…",
	"Checking how long contributors stuck around…",
	"Measuring each contributor's active days…",
	"Side-eyeing that Friday 5pm commit…",
	"Squinting at commits made at 3am…",
	"Flagging suspiciously late-night pushes…",
	"Sorting contributors…",
	"Crunching the numbers…",
	"Almost there, tidying up…",
	"Bribing the git gods for faster history…",
	"Untangling merge commits…",
	"Blaming everyone equally…",
	"Consulting the reflog oracle…",
	"Measuring bus factor with a ruler…",
}
