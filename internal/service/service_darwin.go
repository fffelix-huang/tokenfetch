package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const manager = "launchd"

// Supported reports whether this OS has a backend.
func Supported() bool { return true }

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), nil
}

// logPath is where launchd sends the job's output; its mtime doubles as the
// last-run time, which launchctl doesn't report.
func logPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Logs", "tokenfetch.log"), nil
}

func serviceTarget() string { return fmt.Sprintf("gui/%d/%s", os.Getuid(), Label) }

func domain() string { return fmt.Sprintf("gui/%d", os.Getuid()) }

// Install writes the agent and loads it, replacing any previous version.
func Install(cfg Config) (Info, error) {
	plist, err := plistPath()
	if err != nil {
		return Info{}, err
	}
	log, err := logPath()
	if err != nil {
		return Info{}, err
	}
	if err := os.MkdirAll(filepath.Dir(log), 0o755); err != nil {
		return Info{}, err
	}
	if err := writeFile(plist, renderPlist(cfg, log), 0o755); err != nil {
		return Info{}, fmt.Errorf("write %s: %w", plist, err)
	}
	// A stale job would keep the old schedule; ignore errors as it may not be loaded.
	_ = exec.Command("launchctl", "bootout", serviceTarget()).Run()
	if out, err := exec.Command("launchctl", "bootstrap", domain(), plist).CombinedOutput(); err != nil {
		// Older macOS only has the legacy subcommands.
		if out2, err2 := exec.Command("launchctl", "load", "-w", plist).CombinedOutput(); err2 != nil {
			return Info{}, fmt.Errorf("launchctl bootstrap: %w: %s", err, strings.TrimSpace(string(out)+" "+string(out2)))
		}
	}
	return Info{Manager: manager, Interval: cfg.interval(), Files: []string{plist}, LogPath: log}, nil
}

// Uninstall unloads the agent and removes the plist. The store is untouched.
func Uninstall() (removed bool, err error) {
	plist, err := plistPath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(plist); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if out, err := exec.Command("launchctl", "bootout", serviceTarget()).CombinedOutput(); err != nil {
		if out2, err2 := exec.Command("launchctl", "unload", "-w", plist).CombinedOutput(); err2 != nil {
			return false, fmt.Errorf("launchctl bootout: %w: %s", err, strings.TrimSpace(string(out)+" "+string(out2)))
		}
	}
	if err := os.Remove(plist); err != nil {
		return false, err
	}
	return true, nil
}

var (
	plistInterval = regexp.MustCompile(`<key>StartInterval</key>\s*<integer>(\d+)</integer>`)
	lastExitRe    = regexp.MustCompile(`last exit (?:code|status) = (-?\d+)`)
	stateRe       = regexp.MustCompile(`state = (\w+)`)
)

// Status asks launchctl about the job; absent plist means not installed.
func Status() (State, error) {
	plist, err := plistPath()
	if err != nil {
		return State{}, err
	}
	log, err := logPath()
	if err != nil {
		return State{}, err
	}
	st := State{Manager: manager, LastExit: -1, Files: []string{plist}, LogPath: log}
	data, err := os.ReadFile(plist)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, err
	}
	st.Installed = true
	if m := plistInterval.FindSubmatch(data); m != nil {
		if secs, err := strconv.Atoi(string(m[1])); err == nil {
			st.Interval = intervalFromSeconds(secs)
		}
	}
	if fi, err := os.Stat(log); err == nil && fi.Size() > 0 {
		st.LastRun = fi.ModTime()
		if !st.LastRun.IsZero() && st.Interval != "" {
			st.NextRun = st.LastRun.Add(time.Duration(st.Interval.Seconds()) * time.Second)
		}
	}
	out, err := exec.Command("launchctl", "print", serviceTarget()).CombinedOutput()
	if err != nil {
		// Loaded jobs always print; an error means launchd doesn't know it.
		st.Detail = "not loaded (run `tokenfetch service install`)"
		return st, nil
	}
	st.Active = true
	text := string(out)
	if m := lastExitRe.FindStringSubmatch(text); m != nil {
		if code, err := strconv.Atoi(m[1]); err == nil {
			st.LastExit = code
		}
	}
	if m := stateRe.FindStringSubmatch(text); m != nil {
		st.Detail = "state " + m[1]
	}
	return st, nil
}

// intervalFromSeconds names a StartInterval, "" when it matches none.
func intervalFromSeconds(secs int) Interval {
	for _, i := range []Interval{Hourly, Daily, Weekly} {
		if i.Seconds() == secs {
			return i
		}
	}
	return ""
}
