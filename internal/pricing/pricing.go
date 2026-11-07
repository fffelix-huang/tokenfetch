// Package pricing estimates USD cost from token counts using list prices.
//
// Sources: https://platform.claude.com/docs/en/about-claude/pricing (fetched 2026-11-08),
// https://developers.openai.com/api/docs/pricing (fetched 2026-11-08).
// Subscription users (Claude Pro/Max, ChatGPT plans) are not billed per token;
// this is the API-equivalent cost.
package pricing

import (
	"regexp"
	"slices"
	"strings"

	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

// Rates are USD per million tokens. CacheWrite5m/CacheWrite1h are the short
// and long cache-write lifetimes: Anthropic 5m/1h, OpenAI 30m/12h.
type Rates struct {
	Input, Output, CacheWrite5m, CacheWrite1h, CacheRead float64
	// Fast is the speed:"fast" rate set (OpenAI: Priority), nil if unsupported.
	Fast *Rates
	// Flex is the speed:"flex" rate set (OpenAI Flex processing), nil if unsupported.
	Flex *Rates
	// Ultrafast is the speed:"ultrafast" rate set (OpenAI Ultrafast mode), nil if unsupported.
	Ultrafast *Rates
	// LongPrompt is the rate set for requests whose prompt (input + cache read
	// + cache write) exceeds LongPromptAbove; nil = flat pricing. Fast, Flex
	// and Ultrafast carry their own LongPrompt, applied at the parent's threshold.
	LongPrompt      *Rates
	LongPromptAbove int64
	// GeoPremium: inference_geo "us" costs 1.1x (Claude 4.6+).
	GeoPremium bool
}

const (
	webSearchUSD  = 10.0 / 1000
	geoMultiplier = 1.1
)

// std builds rates using the standard cache multipliers (1.25x, 2x, 0.1x).
func std(in, out float64) Rates {
	return Rates{Input: in, Output: out, CacheWrite5m: in * 1.25, CacheWrite1h: in * 2, CacheRead: in * 0.1}
}

func opusFastModeRates() *Rates { r := std(10, 50); return &r }

// opus55FastModeRates is 2x base, cache multipliers applied on top of the fast base.
func opus55FastModeRates() *Rates {
	return &Rates{Input: 8, Output: 40, CacheWrite5m: 10, CacheWrite1h: 16, CacheRead: 0.4}
}

func withGeo(r Rates) Rates { r.GeoPremium = true; return r }

func withLongPrompt(r, long Rates, above int64) Rates {
	r.LongPrompt, r.LongPromptAbove = &long, above
	return r
}

// PromptThresholds lists every LongPromptAbove in the table, ascending;
// usage must be grouped by the largest one each request's prompt exceeds.
func PromptThresholds() []int64 {
	var out []int64
	for _, r := range table {
		if r.LongPrompt != nil && !slices.Contains(out, r.LongPromptAbove) {
			out = append(out, r.LongPromptAbove)
		}
	}
	slices.Sort(out)
	return out
}

// table is keyed by normalized model id (see Normalize). Anthropic models are
// listed here; OpenAI models are merged in from openai.go.
var table = map[string]Rates{
	"claude-fable-5-1":  withGeo(Rates{Input: 10, Output: 50, CacheWrite5m: 12.5, CacheWrite1h: 20, CacheRead: 0.25}),
	"claude-mythos-5-1": withGeo(Rates{Input: 10, Output: 50, CacheWrite5m: 12.5, CacheWrite1h: 20, CacheRead: 0.25}),
	"claude-fable-5":    withGeo(std(10, 50)),
	"claude-mythos-5":   withGeo(std(10, 50)),
	"claude-opus-5-5":   withGeo(Rates{Input: 4, Output: 20, CacheWrite5m: 5, CacheWrite1h: 8, CacheRead: 0.2, Fast: opus55FastModeRates()}),
	"claude-opus-5":     withGeo(Rates{Input: 5, Output: 25, CacheWrite5m: 6.25, CacheWrite1h: 10, CacheRead: 0.5, Fast: opusFastModeRates()}),
	"claude-opus-4-8":   withGeo(Rates{Input: 5, Output: 25, CacheWrite5m: 6.25, CacheWrite1h: 10, CacheRead: 0.5, Fast: opusFastModeRates()}),
	"claude-opus-4-7":   withGeo(std(5, 25)),
	"claude-opus-4-6":   withGeo(std(5, 25)),
	"claude-opus-4-5":   std(5, 25),
	"claude-opus-4-1":   std(15, 75),
	"claude-opus-4":     std(15, 75),
	"claude-sonnet-5-5": withGeo(Rates{Input: 2, Output: 10, CacheWrite5m: 2.5, CacheWrite1h: 4, CacheRead: 0.1}),
	"claude-sonnet-5":   withGeo(std(2, 10)),
	"claude-sonnet-4-6": withGeo(std(3, 15)),
	"claude-sonnet-4-5": std(3, 15),
	"claude-sonnet-4":   std(3, 15),
	// Haiku 5.5 is priced per request by prompt length: 5x on every category over 100k.
	"claude-haiku-5-5": withGeo(withLongPrompt(std(0.1, 0.5), std(0.5, 2.5), 100_000)),
	"claude-haiku-4-5": std(1, 5),
	"claude-3-5-haiku": std(0.8, 4),
}

var (
	dateSuffix    = regexp.MustCompile(`[-@]\d{8}$|-\d{4}-\d{2}-\d{2}$`) // Anthropic / OpenAI snapshots
	bedrockSuffix = regexp.MustCompile(`-v\d+(:\d+)?$`)
)

// Normalize maps provider/platform model id variants to table keys:
// "us.anthropic.claude-sonnet-4-5-20250929-v1:0" -> "claude-sonnet-4-5",
// "claude-opus-4-6[1m]" -> "claude-opus-4-6", "claude-sonnet-4-0" -> "claude-sonnet-4".
func Normalize(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	if i := strings.Index(m, "["); i >= 0 {
		m = m[:i]
	}
	if i := strings.LastIndex(m, "anthropic."); i >= 0 {
		m = m[i+len("anthropic."):]
	}
	m = bedrockSuffix.ReplaceAllString(m, "")
	m = dateSuffix.ReplaceAllString(m, "")
	m = strings.TrimSuffix(m, "-0")
	return m
}

// Lookup returns rates for a model id.
func Lookup(model string) (Rates, bool) {
	r, ok := table[Normalize(model)]
	return r, ok
}

// Cost prices tokens that share model, speed, geo and prompt-length bucket
// (promptOver: the largest PromptThresholds value every request's prompt
// exceeded, 0 for none); ok=false if the model is unknown.
func Cost(model, speed, geo string, promptOver int64, t usage.Tokens, webSearches int64) (float64, bool) {
	r, ok := Lookup(model)
	if !ok {
		return 0, false
	}
	return costOf(r, speed, geo, promptOver, t, webSearches), true
}

func costOf(r Rates, speed, geo string, promptOver int64, t usage.Tokens, webSearches int64) float64 {
	geoPremium := r.GeoPremium
	long := r.LongPromptAbove > 0 && promptOver >= r.LongPromptAbove
	switch {
	case speed == "fast" && r.Fast != nil:
		r = *r.Fast
	case speed == "flex" && r.Flex != nil:
		r = *r.Flex
	case speed == "ultrafast" && r.Ultrafast != nil:
		r = *r.Ultrafast
	}
	if long && r.LongPrompt != nil {
		r = *r.LongPrompt
	}
	r.GeoPremium = geoPremium
	usd := (float64(t.Input)*r.Input +
		float64(t.Output)*r.Output +
		float64(t.CacheWrite5m)*r.CacheWrite5m +
		float64(t.CacheWrite1h)*r.CacheWrite1h +
		float64(t.CacheRead)*r.CacheRead) / 1e6
	if geo == "us" && r.GeoPremium {
		usd *= geoMultiplier
	}
	return usd + float64(webSearches)*webSearchUSD
}
