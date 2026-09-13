package main

// A model profile is everything about the target model that main's fixed config used to
// hardcode as literals: its own documented sampling recommendation and which system prompt
// variant suits it. Adding a model to this harness should mean adding one entry here, not
// editing the CLI's defaults. Tool-call recovery is deliberately NOT part of a profile: the
// registry in recovery_registry.go already selects by output shape, not by declared model
// name, and stays correct for a model no profile below has ever heard of.
type modelProfile struct {
	name            string
	promptProfile   string
	temperature     float64
	topP            float64
	topK            int
	presencePenalty float64
	repeatPenalty   float64
	seed            int
}

var modelProfiles = []modelProfile{
	{
		// Qwen3.6-35B-A3B's own documented instruct/non-thinking-mode recommendation, not
		// greedy decoding: the model card explicitly warns greedy decoding can cause the
		// exact "endless repetition" failure this harness spent real effort building
		// structural workarounds for (the ledger, read-dedup, the per-turn tool ban). A
		// fixed seed keeps runs reproducible for the regression suite despite non-zero
		// temperature.
		name:            "qwen36-35b-a3b",
		promptProfile:   "baseline",
		temperature:     0.7,
		topP:            0.8,
		topK:            20,
		presencePenalty: 1.5,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// Qwen3.6-35B-A3B's own documented THINKING-mode recommendation for precise coding
		// tasks specifically (huggingface.co/Qwen/Qwen3.6-35B-A3B, "Thinking Mode - Precise
		// Coding Tasks"), distinct from both its general-tasks thinking profile (temp 1.0,
		// presence_penalty 1.5) and the instruct/non-thinking profile above. Requires the
		// server started WITHOUT --reasoning off (thinking is this model's default mode).
		// Exists to A/B against the non-thinking default: a live agentic run this session
		// found the non-thinking model exploring a real repo for minutes without ever
		// committing to an edit, which thinking mode may or may not fix.
		name:            "qwen36-35b-a3b-thinking",
		promptProfile:   "baseline",
		temperature:     0.6,
		topP:            0.95,
		topK:            20,
		presencePenalty: 0.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
}

// defaultProfile applies whenever --model names something not in modelProfiles above: the
// model ID still passes through to the server untouched, sampling just falls back to a
// conservative non-greedy default instead of a model-specific tuning nobody has captured yet.
var defaultProfile = modelProfile{
	promptProfile:   "baseline",
	temperature:     0.7,
	topP:            0.8,
	topK:            20,
	presencePenalty: 1.0,
	repeatPenalty:   1.0,
	seed:            42,
}

func profileFor(model string) modelProfile {
	for _, profile := range modelProfiles {
		if profile.name == model {
			return profile
		}
	}
	return defaultProfile
}

func applyProfile(config *Config, profile modelProfile) {
	config.PromptProfile = profile.promptProfile
	config.Temperature = profile.temperature
	topP, topK, presencePenalty, repeatPenalty, seed := profile.topP, profile.topK, profile.presencePenalty, profile.repeatPenalty, profile.seed
	config.TopP = &topP
	config.TopK = &topK
	config.PresencePenalty = &presencePenalty
	config.RepeatPenalty = &repeatPenalty
	config.Seed = &seed
}
