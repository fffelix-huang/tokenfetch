package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const manager = "systemd"

// unitName is the systemd unit stem; the full names add .service and .timer.
const unitName = "tokenfetch"

// Supported reports whether this OS has a backend.
func Supported() bool { return true }

// unitDir is the systemd user unit directory, honoring XDG_CONFIG_HOME.
func unitDir() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "systemd", "user"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

func unitPaths() (service, timer string, err error) {
	dir, err := unitDir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, unitName+".service"), filepath.Join(dir, unitName+".timer"), nil
}

func systemctl(args ...string) (string, error) {
	out, err := exec.Command("systemctl", append([]string{"--user"}, args...)...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Install writes both units, then enables and starts the timer.
func Install(cfg Config) (Info, error) {
	svc, timer, err := unitPaths()
	if err != nil {
		return Info{}, err
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return Info{}, fmt.Errorf("systemctl not found: install a cron entry running %q instead", cfg.Exe+" ingest")
	}
	if err := writeFile(svc, renderSystemdService(cfg), 0o755); err != nil {
		return Info{}, fmt.Errorf("write %s: %w", svc, err)
	}
	if err := writeFile(timer, renderSystemdTimer(cfg), 0o755); err != nil {
		return Info{}, fmt.Errorf("write %s: %w", timer, err)
	}
	if out, err := systemctl("daemon-reload"); err != nil {
		return Info{}, fmt.Errorf("systemctl daemon-reload: %w: %s", err, out)
	}
	if out, err := systemctl("enable", "--now", unitName+".timer"); err != nil {
		return Info{}, fmt.Errorf("systemctl enable: %w: %s", err, out)
	}
	// Ingest once now, so install is immediately useful and a broken job shows up.
	if out, err := systemctl("start", unitName+".service"); err != nil {
		return Info{}, fmt.Errorf("systemctl start: %w: %s", err, out)
	}
	return Info{Manager: manager, Interval: cfg.interval(), Files: []string{svc, timer}}, nil
}

// Uninstall stops and disables the timer, then removes both units. The store is
// untouched.
func Uninstall() (removed bool, err error) {
	svc, timer, err := unitPaths()
	if err != nil {
		return false, err
	}
	var found bool
	for _, p := range []string{svc, timer} {
		if _, err := os.Stat(p); err == nil {
			found = true
		}
	}
	if !found {
		return false, nil
	}
	if out, err := systemctl("disable", "--now", unitName+".timer"); err != nil {
		return false, fmt.Errorf("systemctl disable: %w: %s", err, out)
	}
	for _, p := range []string{svc, timer} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return false, err
		}
	}
	if out, err := systemctl("daemon-reload"); err != nil {
		return false, fmt.Errorf("systemctl daemon-reload: %w: %s", err, out)
	}
	return true, nil
}

// Status reads the timer's properties; absent units mean not installed.
func Status() (State, error) {
	svc, timer, err := unitPaths()
	if err != nil {
		return State{}, err
	}
	st := State{Manager: manager, LastExit: -1, Files: []string{svc, timer}}
	data, err := os.ReadFile(timer)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, err
	}
	st.Installed = true
	st.Interval = intervalFromTimer(data)
	if out, err := systemctl("is-active", unitName+".timer"); err == nil {
		st.Active = out == "active"
	} else if out != "" {
		st.Detail = out
	}
	// The service, not the timer: install starts the service directly, which
	// the timer's LastTrigger doesn't count.
	st.LastRun = showTime(unitName+".service", "ExecMainExitTimestamp")
	st.NextRun = showTime(unitName+".timer", "NextElapseUSecRealtime")
	// ExecMainStatus reads 0 before the first run too; only trust it after one.
	if !st.LastRun.IsZero() {
		if out, err := systemctl("show", unitName+".service", "--property=ExecMainStatus", "--value"); err == nil {
			if code, err := strconv.Atoi(strings.TrimSpace(out)); err == nil {
				st.LastExit = code
			}
		}
	}
	return st, nil
}

// showTime reads a systemd timestamp property as "@<unix seconds>"; zero when
// unset or when systemd predates --timestamp=unix (v247).
func showTime(unit, property string) time.Time {
	out, err := systemctl("show", unit, "--property="+property, "--value", "--timestamp=unix")
	if err != nil {
		return time.Time{}
	}
	return parseUnixTimestamp(out)
}

// intervalFromTimer reads OnCalendar back out of a timer unit, "" when it is
// not one of ours.
func intervalFromTimer(data []byte) Interval {
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "OnCalendar="); ok {
			if i, err := ParseInterval(strings.TrimSpace(v)); err == nil {
				return i
			}
		}
	}
	return ""
}
