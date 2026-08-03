// Package cmd defines the command-line interface for contributors: the root
// command and its subcommands, wired with Cobra and Viper.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ildyria/contrib-stats-tui/internal/gitstats"
	"github.com/ildyria/contrib-stats-tui/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// errorStyle renders fatal command-line errors in bold red.
var errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)

// Execute builds the root command and runs it. Any error is printed in red to
// stderr and returned so the caller (main) can set the process exit code.
func Execute() error {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("Error: "+err.Error()))
		return err
	}
	return nil
}

// newRootCmd builds the root `contributors` command that scans a repository and
// launches the terminal UI.
func newRootCmd() *cobra.Command {
	v := viper.New()

	cmd := &cobra.Command{
		Use:   "contributors [path|url ...|config.yaml]",
		Short: "GitHub-style contributors TUI for a git repository",
		Long: "Render a GitHub-style contributors page and contribution calendar " +
			"for a git repository as a terminal UI.\n" +
			"Arguments may be directories (git repositories to scan) and/or git " +
			"clone URLs (cloned into a per-user cache and then scanned); when more " +
			"than one is given they are aggregated as a multi-repository run. A " +
			"single YAML file is instead used as the config file. If no argument " +
			"is given, the current directory is used (provided a config file is " +
			"present); otherwise this help is shown.",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Config file discovery: ./.contributors.yaml or ~/.contributors.yaml
			setupViper(v)

			// Classify positional arguments. A single YAML file is an explicit
			// config file; otherwise every argument is a repository to scan (a
			// directory or a clone URL). Multiple arguments are aggregated as a
			// multi-repository run. The clone-URL check takes precedence so a URL
			// ending in .yaml is still cloned rather than read as a config file.
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

			// With no repo arguments and no config file to fall back on, there is
			// nothing to analyze; show the help text instead of scanning (and
			// failing on) whatever the current directory happens to be.
			if len(repoArgs) == 0 && configArg == "" && !configFound {
				return cmd.Help()
			}
			if err := v.BindPFlag("week-start", cmd.Flags().Lookup("week-start")); err != nil {
				return err
			}
			if err := v.BindPFlag("no-cache", cmd.Flags().Lookup("no-cache")); err != nil {
				return err
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

			weekStart, err := parseWeekStart(v.GetString("week-start"))
			if err != nil {
				return err
			}
			useCache := !v.GetBool("no-cache")
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

			// Repository selection. Path/URL arguments always take precedence and
			// yield a single- or multi-repo run; otherwise the config's
			// `repositories:` list (resolved relative to the config file) is used;
			// failing that, the current directory is scanned.
			specs := resolveSpecs(v, repoArgs)
			if len(specs) == 0 {
				return cmd.Help()
			}

			// Repositories given directly on the command line must be git
			// repositories; fail fast (with a red error) rather than launching the
			// UI onto an empty scan. Clone URLs are validated when they are cloned.
			if len(repoArgs) > 0 {
				for _, s := range specs {
					if !s.IsClone && !gitstats.IsGitRepo(s.LocalPath) {
						return fmt.Errorf("%s is not a git repository", s.Raw)
					}
				}
			}

			p := tea.NewProgram(ui.New(specs, weekStart, useCache, ignore, excludeDocs, identities, window), tea.WithAltScreen())
			final, err := p.Run()
			if err != nil {
				return err
			}
			if fm, ok := final.(ui.Model); ok {
				if scanErr := fm.Err(); scanErr != nil {
					return scanErr
				}
			}
			return nil
		},
	}

	cmd.Flags().String("week-start", "monday",
		"first day of the calendar week: monday or sunday")
	cmd.Flags().Bool("no-cache", false,
		"ignore and overwrite the on-disk cache (force a full rescan)")
	cmd.Flags().StringSlice("ignore", nil,
		"contributor names or emails whose commits are excluded (repeatable or comma-separated)")
	cmd.Flags().Bool("exclude-docs", false,
		"ignore all changes to Markdown files (*.md) and docs/ folders")
	cmd.Flags().Int("time-window", 0,
		"only consider the last N months of history, measured from HEAD (0 = full history)")
	cmd.Flags().Int("commit-window", 0,
		"only consider the most recent N commits (0 = no limit)")

	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newEmailCmd())

	return cmd
}

// setupViper configures the shared config-file discovery
// (./.contributors.yaml or ~/.contributors.yaml) and environment binding used
// by every command.
func setupViper(v *viper.Viper) {
	v.SetConfigName(".contributors")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(home)
	}
	v.SetEnvPrefix("CONTRIBUTORS")
	// Replace hyphens with underscores so config keys like "week-start" map to
	// environment variables like CONTRIBUTORS_WEEK_START.
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
}

// classifyArgs splits positional arguments into repositories to scan and an
// optional explicit config file. A single YAML file that is not a clone URL is
// treated as a config file; everything else is a repository (a directory or a
// clone URL).
func classifyArgs(args []string) (repoArgs []string, configArg string) {
	if len(args) == 1 && isYAMLFile(args[0]) && !gitstats.IsCloneURL(args[0]) {
		return nil, args[0]
	}
	return args, ""
}

// resolveSpecs selects the repositories to scan: command-line arguments take
// precedence; otherwise the config's `repositories:` list (resolved relative to
// the config file) is used; failing that, the current directory.
func resolveSpecs(v *viper.Viper, repoArgs []string) []gitstats.RepoSpec {
	if len(repoArgs) > 0 {
		return gitstats.ResolveRepos(repoArgs, "")
	}
	repos := v.GetStringSlice("repositories")
	if len(repos) > 0 {
		configDir := ""
		if f := v.ConfigFileUsed(); f != "" {
			configDir = filepath.Dir(f)
		}
		return gitstats.ResolveRepos(repos, configDir)
	}
	return gitstats.ResolveRepos([]string{"."}, "")
}

// isYAMLFile reports whether the argument names a YAML file (by extension), in
// which case it is treated as a config file rather than a repository path.
func isYAMLFile(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
}

// userConfig mirrors an entry of the config file's `users:` array, used to
// aggregate several git author identities (different emails and/or author
// names) under a single display name.
type userConfig struct {
	DisplayName  string   `mapstructure:"display-name"`
	DisplayEmail string   `mapstructure:"display-email"`
	Emails       []string `mapstructure:"emails"`
	Usernames    []string `mapstructure:"usernames"`
}

// parseIdentities reads the `users:` section from the config and converts it
// into gitstats identities. It returns an error only when the section is
// present but malformed.
func parseIdentities(v *viper.Viper) ([]gitstats.Identity, error) {
	if !v.IsSet("users") {
		return nil, nil
	}
	var users []userConfig
	if err := v.UnmarshalKey("users", &users); err != nil {
		return nil, fmt.Errorf("parsing users: %w", err)
	}
	ids := make([]gitstats.Identity, 0, len(users))
	for _, u := range users {
		if strings.TrimSpace(u.DisplayName) == "" {
			return nil, fmt.Errorf("each users entry requires a display-name")
		}
		ids = append(ids, gitstats.Identity{
			DisplayName:  u.DisplayName,
			DisplayEmail: u.DisplayEmail,
			Emails:       u.Emails,
			Usernames:    u.Usernames,
		})
	}
	if err := gitstats.ValidateIdentities(ids); err != nil {
		return nil, fmt.Errorf("parsing users: %w", err)
	}
	return ids, nil
}

// parseWeekStart converts a week-start config/flag value into a time.Weekday.
func parseWeekStart(s string) (time.Weekday, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "monday", "mon", "1":
		return time.Monday, nil
	case "sunday", "sun", "0", "7":
		return time.Sunday, nil
	default:
		return time.Monday, fmt.Errorf("invalid week-start %q (want \"monday\" or \"sunday\")", s)
	}
}

// parseWindow reads the optional time-window and commit-window values from the
// config, validates them (both must be non-negative), and returns a Window. A
// zero Window means no constraint (full history).
func parseWindow(v *viper.Viper) (gitstats.Window, error) {
	months := v.GetInt("time-window")
	if months < 0 {
		return gitstats.Window{}, fmt.Errorf("time-window must be a non-negative number of months (got %d)", months)
	}
	commits := v.GetInt("commit-window")
	if commits < 0 {
		return gitstats.Window{}, fmt.Errorf("commit-window must be a non-negative number of commits (got %d)", commits)
	}
	return gitstats.Window{Months: months, Commits: commits}, nil
}
