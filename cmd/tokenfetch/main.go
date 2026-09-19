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
	"github.com/fffelix-huang/tokenfetch/internal/source/claudecode"
	"github.com/fffelix-huang/tokenfetch/internal/store"
	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

// Bump version by hand for each release; the release workflow checks it
// matches the tag. Release builds override both via -ldflags -X.
var version = "0.4.0"
var revision = "devel"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "tokenfetch:", err)
		os.Exit(1)
	}
}

const usageText = `Tokenfetch gives summary of Claude Code token usage and estimated API cost.

Usage: tokenfetch [range] [flags]

Range (pick one):
  --all                everything recorded, activity over the past year (default)
  --month              last 30 days
  --week               last 7 days
  --today              since local midnight

Flags:
  --json               print the report as JSON
  --rebuild            re-read all logs; history of already-deleted logs is kept
  --color=WHEN         color output (auto|always|never, default: auto)

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

	path, err := store.Path()
	if err != nil {
		return err
	}
	if os.Getenv(store.DataDirEnv) == "" {
		if legacy, err := store.LegacyPath(); err == nil {
			moved, err := store.MoveLegacy(legacy, path)
			if err != nil {
				return fmt.Errorf("move database from %s: %w", legacy, err)
			}
			if moved {
				fmt.Fprintf(os.Stderr, "tokenfetch: moved database %s -> %s\n", legacy, path)
			}
		}
	}
	st, err := store.Open(path)
	if err != nil {
		return fmt.Errorf("open database %s: %w", path, err)
	}
	defer st.Close()
	if *rebuild {
		if err := st.ForgetFiles(); err != nil {
			return err
		}
	}

	stats, err := ingest.Run(st, []usage.Source{claudecode.New()})
	if err != nil {
		return fmt.Errorf("ingest: %w", err)
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
