package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fffelix-huang/tokenfetch/internal/completion"
)

// Every long flag in --help must be offered by every completion script.
func TestCompletionsCoverHelp(t *testing.T) {
	flags := regexp.MustCompile(`--([a-z]+)`).FindAllStringSubmatch(usageText, -1)
	require.GreaterOrEqual(t, len(flags), 10, "too few flags in usageText")
	for shell, script := range map[string]string{"bash": completion.Bash, "zsh": completion.Zsh, "fish": completion.Fish} {
		for _, f := range flags {
			name := f[1]
			assert.True(t, strings.Contains(script, "--"+name) || strings.Contains(script, "-l "+name),
				"%s completion missing --%s", shell, name)
		}
	}
}

// Every command in --help must be offered by every completion script.
func TestCompletionsCoverCommands(t *testing.T) {
	for _, cmd := range []string{"ingest", "service", "install", "uninstall", "status", "hourly", "daily", "weekly"} {
		require.Contains(t, usageText+serviceUsage, cmd, "not in help; update this test")
		for shell, script := range map[string]string{"bash": completion.Bash, "zsh": completion.Zsh, "fish": completion.Fish} {
			assert.Contains(t, script, cmd, "%s completion missing %s", shell, cmd)
		}
	}
}

func TestSetColor(t *testing.T) {
	for _, when := range []string{"auto", "always", "never"} {
		assert.NoError(t, setColor(when), "setColor(%q)", when)
	}
	assert.Error(t, setColor("sometimes"), "invalid --color value should fail")
}

// go run builds under the temp dir; on macOS that path resolves through the
// /var -> /private/var symlink, so the check must compare resolved paths.
func TestUnderTempDir(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	assert.True(t, underTempDir(filepath.Join(dir, "exe", "tokenfetch")), "resolved temp path")
	assert.True(t, underTempDir(filepath.Join(os.TempDir(), "go-build1", "tokenfetch")), "unresolved temp path")
	assert.False(t, underTempDir("/usr/local/bin/tokenfetch"))
}
