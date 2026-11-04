package service

import (
	"strings"
	"testing"
	"time"
)

func TestParseInterval(t *testing.T) {
	for _, s := range []string{"hourly", "daily", "weekly"} {
		if _, err := ParseInterval(s); err != nil {
			t.Errorf("ParseInterval(%q): %v", s, err)
		}
	}
	if _, err := ParseInterval("fortnightly"); err == nil {
		t.Error("unknown interval should fail")
	}
}

func TestParseUnixTimestamp(t *testing.T) {
	if got := parseUnixTimestamp("@1762232400\n"); !got.Equal(time.Unix(1762232400, 0)) {
		t.Errorf("parseUnixTimestamp(@1762232400) = %v", got)
	}
	for _, s := range []string{"", "n/a", "@0", "1762232400", "Wed 2025-11-04 05:00:00 UTC"} {
		if got := parseUnixTimestamp(s); !got.IsZero() {
			t.Errorf("parseUnixTimestamp(%q) = %v, want zero", s, got)
		}
	}
}

func TestIntervalUnit(t *testing.T) {
	for i, want := range map[Interval]string{Hourly: "hour", Daily: "day", Weekly: "week"} {
		if got := i.Unit(); got != want {
			t.Errorf("%s.Unit() = %q, want %q", i, got, want)
		}
	}
}

func TestIntervalSeconds(t *testing.T) {
	for i, want := range map[Interval]int{Hourly: 3600, Daily: 86400, Weekly: 604800} {
		if got := i.Seconds(); got != want {
			t.Errorf("%s.Seconds() = %d, want %d", i, got, want)
		}
	}
}

// The plist must carry the schedule, the ingest subcommand and the env the job
// needs; a path with XML metacharacters must not break the file.
func TestRenderPlist(t *testing.T) {
	cfg := Config{
		Exe:      "/opt/home & brew/bin/tokenfetch",
		Interval: Daily,
		Env:      map[string]string{"TOKENFETCH_DATA_DIR": "/tmp/tf", "CLAUDE_CONFIG_DIR": "/tmp/cc"},
	}
	got := string(renderPlist(cfg, "/tmp/tokenfetch.log"))
	for _, want := range []string{
		"<key>Label</key>\n\t<string>" + Label + "</string>",
		"<string>/opt/home &amp; brew/bin/tokenfetch</string>",
		"<string>ingest</string>",
		"<key>StartInterval</key>\n\t<integer>86400</integer>",
		"<key>RunAtLoad</key>",
		"<key>TOKENFETCH_DATA_DIR</key>\n\t\t<string>/tmp/tf</string>",
		"<string>/tmp/tokenfetch.log</string>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plist missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "home & brew") {
		t.Error("plist left a raw & in a path")
	}
}

// No env set: the dict is omitted rather than written empty.
func TestRenderPlistNoEnv(t *testing.T) {
	got := string(renderPlist(Config{Exe: "/usr/local/bin/tokenfetch"}, "/tmp/log"))
	if strings.Contains(got, "EnvironmentVariables") {
		t.Errorf("empty env should be omitted:\n%s", got)
	}
	if !strings.Contains(got, "<integer>3600</integer>") {
		t.Error("zero Interval should default to hourly")
	}
}

func TestRenderSystemdUnits(t *testing.T) {
	cfg := Config{
		Exe:      "/home/a b/bin/tokenfetch",
		Interval: Weekly,
		Env:      map[string]string{"TOKENFETCH_DATA_DIR": "/tmp/tf"},
	}
	svc := string(renderSystemdService(cfg))
	for _, want := range []string{
		`ExecStart="/home/a b/bin/tokenfetch" ingest`,
		`Environment="TOKENFETCH_DATA_DIR=/tmp/tf"`,
		"Type=oneshot",
	} {
		if !strings.Contains(svc, want) {
			t.Errorf("service unit missing %q:\n%s", want, svc)
		}
	}
	timer := string(renderSystemdTimer(cfg))
	for _, want := range []string{"OnCalendar=weekly", "Persistent=true", "WantedBy=timers.target"} {
		if !strings.Contains(timer, want) {
			t.Errorf("timer unit missing %q:\n%s", want, timer)
		}
	}
}

// Env is rendered in a stable order so reinstalling doesn't churn the file.
func TestRenderEnvSorted(t *testing.T) {
	cfg := Config{Exe: "tf", Env: map[string]string{"TOKENFETCH_DATA_DIR": "b", "CLAUDE_CONFIG_DIR": "a"}}
	svc := string(renderSystemdService(cfg))
	if strings.Index(svc, "CLAUDE_CONFIG_DIR") > strings.Index(svc, "TOKENFETCH_DATA_DIR") {
		t.Errorf("env not sorted:\n%s", svc)
	}
}
