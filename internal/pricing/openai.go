// OpenAI list prices extracted from https://developers.openai.com/api/docs/pricing
// (fetched 2026-11-14). No generator is kept: the page's markup changes, so
// re-extract or edit by hand, re-checking every tier against the page.

package pricing

// openAILongContext: requests over 272K input tokens use LongPrompt rates.
const openAILongContext = 272_000

// openAI lists every text-token model on the OpenAI pricing page except
// Batch-only, embedding, moderation, audio, image and fine-tuned prices.
// Page rules: CacheWrite5m is the 30m write, CacheWrite1h the 12h write
// (input × 2.5); a missing write price means writes cost input, a missing
// cached price means no discount. Fast = Priority; Flex rows come from the
// page's Batch/Flex table.
var openAI = map[string]Rates{
	"chat-latest": {
		Input: 5, CacheRead: 0.5, CacheWrite5m: 5, CacheWrite1h: 5, Output: 30,
	},
	"gpt-4.1": {
		Input: 2, CacheRead: 0.5, CacheWrite5m: 2, CacheWrite1h: 2, Output: 8,
		Fast: &Rates{Input: 3.5, CacheRead: 0.875, CacheWrite5m: 3.5, CacheWrite1h: 3.5, Output: 14},
	},
	"gpt-4.1-mini": {
		Input: 0.4, CacheRead: 0.1, CacheWrite5m: 0.4, CacheWrite1h: 0.4, Output: 1.6,
		Fast: &Rates{Input: 0.7, CacheRead: 0.175, CacheWrite5m: 0.7, CacheWrite1h: 0.7, Output: 2.8},
	},
	"gpt-4o": {
		Input: 2.5, CacheRead: 1.25, CacheWrite5m: 2.5, CacheWrite1h: 2.5, Output: 10,
		Fast: &Rates{Input: 4.25, CacheRead: 2.125, CacheWrite5m: 4.25, CacheWrite1h: 4.25, Output: 17},
	},
	"gpt-4o-mini": {
		Input: 0.15, CacheRead: 0.075, CacheWrite5m: 0.15, CacheWrite1h: 0.15, Output: 0.6,
		Fast: &Rates{Input: 0.25, CacheRead: 0.125, CacheWrite5m: 0.25, CacheWrite1h: 0.25, Output: 1},
	},
	"gpt-5": {
		Input: 1.25, CacheRead: 0.125, CacheWrite5m: 1.25, CacheWrite1h: 1.25, Output: 10,
		Flex: &Rates{Input: 0.625, CacheRead: 0.0625, CacheWrite5m: 0.625, CacheWrite1h: 0.625, Output: 5},
		Fast: &Rates{Input: 2.5, CacheRead: 0.25, CacheWrite5m: 2.5, CacheWrite1h: 2.5, Output: 20},
	},
	"gpt-5-mini": {
		Input: 0.25, CacheRead: 0.025, CacheWrite5m: 0.25, CacheWrite1h: 0.25, Output: 2,
		Flex: &Rates{Input: 0.125, CacheRead: 0.0125, CacheWrite5m: 0.125, CacheWrite1h: 0.125, Output: 1},
		Fast: &Rates{Input: 0.45, CacheRead: 0.045, CacheWrite5m: 0.45, CacheWrite1h: 0.45, Output: 3.6},
	},
	"gpt-5-nano": {
		Input: 0.05, CacheRead: 0.005, CacheWrite5m: 0.05, CacheWrite1h: 0.05, Output: 0.4,
		Flex: &Rates{Input: 0.025, CacheRead: 0.0025, CacheWrite5m: 0.025, CacheWrite1h: 0.025, Output: 0.2},
	},
	"gpt-5-pro": {
		Input: 15, CacheRead: 15, CacheWrite5m: 15, CacheWrite1h: 15, Output: 120,
	},
	"gpt-5-search-api": {
		Input: 1.25, CacheRead: 0.125, CacheWrite5m: 1.25, CacheWrite1h: 1.25, Output: 10,
	},
	"gpt-5.1": {
		Input: 1.25, CacheRead: 0.125, CacheWrite5m: 1.25, CacheWrite1h: 1.25, Output: 10,
		Flex: &Rates{Input: 0.625, CacheRead: 0.0625, CacheWrite5m: 0.625, CacheWrite1h: 0.625, Output: 5},
		Fast: &Rates{Input: 2.5, CacheRead: 0.25, CacheWrite5m: 2.5, CacheWrite1h: 2.5, Output: 20},
	},
	"gpt-5.2": {
		Input: 1.75, CacheRead: 0.175, CacheWrite5m: 1.75, CacheWrite1h: 1.75, Output: 14,
		Flex: &Rates{Input: 0.875, CacheRead: 0.0875, CacheWrite5m: 0.875, CacheWrite1h: 0.875, Output: 7},
		Fast: &Rates{Input: 3.5, CacheRead: 0.35, CacheWrite5m: 3.5, CacheWrite1h: 3.5, Output: 28},
	},
	"gpt-5.2-pro": {
		Input: 21, CacheRead: 21, CacheWrite5m: 21, CacheWrite1h: 21, Output: 168,
	},
	"gpt-5.3-codex": {
		Input: 1.75, CacheRead: 0.175, CacheWrite5m: 1.75, CacheWrite1h: 1.75, Output: 14,
		Fast: &Rates{Input: 3.5, CacheRead: 0.35, CacheWrite5m: 3.5, CacheWrite1h: 3.5, Output: 28},
	},
	"gpt-5.4": {
		Input: 2.5, CacheRead: 0.25, CacheWrite5m: 2.5, CacheWrite1h: 2.5, Output: 15,
		LongPromptAbove: openAILongContext, LongPrompt: &Rates{Input: 5, CacheRead: 0.5, CacheWrite5m: 5, CacheWrite1h: 5, Output: 22.5},
		Flex: &Rates{Input: 1.25, CacheRead: 0.13, CacheWrite5m: 1.25, CacheWrite1h: 1.25, Output: 7.5,
			LongPrompt: &Rates{Input: 2.5, CacheRead: 0.25, CacheWrite5m: 2.5, CacheWrite1h: 2.5, Output: 11.25}},
		Fast: &Rates{Input: 5, CacheRead: 0.5, CacheWrite5m: 5, CacheWrite1h: 5, Output: 30},
	},
	"gpt-5.4-mini": {
		Input: 0.75, CacheRead: 0.075, CacheWrite5m: 0.75, CacheWrite1h: 0.75, Output: 4.5,
		Flex: &Rates{Input: 0.375, CacheRead: 0.0375, CacheWrite5m: 0.375, CacheWrite1h: 0.375, Output: 2.25},
		Fast: &Rates{Input: 1.5, CacheRead: 0.15, CacheWrite5m: 1.5, CacheWrite1h: 1.5, Output: 9},
	},
	"gpt-5.4-nano": {
		Input: 0.2, CacheRead: 0.02, CacheWrite5m: 0.2, CacheWrite1h: 0.2, Output: 1.25,
		Flex: &Rates{Input: 0.1, CacheRead: 0.01, CacheWrite5m: 0.1, CacheWrite1h: 0.1, Output: 0.625},
	},
	"gpt-5.4-pro": {
		Input: 30, CacheRead: 30, CacheWrite5m: 30, CacheWrite1h: 30, Output: 180,
		LongPromptAbove: openAILongContext, LongPrompt: &Rates{Input: 60, CacheRead: 60, CacheWrite5m: 60, CacheWrite1h: 60, Output: 270},
		Flex: &Rates{Input: 15, CacheRead: 15, CacheWrite5m: 15, CacheWrite1h: 15, Output: 90,
			LongPrompt: &Rates{Input: 30, CacheRead: 30, CacheWrite5m: 30, CacheWrite1h: 30, Output: 135}},
	},
	"gpt-5.5": {
		Input: 5, CacheRead: 0.5, CacheWrite5m: 5, CacheWrite1h: 5, Output: 30,
		LongPromptAbove: openAILongContext, LongPrompt: &Rates{Input: 10, CacheRead: 1, CacheWrite5m: 10, CacheWrite1h: 10, Output: 45},
		Flex: &Rates{Input: 2.5, CacheRead: 0.25, CacheWrite5m: 2.5, CacheWrite1h: 2.5, Output: 15,
			LongPrompt: &Rates{Input: 5, CacheRead: 0.5, CacheWrite5m: 5, CacheWrite1h: 5, Output: 22.5}},
		Fast: &Rates{Input: 12.5, CacheRead: 1.25, CacheWrite5m: 12.5, CacheWrite1h: 12.5, Output: 75},
	},
	"gpt-5.5-cyber-preview": {
		Input: 12.5, CacheRead: 1.25, CacheWrite5m: 12.5, CacheWrite1h: 12.5, Output: 75,
	},
	// gpt-5.5-cyber: earlier name of gpt-5.5-cyber-preview, kept for older logs.
	"gpt-5.5-cyber": {
		Input: 12.5, CacheRead: 1.25, CacheWrite5m: 12.5, CacheWrite1h: 12.5, Output: 75,
	},
	"gpt-5.5-pro": {
		Input: 30, CacheRead: 30, CacheWrite5m: 30, CacheWrite1h: 30, Output: 180,
		LongPromptAbove: openAILongContext, LongPrompt: &Rates{Input: 60, CacheRead: 60, CacheWrite5m: 60, CacheWrite1h: 60, Output: 270},
		Flex: &Rates{Input: 15, CacheRead: 15, CacheWrite5m: 15, CacheWrite1h: 15, Output: 90},
	},
	"gpt-5.6-cyber": {
		Input: 12.5, CacheRead: 1.25, CacheWrite5m: 15.625, CacheWrite1h: 31.25, Output: 75,
	},
	"gpt-5.6-luna": {
		Input: 0.2, CacheRead: 0.02, CacheWrite5m: 0.25, CacheWrite1h: 0.5, Output: 1.2,
		LongPromptAbove: openAILongContext, LongPrompt: &Rates{Input: 0.4, CacheRead: 0.04, CacheWrite5m: 0.5, CacheWrite1h: 1, Output: 1.8},
		Flex: &Rates{Input: 0.1, CacheRead: 0.01, CacheWrite5m: 0.125, CacheWrite1h: 0.25, Output: 0.6,
			LongPrompt: &Rates{Input: 0.2, CacheRead: 0.02, CacheWrite5m: 0.25, CacheWrite1h: 0.5, Output: 0.9}},
		Fast: &Rates{Input: 0.4, CacheRead: 0.04, CacheWrite5m: 0.5, CacheWrite1h: 1, Output: 2.4,
			LongPrompt: &Rates{Input: 0.8, CacheRead: 0.08, CacheWrite5m: 1, CacheWrite1h: 2, Output: 3.6}},
	},
	"gpt-5.6-sol": {
		Input: 4, CacheRead: 0.4, CacheWrite5m: 5, CacheWrite1h: 10, Output: 20,
		LongPromptAbove: openAILongContext, LongPrompt: &Rates{Input: 8, CacheRead: 0.8, CacheWrite5m: 10, CacheWrite1h: 20, Output: 30},
		Flex: &Rates{Input: 2, CacheRead: 0.2, CacheWrite5m: 2.5, CacheWrite1h: 5, Output: 10,
			LongPrompt: &Rates{Input: 4, CacheRead: 0.4, CacheWrite5m: 5, CacheWrite1h: 10, Output: 15}},
		Fast: &Rates{Input: 8, CacheRead: 0.8, CacheWrite5m: 10, CacheWrite1h: 20, Output: 40,
			LongPrompt: &Rates{Input: 16, CacheRead: 1.6, CacheWrite5m: 20, CacheWrite1h: 40, Output: 60}},
	},
	"gpt-5.6-terra": {
		Input: 2, CacheRead: 0.2, CacheWrite5m: 2.5, CacheWrite1h: 5, Output: 12,
		LongPromptAbove: openAILongContext, LongPrompt: &Rates{Input: 4, CacheRead: 0.4, CacheWrite5m: 5, CacheWrite1h: 10, Output: 18},
		Flex: &Rates{Input: 1, CacheRead: 0.1, CacheWrite5m: 1.25, CacheWrite1h: 2.5, Output: 6,
			LongPrompt: &Rates{Input: 2, CacheRead: 0.2, CacheWrite5m: 2.5, CacheWrite1h: 5, Output: 9}},
		Fast: &Rates{Input: 4, CacheRead: 0.4, CacheWrite5m: 5, CacheWrite1h: 10, Output: 24,
			LongPrompt: &Rates{Input: 8, CacheRead: 0.8, CacheWrite5m: 10, CacheWrite1h: 20, Output: 36}},
	},
	"gpt-6-astra": {
		Input: 10, CacheRead: 1, CacheWrite5m: 12.5, CacheWrite1h: 25, Output: 50,
		LongPromptAbove: openAILongContext, LongPrompt: &Rates{Input: 20, CacheRead: 2, CacheWrite5m: 25, CacheWrite1h: 50, Output: 75},
		Flex: &Rates{Input: 5, CacheRead: 0.5, CacheWrite5m: 6.25, CacheWrite1h: 12.5, Output: 25,
			LongPrompt: &Rates{Input: 10, CacheRead: 1, CacheWrite5m: 12.5, CacheWrite1h: 25, Output: 37.5}},
		Fast: &Rates{Input: 20, CacheRead: 2, CacheWrite5m: 25, CacheWrite1h: 50, Output: 100,
			LongPrompt: &Rates{Input: 40, CacheRead: 4, CacheWrite5m: 50, CacheWrite1h: 100, Output: 150}},
		Ultrafast: &Rates{Input: 60, CacheRead: 6, CacheWrite5m: 75, CacheWrite1h: 150, Output: 300,
			LongPrompt: &Rates{Input: 120, CacheRead: 12, CacheWrite5m: 150, CacheWrite1h: 300, Output: 450}},
	},
	"gpt-6-astra-law": {
		Input: 10, CacheRead: 1, CacheWrite5m: 10, CacheWrite1h: 10, Output: 50,
	},
	"gpt-6-cyber": {
		Input: 20, CacheRead: 2, CacheWrite5m: 25, CacheWrite1h: 50, Output: 100,
		LongPromptAbove: openAILongContext, LongPrompt: &Rates{Input: 40, CacheRead: 4, CacheWrite5m: 40, CacheWrite1h: 40, Output: 150},
	},
	"gpt-6-luna": {
		Input: 0.1, CacheRead: 0.01, CacheWrite5m: 0.125, CacheWrite1h: 0.25, Output: 0.5,
		LongPromptAbove: openAILongContext, LongPrompt: &Rates{Input: 0.2, CacheRead: 0.02, CacheWrite5m: 0.25, CacheWrite1h: 0.5, Output: 0.75},
		Flex: &Rates{Input: 0.05, CacheRead: 0.005, CacheWrite5m: 0.0625, CacheWrite1h: 0.125, Output: 0.25,
			LongPrompt: &Rates{Input: 0.1, CacheRead: 0.01, CacheWrite5m: 0.125, CacheWrite1h: 0.25, Output: 0.375}},
		Fast: &Rates{Input: 0.2, CacheRead: 0.02, CacheWrite5m: 0.25, CacheWrite1h: 0.5, Output: 1,
			LongPrompt: &Rates{Input: 0.4, CacheRead: 0.04, CacheWrite5m: 0.5, CacheWrite1h: 1, Output: 1.5}},
	},
	"gpt-6-rosalind": {
		Input: 15, CacheRead: 1.5, CacheWrite5m: 18.75, CacheWrite1h: 37.5, Output: 75,
		LongPromptAbove: openAILongContext, LongPrompt: &Rates{Input: 30, CacheRead: 3, CacheWrite5m: 37.5, CacheWrite1h: 75, Output: 112.5},
	},
	"gpt-6-sol": {
		Input: 2, CacheRead: 0.2, CacheWrite5m: 2.5, CacheWrite1h: 5, Output: 10,
		LongPromptAbove: openAILongContext, LongPrompt: &Rates{Input: 4, CacheRead: 0.4, CacheWrite5m: 5, CacheWrite1h: 10, Output: 15},
		Flex: &Rates{Input: 1, CacheRead: 0.1, CacheWrite5m: 1.25, CacheWrite1h: 2.5, Output: 5,
			LongPrompt: &Rates{Input: 2, CacheRead: 0.2, CacheWrite5m: 2.5, CacheWrite1h: 5, Output: 7.5}},
		Fast: &Rates{Input: 4, CacheRead: 0.4, CacheWrite5m: 5, CacheWrite1h: 10, Output: 20,
			LongPrompt: &Rates{Input: 8, CacheRead: 0.8, CacheWrite5m: 10, CacheWrite1h: 20, Output: 30}},
	},
	"gpt-6.1-sol": {
		Input: 2, CacheRead: 0.1, CacheWrite5m: 2.5, CacheWrite1h: 5, Output: 10,
		LongPromptAbove: openAILongContext, LongPrompt: &Rates{Input: 4, CacheRead: 0.2, CacheWrite5m: 5, CacheWrite1h: 10, Output: 15},
		Flex: &Rates{Input: 1, CacheRead: 0.05, CacheWrite5m: 1.25, CacheWrite1h: 2.5, Output: 5,
			LongPrompt: &Rates{Input: 2, CacheRead: 0.1, CacheWrite5m: 2.5, CacheWrite1h: 5, Output: 7.5}},
		Fast: &Rates{Input: 4, CacheRead: 0.2, CacheWrite5m: 5, CacheWrite1h: 10, Output: 20,
			LongPrompt: &Rates{Input: 8, CacheRead: 0.4, CacheWrite5m: 10, CacheWrite1h: 20, Output: 30}},
		Ultrafast: &Rates{Input: 12, CacheRead: 0.6, CacheWrite5m: 15, CacheWrite1h: 30, Output: 60,
			LongPrompt: &Rates{Input: 24, CacheRead: 1.2, CacheWrite5m: 30, CacheWrite1h: 60, Output: 90}},
	},
	"gpt-6.1-terra": {
		Input: 1, CacheRead: 0.05, CacheWrite5m: 1.25, CacheWrite1h: 2.5, Output: 5,
		LongPromptAbove: openAILongContext, LongPrompt: &Rates{Input: 2, CacheRead: 0.1, CacheWrite5m: 2.5, CacheWrite1h: 5, Output: 7.5},
		Flex: &Rates{Input: 0.5, CacheRead: 0.025, CacheWrite5m: 0.625, CacheWrite1h: 1.25, Output: 2.5,
			LongPrompt: &Rates{Input: 1, CacheRead: 0.05, CacheWrite5m: 1.25, CacheWrite1h: 2.5, Output: 3.75}},
		Fast: &Rates{Input: 2, CacheRead: 0.1, CacheWrite5m: 2.5, CacheWrite1h: 5, Output: 10,
			LongPrompt: &Rates{Input: 4, CacheRead: 0.2, CacheWrite5m: 5, CacheWrite1h: 10, Output: 15}},
	},
	"gpt-rosalind-discovery": {
		Input: 5, CacheRead: 0.5, CacheWrite5m: 5, CacheWrite1h: 5, Output: 25,
	},
	"gpt-rosalind-research": {
		Input: 5, CacheRead: 0.5, CacheWrite5m: 5, CacheWrite1h: 5, Output: 25,
	},
	"o3": {
		Input: 2, CacheRead: 0.5, CacheWrite5m: 2, CacheWrite1h: 2, Output: 8,
		Flex: &Rates{Input: 1, CacheRead: 0.25, CacheWrite5m: 1, CacheWrite1h: 1, Output: 4},
		Fast: &Rates{Input: 3.5, CacheRead: 0.875, CacheWrite5m: 3.5, CacheWrite1h: 3.5, Output: 14},
	},
	"o3-pro": {
		Input: 20, CacheRead: 20, CacheWrite5m: 20, CacheWrite1h: 20, Output: 80,
	},
}

func init() {
	for k, v := range openAI {
		table[k] = v
	}
}
