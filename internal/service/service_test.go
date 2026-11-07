package service

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseInterval(t *testing.T) {
	for _, s := range []string{"hourly", "daily", "weekly"} {
		_, err := ParseInterval(s)
		assert.NoError(t, err, "ParseInterval(%q)", s)
	}
	_, err := ParseInterval("fortnightly")
	assert.Error(t, err, "unknown interval should fail")
}

func TestParseUnixTimestamp(t *testing.T) {
	assert.True(t, parseUnixTimestamp("@1762232400\n").Equal(time.Unix(1762232400, 0)))
	for _, s := range []string{"", "n/a", "@0", "1762232400", "Wed 2025-11-04 05:00:00 UTC"} {
		assert.Zero(t, parseUnixTimestamp(s), "parseUnixTimestamp(%q)", s)
	}
}

func TestIntervalUnit(t *testing.T) {
	for i, want := range map[Interval]string{Hourly: "hour", Daily: "day", Weekly: "week"} {
		assert.Equal(t, want, i.Unit(), "%s.Unit()", i)
	}
}

func TestIntervalSeconds(t *testing.T) {
	for i, want := range map[Interval]int{Hourly: 3600, Daily: 86400, Weekly: 604800} {
		assert.Equal(t, want, i.Seconds(), "%s.Seconds()", i)
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
		assert.Contains(t, got, want)
	}
	assert.NotContains(t, got, "home & brew", "plist left a raw & in a path")
}

// No env set: the dict is omitted rather than written empty.
func TestRenderPlistNoEnv(t *testing.T) {
	got := string(renderPlist(Config{Exe: "/usr/local/bin/tokenfetch"}, "/tmp/log"))
	assert.NotContains(t, got, "EnvironmentVariables", "empty env should be omitted")
	assert.Contains(t, got, "<integer>3600</integer>", "zero Interval should default to hourly")
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
		assert.Contains(t, svc, want, "service unit")
	}
	timer := string(renderSystemdTimer(cfg))
	for _, want := range []string{"OnCalendar=weekly", "Persistent=true", "WantedBy=timers.target"} {
		assert.Contains(t, timer, want, "timer unit")
	}
}

// Env is rendered in a stable order so reinstalling doesn't churn the file.
func TestRenderEnvSorted(t *testing.T) {
	cfg := Config{Exe: "tf", Env: map[string]string{"TOKENFETCH_DATA_DIR": "b", "CLAUDE_CONFIG_DIR": "a"}}
	svc := string(renderSystemdService(cfg))
	assert.Less(t, strings.Index(svc, "CLAUDE_CONFIG_DIR"), strings.Index(svc, "TOKENFETCH_DATA_DIR"), "env not sorted:\n%s", svc)
}
