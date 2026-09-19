package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/fffelix-huang/tokenfetch/internal/completion"
)

// Every long flag in --help must be offered by every completion script.
func TestCompletionsCoverHelp(t *testing.T) {
	flags := regexp.MustCompile(`--([a-z]+)`).FindAllStringSubmatch(usageText, -1)
	if len(flags) < 10 {
		t.Fatalf("found only %d flags in usageText", len(flags))
	}
	for shell, script := range map[string]string{"bash": completion.Bash, "zsh": completion.Zsh, "fish": completion.Fish} {
		for _, f := range flags {
			name := f[1]
			if !strings.Contains(script, "--"+name) && !strings.Contains(script, "-l "+name) {
				t.Errorf("%s completion missing --%s", shell, name)
			}
		}
	}
}

func TestSetColor(t *testing.T) {
	for _, when := range []string{"auto", "always", "never"} {
		if err := setColor(when); err != nil {
			t.Errorf("setColor(%q): %v", when, err)
		}
	}
	if err := setColor("sometimes"); err == nil {
		t.Error("invalid --color value should fail")
	}
}
