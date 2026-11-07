package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"

	"github.com/fffelix-huang/tokenfetch/internal/completion"
	"github.com/fffelix-huang/tokenfetch/internal/ingest"
	"github.com/fffelix-huang/tokenfetch/internal/render"
	"github.com/fffelix-huang/tokenfetch/internal/report"
	"github.com/fffelix-huang/tokenfetch/internal/service"
	"github.com/fffelix-huang/tokenfetch/internal/source/claudecode"
	"github.com/fffelix-huang/tokenfetch/internal/store"
	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

// Bump version by hand for each release; the release workflow checks it
// matches the tag. Release builds override both via -ldflags -X.
var version = "0.6.0"
var revision = "devel"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "tokenfetch:", err)
		os.Exit(1)
	}
}

const usageText = `Tokenfetch gives summary of Claude Code token usage and estimated API cost.

Usage: tokenfetch [range] [flags]
       tokenfetch <command> [flags]

Commands:
  ingest               record new usage without printing a report
  service install      run ingest periodically, so usage is recorded even
                       when tokenfetch isn't (Claude Code deletes old logs)
  service uninstall    remove the periodic job; recorded usage is kept
  service status       show whether the periodic job is installed and healthy

Range (pick one):
  --all                everything recorded, activity over the past year (default)
  --month              last 30 days
  --week               last 7 days
  --today              since local midnight

Flags:
  --json               print the report as JSON
  --rebuild            re-read all logs; history of already-deleted logs is kept
  --color=WHEN         color output (auto|always|never, default: auto)
  --interval=EVERY     how often "service install" runs ingest
                       (hourly|daily|weekly, default: hourly)

Shell Integration:
  --bash               print bash completion script
  --zsh                print zsh completion script
  --fish               print fish completion script

Help:
  -v, --version        print version
  -h, --help           show this help

Environment Variables:
  TOKENFETCH_DATA_DIR  where usage history is stored
                       (default: %s)
  CLAUDE_CONFIG_DIR    Claude Code config dirs to read, comma-separated
                       (default: ~/.claude and ~/.config/claude)
  NO_COLOR             disable colors when set (unless --color=always)
`

func run() error {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "ingest":
			return runIngest(os.Args[2:])
		case "service":
			return runService(os.Args[2:])
		}
	}
	var (
		today        = flag.Bool("today", false, "")
		week         = flag.Bool("week", false, "")
		month        = flag.Bool("month", false, "")
		all          = flag.Bool("all", false, "")
		asJSON       = flag.Bool("json", false, "")
		rebuild      = flag.Bool("rebuild", false, "")
		showVer      = flag.Bool("version", false, "")
		showVerShort = flag.Bool("v", false, "")
		color        = flag.String("color", "auto", "")
		bash         = flag.Bool("bash", false, "")
		zsh          = flag.Bool("zsh", false, "")
		fish         = flag.Bool("fish", false, "")
	)
	flag.Usage = func() {
		dir, err := store.DefaultDataDir()
		if err != nil {
			dir = "unavailable: " + err.Error()
		}
		if home, err := os.UserHomeDir(); err == nil {
			if rel, err := filepath.Rel(home, dir); err == nil && !strings.HasPrefix(rel, "..") {
				dir = filepath.Join("~", rel)
			}
		}
		fmt.Fprintf(flag.CommandLine.Output(), usageText, dir)
	}
	flag.Parse()
	if *showVer || *showVerShort {
		if len(revision) > 0 {
			fmt.Printf("%s (%s)\n", version, revision)
		} else {
			fmt.Println(version)
		}
		return nil
	}
	if script, err := completionScript(*bash, *zsh, *fish); err != nil || script != "" {
		if err == nil {
			fmt.Print(script)
		}
		return err
	}
	if err := setColor(*color); err != nil {
		return err
	}

	rangeName := "all"
	n := 0
	for name, set := range map[string]bool{"today": *today, "week": *week, "month": *month, "all": *all} {
		if set {
			rangeName = name
			n++
		}
	}
	if n > 1 {
		return fmt.Errorf("pick one of --today, --week, --month, --all")
	}

	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()
	if *rebuild {
		if err := st.ForgetFiles(); err != nil {
			return err
		}
	}

	// A failed ingest (e.g. the periodic job holding the write lock) only
	// means newest usage is missing; the stored history still reports.
	stats, err := ingestAll(st)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tokenfetch: %v; showing recorded usage only\n", err)
	}
	if stats.BadLines > 0 {
		fmt.Fprintf(os.Stderr, "tokenfetch: skipped %d malformed log lines\n", stats.BadLines)
	}

	now := time.Now()
	rng, _ := report.NewRange(rangeName, now)
	rep, err := report.Build(st, rng, now.Location())
	if err != nil {
		return err
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	width, _, _ := term.GetSize(int(os.Stdout.Fd()))
	render.Fetch(os.Stdout, rep, width)
	return nil
}

// openStore opens the usage database, moving the pre-data-dir one first.
func openStore() (*store.Store, error) {
	path, err := store.Path()
	if err != nil {
		return nil, err
	}
	if os.Getenv(store.DataDirEnv) == "" {
		if legacy, err := store.LegacyPath(); err == nil {
			moved, err := store.MoveLegacy(legacy, path)
			if err != nil {
				return nil, fmt.Errorf("move database from %s: %w", legacy, err)
			}
			if moved {
				fmt.Fprintf(os.Stderr, "tokenfetch: moved database %s -> %s\n", legacy, path)
			}
		}
	}
	st, err := store.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	return st, nil
}

func ingestAll(st *store.Store) (ingest.Stats, error) {
	stats, err := ingest.Run(st, []usage.Source{claudecode.New()})
	if err != nil {
		return stats, fmt.Errorf("ingest: %w", err)
	}
	return stats, nil
}

const ingestUsage = `Usage: tokenfetch ingest

Record new usage in the database and print one line about what was read.
Meant for the periodic job installed by ` + "`tokenfetch service install`" + `.
`

// runIngest records usage without building a report; this is what the
// scheduled job runs, and its output is the job's log.
func runIngest(args []string) error {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(fs.Output(), ingestUsage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("ingest takes no arguments, got %q", fs.Arg(0))
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()
	stats, err := ingestAll(st)
	if err != nil {
		return err
	}
	// Timestamped: launchd and the journal both keep appending to one log.
	fmt.Printf("%s ingested %d events from %d %s", time.Now().Format(time.RFC3339), stats.Events, stats.FilesScanned, plural(stats.FilesScanned, "file"))
	if stats.BadLines > 0 {
		fmt.Printf(", skipped %d malformed lines", stats.BadLines)
	}
	fmt.Println()
	return nil
}

const serviceUsage = `Usage: tokenfetch service <install|uninstall|status> [flags]

Keeps usage recorded even when you don't run tokenfetch: Claude Code deletes
transcripts after a while (cleanupPeriodDays, 30 days by default), so usage
that was never ingested is gone for good.

Commands:
  install              install the periodic job and ingest once now
  uninstall            remove the job; recorded usage is kept
  status               show whether the job is installed and healthy

Flags:
  --interval=EVERY     how often to run (hourly|daily|weekly, default: hourly)

The job runs ` + "`tokenfetch ingest`" + ` as a launchd user agent on macOS, or a
systemd user timer on Linux, and inherits TOKENFETCH_DATA_DIR and
CLAUDE_CONFIG_DIR as currently set.
`

// runService dispatches the service subcommands.
func runService(args []string) error {
	fs := flag.NewFlagSet("service", flag.ExitOnError)
	interval := fs.String("interval", string(service.DefaultInterval), "")
	fs.Usage = func() { fmt.Fprint(fs.Output(), serviceUsage) }
	// flag stops at the first non-flag, so parse again after the command to
	// accept both `service --interval=daily install` and `service install --interval=daily`.
	var cmd string
	for rest := args; ; rest = fs.Args()[1:] {
		if err := fs.Parse(rest); err != nil {
			return err
		}
		if fs.NArg() == 0 {
			break
		}
		if cmd != "" {
			return fmt.Errorf("unexpected argument %q after %s", fs.Arg(0), cmd)
		}
		cmd = fs.Arg(0)
	}
	switch cmd {
	case "install":
		every, err := service.ParseInterval(*interval)
		if err != nil {
			return err
		}
		return serviceInstall(every)
	case "uninstall":
		return serviceUninstall()
	case "status":
		return serviceStatus()
	case "":
		fs.Usage()
		return fmt.Errorf("service needs install, uninstall or status")
	default:
		return fmt.Errorf("unknown service command %q, want install, uninstall or status", cmd)
	}
}

func serviceInstall(every service.Interval) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find own path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	// `go run` builds into a temp dir that is deleted right after, which would
	// install a job pointing at a binary that no longer exists.
	if underTempDir(exe) {
		return fmt.Errorf("refusing to schedule %s: install tokenfetch first (make build, go install or brew install)", exe)
	}
	info, err := service.Install(service.Config{Exe: exe, Interval: every, Env: service.EnvFromOS()})
	if err != nil {
		return err
	}
	fmt.Printf("installed %s job %s\n", info.Manager, service.Label)
	fmt.Printf("  runs:  %s (%s ingest)\n", info.Interval, exe)
	for _, f := range info.Files {
		fmt.Printf("  unit:  %s\n", tildeHome(f))
	}
	if info.LogPath != "" {
		fmt.Printf("  log:   %s\n", tildeHome(info.LogPath))
	} else {
		fmt.Printf("  log:   journalctl --user -u tokenfetch\n")
	}
	return nil
}

// underTempDir reports whether path is inside os.TempDir, comparing both
// with symlinks resolved: on macOS $TMPDIR is /var/folders/… while the
// resolved executable is /private/var/folders/….
func underTempDir(path string) bool {
	tmps := []string{os.TempDir()}
	if resolved, err := filepath.EvalSymlinks(tmps[0]); err == nil {
		tmps = append(tmps, resolved)
	}
	for _, tmp := range tmps {
		rel, err := filepath.Rel(tmp, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func serviceUninstall() error {
	removed, err := service.Uninstall()
	if err != nil {
		return err
	}
	if !removed {
		fmt.Println("no periodic job installed")
		return nil
	}
	fmt.Println("removed the periodic job; recorded usage is kept")
	return nil
}

func serviceStatus() error {
	st, err := service.Status()
	if err != nil {
		return err
	}
	if !st.Installed {
		fmt.Printf("not installed (%s); run `tokenfetch service install`\n", st.Manager)
		return nil
	}
	state := "installed but not loaded"
	if st.Active {
		state = "active"
	}
	fmt.Printf("%s · %s", state, st.Manager)
	if st.Interval != "" {
		fmt.Printf(" · every %s", st.Interval.Unit())
	}
	fmt.Println()
	if !st.LastRun.IsZero() {
		fmt.Printf("  last run:  %s\n", st.LastRun.Local().Format("2006-01-02 15:04"))
	}
	if !st.NextRun.IsZero() {
		fmt.Printf("  next run:  %s\n", st.NextRun.Local().Format("2006-01-02 15:04"))
	}
	if st.LastExit >= 0 {
		result := "ok"
		if st.LastExit != 0 {
			result = fmt.Sprintf("failed (exit %d)", st.LastExit)
		}
		fmt.Printf("  last exit: %s\n", result)
	}
	if st.Detail != "" {
		fmt.Printf("  detail:    %s\n", st.Detail)
	}
	for _, f := range st.Files {
		fmt.Printf("  unit:      %s\n", tildeHome(f))
	}
	if st.LogPath != "" {
		fmt.Printf("  log:       %s\n", tildeHome(st.LogPath))
	}
	return nil
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// tildeHome shortens a path under the home directory for display.
func tildeHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return filepath.Join("~", rel)
}

// completionScript returns the script for the one shell flag set, "" if none.
func completionScript(bash, zsh, fish bool) (string, error) {
	var scripts []string
	for _, c := range []struct {
		set    bool
		script string
	}{{bash, completion.Bash}, {zsh, completion.Zsh}, {fish, completion.Fish}} {
		if c.set {
			scripts = append(scripts, c.script)
		}
	}
	if len(scripts) > 1 {
		return "", fmt.Errorf("pick one of --bash, --zsh, --fish")
	}
	if len(scripts) == 0 {
		return "", nil
	}
	return scripts[0], nil
}

// setColor applies --color. auto keeps terminal detection and honors
// NO_COLOR; always forces color even when piped.
func setColor(when string) error {
	switch when {
	case "auto":
		if os.Getenv("NO_COLOR") != "" {
			lipgloss.SetColorProfile(termenv.Ascii)
		}
	case "never":
		lipgloss.SetColorProfile(termenv.Ascii)
	case "always":
		profile := termenv.ANSI256
		if ct := os.Getenv("COLORTERM"); ct == "truecolor" || ct == "24bit" {
			profile = termenv.TrueColor
		}
		lipgloss.SetColorProfile(profile)
	default:
		return fmt.Errorf("--color must be auto, always or never, got %q", when)
	}
	return nil
}
