// Package service installs a periodic `tokenfetch ingest` job, so usage keeps
// being recorded even when the user doesn't run tokenfetch: Claude Code prunes
// transcripts (cleanupPeriodDays, default 30) and the store is the only
// long-term history.
//
// launchd user agent on macOS, systemd user timer on Linux. Unit rendering
// lives here (pure, testable on any OS); paths and launchctl/systemctl calls
// live in the per-OS files.
package service

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Label identifies the job to launchd; the systemd units are named after it.
const Label = "com.github.fffelix-huang.tokenfetch"

// Interval is how often the job runs. Both schedulers understand these three
// directly, which keeps the rendered units simple.
type Interval string

const (
	Hourly Interval = "hourly"
	Daily  Interval = "daily"
	Weekly Interval = "weekly"

	// DefaultInterval is cheap (ingest is incremental) and keeps the data
	// fresh enough for a menubar client.
	DefaultInterval = Hourly
)

// ParseInterval accepts the values of the --interval flag.
func ParseInterval(s string) (Interval, error) {
	switch Interval(s) {
	case Hourly:
		return Hourly, nil
	case Daily:
		return Daily, nil
	case Weekly:
		return Weekly, nil
	}
	return "", fmt.Errorf("--interval must be hourly, daily or weekly, got %q", s)
}

// Seconds is the launchd StartInterval.
func (i Interval) Seconds() int {
	switch i {
	case Daily:
		return int((24 * time.Hour).Seconds())
	case Weekly:
		return int((7 * 24 * time.Hour).Seconds())
	default:
		return int(time.Hour.Seconds())
	}
}

// Unit names one period, as in "every hour".
func (i Interval) Unit() string {
	switch i {
	case Daily:
		return "day"
	case Weekly:
		return "week"
	default:
		return "hour"
	}
}

// Config describes the job to install.
type Config struct {
	Exe      string            // absolute path to the tokenfetch binary
	Interval Interval          // zero value means DefaultInterval
	Env      map[string]string // passed to the job, see EnvFromOS
}

func (c Config) interval() Interval {
	if c.Interval == "" {
		return DefaultInterval
	}
	return c.Interval
}

// envKeys are the variables a scheduled run needs to see the same data as the
// interactive one; a job inherits almost nothing from the user's shell.
var envKeys = []string{"TOKENFETCH_DATA_DIR", "CLAUDE_CONFIG_DIR"}

// EnvFromOS captures the tokenfetch variables currently set, so a job installed
// from a shell that overrides them keeps reading and writing the same places.
func EnvFromOS() map[string]string {
	env := map[string]string{}
	for _, k := range envKeys {
		if v := os.Getenv(k); v != "" {
			env[k] = v
		}
	}
	return env
}

func sortedEnv(env map[string]string) [][2]string {
	out := make([][2]string, 0, len(env))
	for k, v := range env {
		out = append(out, [2]string{k, v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// Info describes an installed job.
type Info struct {
	Manager  string // "launchd" or "systemd"
	Interval Interval
	Files    []string // unit files written
	LogPath  string   // "" when the manager keeps the logs (journal)
}

// State describes an installed job's current state. Zero times mean the
// manager didn't report one.
type State struct {
	Manager   string
	Installed bool
	Active    bool
	Interval  Interval
	LastRun   time.Time
	NextRun   time.Time
	LastExit  int // exit status of the last run, -1 when unknown
	Files     []string
	LogPath   string
	Detail    string // manager's own words, shown as-is
}

// parseUnixTimestamp reads systemctl's --timestamp=unix form, "@1699999999";
// zero for anything else, including "n/a" and "" (never happened).
func parseUnixTimestamp(s string) time.Time {
	secs, err := strconv.ParseInt(strings.TrimPrefix(strings.TrimSpace(s), "@"), 10, 64)
	if err != nil || secs <= 0 || !strings.HasPrefix(strings.TrimSpace(s), "@") {
		return time.Time{}
	}
	return time.Unix(secs, 0)
}

// renderPlist builds the launchd agent. RunAtLoad makes `service install`
// ingest right away, which also surfaces a broken job immediately.
func renderPlist(cfg Config, logPath string) []byte {
	var b bytes.Buffer
	b.WriteString(xml.Header)
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString("<plist version=\"1.0\">\n<dict>\n")
	fmt.Fprintf(&b, "\t<key>Label</key>\n\t<string>%s</string>\n", esc(Label))
	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	fmt.Fprintf(&b, "\t\t<string>%s</string>\n\t\t<string>ingest</string>\n", esc(cfg.Exe))
	b.WriteString("\t</array>\n")
	fmt.Fprintf(&b, "\t<key>StartInterval</key>\n\t<integer>%d</integer>\n", cfg.interval().Seconds())
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n")
	b.WriteString("\t<key>ProcessType</key>\n\t<string>Background</string>\n")
	b.WriteString("\t<key>LowPriorityIO</key>\n\t<true/>\n")
	fmt.Fprintf(&b, "\t<key>StandardOutPath</key>\n\t<string>%s</string>\n", esc(logPath))
	fmt.Fprintf(&b, "\t<key>StandardErrorPath</key>\n\t<string>%s</string>\n", esc(logPath))
	if env := sortedEnv(cfg.Env); len(env) > 0 {
		b.WriteString("\t<key>EnvironmentVariables</key>\n\t<dict>\n")
		for _, kv := range env {
			fmt.Fprintf(&b, "\t\t<key>%s</key>\n\t\t<string>%s</string>\n", esc(kv[0]), esc(kv[1]))
		}
		b.WriteString("\t</dict>\n")
	}
	b.WriteString("</dict>\n</plist>\n")
	return b.Bytes()
}

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// renderSystemdService builds the oneshot unit the timer triggers. Values are
// quoted: systemd would otherwise split a path on spaces.
func renderSystemdService(cfg Config) []byte {
	var b bytes.Buffer
	b.WriteString("[Unit]\n")
	b.WriteString("Description=Record Claude Code token usage\n")
	b.WriteString("Documentation=https://github.com/fffelix-huang/tokenfetch\n\n")
	b.WriteString("[Service]\n")
	b.WriteString("Type=oneshot\n")
	fmt.Fprintf(&b, "ExecStart=%s ingest\n", quote(cfg.Exe))
	for _, kv := range sortedEnv(cfg.Env) {
		fmt.Fprintf(&b, "Environment=%s\n", quote(kv[0]+"="+kv[1]))
	}
	b.WriteString("Nice=10\n")
	b.WriteString("IOSchedulingClass=idle\n")
	return b.Bytes()
}

// renderSystemdTimer builds the timer. Persistent catches up a run missed
// while the machine was off, which is the whole point of the job.
func renderSystemdTimer(cfg Config) []byte {
	var b bytes.Buffer
	b.WriteString("[Unit]\n")
	b.WriteString("Description=Record Claude Code token usage periodically\n\n")
	b.WriteString("[Timer]\n")
	fmt.Fprintf(&b, "OnCalendar=%s\n", cfg.interval())
	b.WriteString("Persistent=true\n")
	b.WriteString("RandomizedDelaySec=300\n\n")
	b.WriteString("[Install]\n")
	b.WriteString("WantedBy=timers.target\n")
	return b.Bytes()
}

// quote wraps a systemd value in double quotes, escaping what systemd treats
// as special inside them.
func quote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}

// writeFile creates parents and writes a unit file.
func writeFile(path string, data []byte, dirPerm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
