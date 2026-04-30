package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/RileighLuvsCatz/retrace/db"
	"github.com/RileighLuvsCatz/retrace/internal/daemon"
	"github.com/spf13/cobra"
)

// Execute is the CLI entrypoint.
func Execute() int {
	root := NewRootCommand(os.Stdout, os.Stderr)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(root.ErrOrStderr(), err)
		return 1
	}

	return 0
}

// NewRootCommand builds the ReTrace Cobra command tree.
func NewRootCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	options := cliOptions{
		dbPath:       daemon.DefaultDBPath,
		pidPath:      daemon.DefaultPIDPath,
		logPath:      daemon.DefaultLogPath,
		pollInterval: daemon.DefaultPollInterval,
		now:          time.Now,
	}

	root := &cobra.Command{
		Use:   "retrace",
		Short: "Track and inspect app usage sessions",
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SilenceUsage = true
	root.SilenceErrors = true
	root.PersistentFlags().StringVar(&options.dbPath, "db", options.dbPath, "SQLite database path")

	root.AddCommand(newDaemonCommand(&options))
	root.AddCommand(newTodayCommand(&options))
	root.AddCommand(newWeekCommand(&options))
	root.AddCommand(newAppCommand(&options))
	root.AddCommand(newStatsCommand(&options))

	return root
}

type cliOptions struct {
	dbPath       string
	pidPath      string
	logPath      string
	pollInterval time.Duration
	now          func() time.Time
}

func (o cliOptions) daemonConfig() daemon.Config {
	return daemon.Config{
		DBPath:       o.dbPath,
		PIDPath:      o.pidPath,
		LogPath:      o.logPath,
		PollInterval: o.pollInterval,
	}
}

func newDaemonCommand(options *cliOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the background usage collector",
	}
	command.PersistentFlags().StringVar(&options.pidPath, "pid-file", options.pidPath, "daemon PID file path")
	command.PersistentFlags().StringVar(&options.logPath, "log-file", options.logPath, "daemon log file path")
	command.PersistentFlags().DurationVar(&options.pollInterval, "poll-interval", options.pollInterval, "active window polling interval")

	command.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start the background collector",
		RunE: func(cmd *cobra.Command, _ []string) error {
			config := options.daemonConfig()
			if err := daemon.Start(config); err != nil {
				return fmt.Errorf("daemon start failed: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "daemon started; pid file: %s\n", config.PIDPath)
			return nil
		},
	})

	command.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop the background collector",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := daemon.Stop(options.daemonConfig()); err != nil {
				return fmt.Errorf("daemon stop failed: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "daemon stopped")
			return nil
		},
	})

	command.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show collector daemon status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pid, running, err := daemon.Status(options.daemonConfig())
			if err != nil {
				return fmt.Errorf("daemon status failed: %w", err)
			}
			if running {
				fmt.Fprintf(cmd.OutOrStdout(), "daemon running with pid %d\n", pid)
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "daemon stopped")
			return nil
		},
	})

	command.AddCommand(&cobra.Command{
		Use:    "run",
		Short:  "Run the collector in the foreground",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := daemon.RunForeground(context.Background(), options.daemonConfig()); err != nil {
				return fmt.Errorf("daemon run failed: %w", err)
			}

			return nil
		},
	})

	return command
}

func newTodayCommand(options *cliOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "today",
		Short: "Show today's app usage",
		RunE: func(cmd *cobra.Command, _ []string) error {
			now := options.now()
			start := startOfDay(now)
			end := start.AddDate(0, 0, 1)

			store, err := openCLIStore(cmd.Context(), options.dbPath)
			if err != nil {
				return err
			}
			defer store.Close()

			usage, err := store.AppUsageBetween(cmd.Context(), start, end)
			if err != nil {
				return err
			}

			printAppUsage(cmd.OutOrStdout(), fmt.Sprintf("Today (%s)", start.Format(time.DateOnly)), usage)
			return nil
		},
	}
}

func newWeekCommand(options *cliOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "week",
		Short: "Show this week's usage summary",
		RunE: func(cmd *cobra.Command, _ []string) error {
			now := options.now()
			start := startOfWeek(now)
			end := start.AddDate(0, 0, 7)

			store, err := openCLIStore(cmd.Context(), options.dbPath)
			if err != nil {
				return err
			}
			defer store.Close()

			daily, err := store.DailyUsageBetween(cmd.Context(), start, end)
			if err != nil {
				return err
			}
			apps, err := store.AppUsageBetween(cmd.Context(), start, end)
			if err != nil {
				return err
			}

			printDailyUsage(cmd.OutOrStdout(), fmt.Sprintf("Week of %s", start.Format(time.DateOnly)), daily)
			fmt.Fprintln(cmd.OutOrStdout())
			printAppUsage(cmd.OutOrStdout(), "Top apps", apps)
			return nil
		},
	}
}

func newAppCommand(options *cliOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "app <name>",
		Short: "Show stats for a specific app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openCLIStore(cmd.Context(), options.dbPath)
			if err != nil {
				return err
			}
			defer store.Close()

			usage, sessions, err := store.AppUsageByName(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s\n", usage.App)
			if len(sessions) == 0 {
				fmt.Fprintln(out, "No recorded sessions.")
				return nil
			}

			fmt.Fprintf(out, "Total: %s across %d sessions\n", formatDuration(usage.Seconds), usage.Sessions)
			fmt.Fprintln(out, "Recent sessions:")
			limit := min(len(sessions), 10)
			for _, session := range sessions[:limit] {
				fmt.Fprintf(
					out,
					"  %s  %s  %s\n",
					session.StartedAt.Format("2006-01-02 15:04"),
					formatDuration(session.Duration),
					session.Title,
				)
			}
			return nil
		},
	}
}

func newStatsCommand(options *cliOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show overall usage stats",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := openCLIStore(cmd.Context(), options.dbPath)
			if err != nil {
				return err
			}
			defer store.Close()

			total, sessions, err := store.TotalUsage(cmd.Context())
			if err != nil {
				return err
			}
			apps, err := store.AppUsageBetween(cmd.Context(), time.Time{}, options.now().AddDate(100, 0, 0))
			if err != nil {
				return err
			}
			hour, ok, err := store.MostActiveHour(cmd.Context())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Total screen time: %s\n", formatDuration(total))
			fmt.Fprintf(out, "Recorded sessions: %d\n", sessions)
			if ok {
				fmt.Fprintf(out, "Most active hour: %02d:00 (%s)\n", hour.Hour, formatDuration(hour.Seconds))
			} else {
				fmt.Fprintln(out, "Most active hour: none")
			}
			fmt.Fprintln(out)
			printAppUsage(out, "Top apps", apps)
			return nil
		},
	}
}

func openCLIStore(ctx context.Context, path string) (*db.Store, error) {
	store, err := db.Open(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return store, nil
}

func printAppUsage(out io.Writer, title string, usage []db.AppUsage) {
	fmt.Fprintln(out, title)
	if len(usage) == 0 {
		fmt.Fprintln(out, "No recorded sessions.")
		return
	}

	total := int64(0)
	for _, row := range usage {
		total += row.Seconds
	}

	for _, row := range usage {
		share := 0.0
		if total > 0 {
			share = float64(row.Seconds) / float64(total) * 100
		}
		fmt.Fprintf(out, "  %-24s %10s  %5.1f%%  %d sessions\n", truncate(row.App, 24), formatDuration(row.Seconds), share, row.Sessions)
	}
}

func printDailyUsage(out io.Writer, title string, usage []db.DailyUsage) {
	fmt.Fprintln(out, title)
	if len(usage) == 0 {
		fmt.Fprintln(out, "No recorded sessions.")
		return
	}

	for _, row := range usage {
		fmt.Fprintf(out, "  %s  %10s  %d sessions\n", row.Day.Format("Mon 2006-01-02"), formatDuration(row.Seconds), row.Sessions)
	}
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func startOfWeek(t time.Time) time.Time {
	day := startOfDay(t)
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

func formatDuration(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}

	duration := time.Duration(seconds) * time.Second
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	secs := int(duration.Seconds()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %02dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %02ds", minutes, secs)
	}

	return fmt.Sprintf("%ds", secs)
}

func truncate(value string, width int) string {
	value = strings.TrimSpace(value)
	if len(value) <= width {
		return value
	}
	if width <= 3 {
		return value[:width]
	}

	return value[:width-3] + "..."
}
