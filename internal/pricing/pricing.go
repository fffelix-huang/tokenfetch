// Package pricing estimates USD cost from token counts using Anthropic list prices.
//
// Source: https://platform.claude.com/docs/en/about-claude/pricing (fetched 2026-09-17).
// Subscription (Pro/Max) users are not billed per token; this is the API-equivalent cost.
package pricing

import (
	"regexp"
	"strings"

	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

// Rates are USD per million tokens.
type Rates struct {
	Input, Output, CacheWrite5m, CacheWrite1h, CacheRead float64
	// Fast is the speed:"fast" base rate set, nil if unsupported.
	Fast *Rates
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

func withGeo(r Rates) Rates { r.GeoPremium = true; return r }

// table is keyed by normalized model id (see Normalize).
var table = map[string]Rates{
	"claude-fable-5-1":  withGeo(Rates{Input: 10, Output: 50, CacheWrite5m: 12.5, CacheWrite1h: 20, CacheRead: 0.25}),
	"claude-mythos-5-1": withGeo(Rates{Input: 10, Output: 50, CacheWrite5m: 12.5, CacheWrite1h: 20, CacheRead: 0.25}),
	"claude-fable-5":    withGeo(std(10, 50)),
	"claude-mythos-5":   withGeo(std(10, 50)),
	"claude-opus-5":     withGeo(Rates{Input: 5, Output: 25, CacheWrite5m: 6.25, CacheWrite1h: 10, CacheRead: 0.5, Fast: opusFastModeRates()}),
	"claude-opus-4-8":   withGeo(Rates{Input: 5, Output: 25, CacheWrite5m: 6.25, CacheWrite1h: 10, CacheRead: 0.5, Fast: opusFastModeRates()}),
	"claude-opus-4-7":   withGeo(std(5, 25)),
	"claude-opus-4-6":   withGeo(std(5, 25)),
	"claude-opus-4-5":   std(5, 25),
	"claude-opus-4-1":   std(15, 75),
	"claude-opus-4":     std(15, 75),
	"claude-sonnet-5":   withGeo(std(2, 10)),
	"claude-sonnet-4-6": withGeo(std(3, 15)),
	"claude-sonnet-4-5": std(3, 15),
	"claude-sonnet-4":   std(3, 15),
	"claude-haiku-4-5":  std(1, 5),
	"claude-3-5-haiku":  std(0.8, 4),
}

var (
	dateSuffix    = regexp.MustCompile(`[-@]\d{8}$`)
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

// Cost prices tokens that share model, speed and geo; ok=false if the model is unknown.
func Cost(model, speed, geo string, t usage.Tokens, webSearches int64) (float64, bool) {
	r, ok := Lookup(model)
	if !ok {
		return 0, false
	}
	return costOf(r, speed, geo, t, webSearches), true
}

func costOf(r Rates, speed, geo string, t usage.Tokens, webSearches int64) float64 {
	if speed == "fast" && r.Fast != nil {
		r = *r.Fast
	}
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
