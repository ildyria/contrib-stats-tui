package cmd

import (
	"fmt"
	"sort"

	"github.com/ildyria/contrib-stats-tui/internal/gitstats"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// newEmailCmd builds the `email` subcommand, which scans the configured
// repositories and prints the unique list of contributors as plain
// `name: email (N commits)` lines, sorted by commit count (descending). It is
// deliberately plain text (not the TUI) so the full list can be selected,
// piped or counted to quickly see how many distinct users there are.
func newEmailCmd() *cobra.Command {
	v := viper.New()

	cmd := &cobra.Command{
		Use:   "email [path|url ...|config.yaml]",
		Short: "List unique contributors (name: email) across the configured repositories",
		Long: "Scan the configured repositories and print the unique list of " +
			"contributors as `name: email (N commits)` lines, sorted by number " +
			"of commits (descending).\n" +
			"Repositories are taken from arguments (directories or clone URLs) or, " +
			"failing that, from the `repositories:` list in the config file. The " +
			"same `ignore:` and `users:` settings as the main command apply, so " +
			"aggregated identities are folded together. Output is plain text " +
			"(not the TUI) so it can be selected, piped or counted.",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			setupViper(v)

			repoArgs, configArg := classifyArgs(args)
			if configArg != "" {
				v.SetConfigFile(configArg)
			}

			configFound := true
			if err := v.ReadInConfig(); err != nil {
				if _, ok := err.(viper.ConfigFileNotFoundError); ok {
					configFound = false
				} else {
					return fmt.Errorf("reading config: %w", err)
				}
			}
			if len(repoArgs) == 0 && configArg == "" && !configFound {
				return cmd.Help()
			}

			if err := v.BindPFlag("ignore", cmd.Flags().Lookup("ignore")); err != nil {
				return err
			}
			if err := v.BindPFlag("exclude-docs", cmd.Flags().Lookup("exclude-docs")); err != nil {
				return err
			}
			if err := v.BindPFlag("time-window", cmd.Flags().Lookup("time-window")); err != nil {
				return err
			}
			if err := v.BindPFlag("commit-window", cmd.Flags().Lookup("commit-window")); err != nil {
				return err
			}

			ignore := v.GetStringSlice("ignore")
			excludeDocs := v.GetBool("exclude-docs")
			identities, err := parseIdentities(v)
			if err != nil {
				return err
			}
			window, err := parseWindow(v)
			if err != nil {
				return err
			}

			specs := resolveSpecs(v, repoArgs)
			if len(specs) == 0 {
				return cmd.Help()
			}

			// Repositories given directly on the command line must be git
			// repositories; fail fast rather than scanning an empty tree. Clone
			// URLs are validated when they are cloned.
			if len(repoArgs) > 0 {
				for _, s := range specs {
					if !s.IsClone && !gitstats.IsGitRepo(s.LocalPath) {
						return fmt.Errorf("%s is not a git repository", s.Raw)
					}
				}
			}

			_, global, results := gitstats.CollectMulti(specs, true, ignore, excludeDocs, identities, window, nil)

			// Report any repositories that could not be scanned, but continue as
			// long as at least one succeeded.
			for _, r := range results {
				if r.Err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(),
						errorStyle.Render(fmt.Sprintf("skipped %s: %v", r.Spec.Raw, r.Err)))
				}
			}
			if global == nil || len(global.Contributors) == 0 {
				return fmt.Errorf("no contributors found")
			}

			// Sort by commits (descending), breaking ties by name then email so
			// the output is stable.
			contribs := global.Contributors
			sort.SliceStable(contribs, func(i, j int) bool {
				if contribs[i].Commits != contribs[j].Commits {
					return contribs[i].Commits > contribs[j].Commits
				}
				if contribs[i].Name != contribs[j].Name {
					return contribs[i].Name < contribs[j].Name
				}
				return contribs[i].Email < contribs[j].Email
			})

			out := cmd.OutOrStdout()
			for _, c := range contribs {
				fmt.Fprintf(out, "%s: %s (%d commits)\n", c.Name, c.Email, c.Commits)
			}
			fmt.Fprintf(out, "\n%d unique contributors\n", len(contribs))
			return nil
		},
	}

	cmd.Flags().StringSlice("ignore", nil,
		"contributor names or emails whose commits are excluded (repeatable or comma-separated)")
	cmd.Flags().Bool("exclude-docs", false,
		"ignore all changes to Markdown files (*.md) and docs/ folders")
	cmd.Flags().Int("time-window", 0,
		"only consider the last N months of history, measured from HEAD (0 = full history)")
	cmd.Flags().Int("commit-window", 0,
		"only consider the most recent N commits (0 = no limit)")
	return cmd
}
