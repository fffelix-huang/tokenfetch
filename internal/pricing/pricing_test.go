package pricing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		assert.Equal(t, want, Normalize(in), "Normalize(%q)", in)
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
		{"opus 5.5 all categories", "claude-opus-5-5", "standard", "", mtok, 0, 4 + 20 + 0.2 + 5 + 8},
		{"opus 5.5 fast", "claude-opus-5-5", "fast", "", mtok, 0, 8 + 40 + 0.4 + 10 + 16},
		{"sonnet 5.5 cache read 0.05x", "claude-sonnet-5-5", "", "", usage.Tokens{CacheRead: 1e6}, 0, 0.1},
		{"haiku 5.5 short prompt rates", "claude-haiku-5-5", "", "", usage.Tokens{Input: 1e6, Output: 1e6}, 0, 0.1 + 0.5},
		{"opus 5 all categories", "claude-opus-5", "standard", "", mtok, 0, 5 + 25 + 0.5 + 6.25 + 10},
		{"opus 5 fast", "claude-opus-5", "fast", "", mtok, 0, 10 + 50 + 1 + 12.5 + 20},
		{"fable 5.1 cheap cache read", "claude-fable-5-1", "", "", usage.Tokens{CacheRead: 1e6}, 0, 0.25},
		{"us geo premium", "claude-sonnet-5", "", "us", usage.Tokens{Input: 1e6}, 0, 2.2},
		{"no geo premium pre-4.6", "claude-sonnet-4-5", "", "us", usage.Tokens{Input: 1e6}, 0, 3},
		{"fast ignored when unsupported", "claude-opus-4-7", "fast", "", usage.Tokens{Output: 1e6}, 0, 25},
		{"web search", "claude-haiku-4-5", "", "", usage.Tokens{}, 3, 0.03},
	} {
		got, ok := Cost(tc.model, tc.speed, tc.geo, 0, tc.tokens, tc.searches)
		if assert.True(t, ok, tc.name) {
			assert.InDelta(t, tc.want, got, 1e-9, tc.name)
		}
	}
	_, ok := Cost("llama-4-scout", "", "", 0, usage.Tokens{Input: 1}, 0)
	assert.False(t, ok, "unknown model should not be priced")
}

// Haiku 5.5 prompts over 100k tokens pay 5x on every category; geo stacks.
func TestCostLongPrompt(t *testing.T) {
	tok := usage.Tokens{Input: 1e6, Output: 1e6, CacheRead: 1e6, CacheWrite5m: 1e6, CacheWrite1h: 1e6}
	short, _ := Cost("claude-haiku-5-5", "", "", 0, tok, 0)
	long, _ := Cost("claude-haiku-5-5", "", "", 100_000, tok, 0)
	assert.InDelta(t, 0.1+0.5+0.01+0.125+0.2, short, 1e-9)
	assert.InDelta(t, 0.5+2.5+0.05+0.625+1, long, 1e-9)
	longUS, _ := Cost("claude-haiku-5-5", "", "us", 100_000, tok, 0)
	assert.InDelta(t, long*1.1, longUS, 1e-9)
	// Flat-priced models ignore the flag.
	a, _ := Cost("claude-opus-5-5", "", "", 0, tok, 0)
	b, _ := Cost("claude-opus-5-5", "", "", 100_000, tok, 0)
	assert.Equal(t, a, b)
}

// gpt-6-luna: Standard/Flex/Fast tiers, each with >272K long-context rates.
func TestCostGPT6Luna(t *testing.T) {
	tok := usage.Tokens{Input: 1e6, Output: 1e6, CacheRead: 1e6, CacheWrite5m: 1e6, CacheWrite1h: 1e6}
	for _, tc := range []struct {
		name       string
		speed      string
		promptOver int64
		want       float64
	}{
		{"standard", "", 0, 0.10 + 0.50 + 0.01 + 0.125 + 0.25},
		{"over haiku threshold only: still short", "", 100_000, 0.10 + 0.50 + 0.01 + 0.125 + 0.25},
		{"long context", "", 272_000, 0.20 + 0.75 + 0.02 + 0.25 + 0.50},
		{"flex", "flex", 0, 0.05 + 0.25 + 0.005 + 0.0625 + 0.125},
		{"flex long", "flex", 272_000, 0.10 + 0.375 + 0.01 + 0.125 + 0.25},
		{"fast", "fast", 0, 0.20 + 1.00 + 0.02 + 0.25 + 0.50},
		{"fast long", "fast", 272_000, 0.40 + 1.50 + 0.04 + 0.50 + 1.00},
	} {
		got, ok := Cost("gpt-6-luna", tc.speed, "", tc.promptOver, tok, 0)
		if assert.True(t, ok, tc.name) {
			assert.InDelta(t, tc.want, got, 1e-9, tc.name)
		}
	}
	// Haiku 5.5 is long above 100k, so a 272K bucket is long for it too.
	a, _ := Cost("claude-haiku-5-5", "", "", 100_000, tok, 0)
	b, _ := Cost("claude-haiku-5-5", "", "", 272_000, tok, 0)
	assert.Equal(t, a, b)
	assert.Equal(t, []int64{100_000, 272_000}, PromptThresholds())
	assert.Equal(t, "gpt-6-luna", Normalize("gpt-6-luna-2026-10-01"))
}

// Spot checks against the OpenAI pricing page (2026-11-08).
func TestOpenAITableSpotChecks(t *testing.T) {
	r := func(model string) Rates {
		t.Helper()
		v, ok := Lookup(model)
		require.True(t, ok, model)
		return v
	}
	codex := r("gpt-5.3-codex")
	assert.Equal(t, Rates{Input: 1.75, CacheRead: 0.175, CacheWrite5m: 1.75, CacheWrite1h: 1.75, Output: 14,
		Fast: &Rates{Input: 3.5, CacheRead: 0.35, CacheWrite5m: 3.5, CacheWrite1h: 3.5, Output: 28}}, codex)
	astra := r("gpt-6-astra")
	require.NotNil(t, astra.Ultrafast)
	require.NotNil(t, astra.Ultrafast.LongPrompt)
	assert.Equal(t, 120.0, astra.Ultrafast.LongPrompt.Input)
	assert.Equal(t, 450.0, astra.Ultrafast.LongPrompt.Output)
	pro := r("gpt-5.5-pro") // no prompt caching: cached reads at full input price
	assert.Equal(t, pro.Input, pro.CacheRead)
	old := r("gpt-5") // no cache-write price: writes billed as input
	assert.Equal(t, old.Input, old.CacheWrite5m)
	assert.Equal(t, old.Input, old.CacheWrite1h)
	require.NotNil(t, r("o3").Flex)
	assert.Equal(t, 0.25, r("o3").Flex.CacheRead)
	_, ok := Lookup("text-embedding-3-small")
	assert.False(t, ok, "embeddings are not chat models")
}

// Invariants of the generated table: 12h write = 2.5x input wherever a write
// price exists, long context only above 272K, keys already normalized.
func TestOpenAITableInvariants(t *testing.T) {
	var check func(name string, r *Rates)
	check = func(name string, r *Rates) {
		if r == nil {
			return
		}
		if r.CacheWrite5m != r.Input {
			assert.InDelta(t, r.Input*2.5, r.CacheWrite1h, 1e-9, "%s: 12h write", name)
			assert.InDelta(t, r.Input*1.25, r.CacheWrite5m, 1e-9, "%s: 30m write", name)
		}
		check(name+"/long", r.LongPrompt)
		check(name+"/flex", r.Flex)
		check(name+"/fast", r.Fast)
		check(name+"/ultrafast", r.Ultrafast)
	}
	require.GreaterOrEqual(t, len(openAI), 37)
	for name, r := range openAI {
		assert.Equal(t, name, Normalize(name), "key must be normalized")
		if r.LongPrompt != nil {
			assert.Equal(t, int64(272_000), r.LongPromptAbove, name)
		}
		check(name, &r)
	}
	assert.Equal(t, []int64{100_000, 272_000}, PromptThresholds())
}

func TestCostUltrafast(t *testing.T) {
	tok := usage.Tokens{Input: 1e6, Output: 1e6}
	got, ok := Cost("gpt-6.1-sol", "ultrafast", "", 0, tok, 0)
	require.True(t, ok)
	assert.InDelta(t, 12.0+60, got, 1e-9)
	// Models without an Ultrafast tier fall back to Standard.
	got, _ = Cost("gpt-6-luna", "ultrafast", "", 0, tok, 0)
	assert.InDelta(t, 0.10+0.50, got, 1e-9)
}
