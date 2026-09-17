package pricing

import (
	"math"
	"testing"

	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"claude-opus-5":                                "claude-opus-5",
		"claude-haiku-4-5-20251001":                    "claude-haiku-4-5",
		"claude-opus-4-6[1m]":                          "claude-opus-4-6",
		"claude-sonnet-4-0":                            "claude-sonnet-4",
		"claude-sonnet-4-20250514":                     "claude-sonnet-4",
		"claude-opus-4-5@20251101":                     "claude-opus-4-5",
		"us.anthropic.claude-sonnet-4-5-20250929-v1:0": "claude-sonnet-4-5",
		"anthropic.claude-opus-5":                      "claude-opus-5",
		"claude-3-5-haiku-20241022":                    "claude-3-5-haiku",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCost(t *testing.T) {
	mtok := usage.Tokens{Input: 1e6, Output: 1e6, CacheRead: 1e6, CacheWrite5m: 1e6, CacheWrite1h: 1e6}
	for _, tc := range []struct {
		name, model, speed, geo string
		tokens                  usage.Tokens
		searches                int64
		want                    float64
	}{
		{"opus 5 all categories", "claude-opus-5", "standard", "", mtok, 0, 5 + 25 + 0.5 + 6.25 + 10},
		{"opus 5 fast", "claude-opus-5", "fast", "", mtok, 0, 10 + 50 + 1 + 12.5 + 20},
		{"fable 5.1 cheap cache read", "claude-fable-5-1", "", "", usage.Tokens{CacheRead: 1e6}, 0, 0.25},
		{"us geo premium", "claude-sonnet-5", "", "us", usage.Tokens{Input: 1e6}, 0, 2.2},
		{"no geo premium pre-4.6", "claude-sonnet-4-5", "", "us", usage.Tokens{Input: 1e6}, 0, 3},
		{"fast ignored when unsupported", "claude-opus-4-7", "fast", "", usage.Tokens{Output: 1e6}, 0, 25},
		{"web search", "claude-haiku-4-5", "", "", usage.Tokens{}, 3, 0.03},
	} {
		got, ok := Cost(tc.model, tc.speed, tc.geo, tc.tokens, tc.searches)
		if !ok || math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("%s: got %v (ok=%v), want %v", tc.name, got, ok, tc.want)
		}
	}
	if _, ok := Cost("gpt-5", "", "", usage.Tokens{Input: 1}, 0); ok {
		t.Error("unknown model should not be priced")
	}
}
