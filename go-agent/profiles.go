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
		// Qwen3-8B's official GGUF card recommends this thinking-mode sampler:
		// temperature 0.6, top-p 0.95, top-k 20, and presence penalty 1.5.
		name:            "qwen3-8b",
		promptProfile:   "baseline",
		temperature:     0.6,
		topP:            0.95,
		topK:            20,
		presencePenalty: 1.5,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// Qwen3-8B non-thinking mode follows the official card's efficient instruct
		// sampler; this A/B isolates reasoning-mode latency from tool competence.
		name:            "qwen3-8b-no-think",
		promptProfile:   "baseline",
		temperature:     0.7,
		topP:            0.8,
		topK:            20,
		presencePenalty: 1.5,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// Qwen3-14B follows the official thinking-mode sampler: temperature 0.6,
		// top-p 0.95, top-k 20, and presence penalty 1.5. The screen selects
		// low reasoning effort through the server flags.
		name:            "qwen3-14b",
		promptProfile:   "baseline",
		temperature:     0.6,
		topP:            0.95,
		topK:            20,
		presencePenalty: 1.5,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// Mistral Ministral 3 8B Instruct recommends temperature below 0.1 for
		// production/agent use; keep top-p broad so native function-call formatting
		// is tested without adding another restrictive sampler constraint.
		name:            "ministral3-8b",
		promptProfile:   "baseline",
		temperature:     0.05,
		topP:            0.95,
		topK:            0,
		presencePenalty: 0.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// The 14B Ministral 3 Instruct checkpoint uses the same documented
		// low-temperature production/agent recipe as the 8B family member.
		name:            "ministral3-14b",
		promptProfile:   "baseline",
		temperature:     0.05,
		topP:            0.95,
		topK:            0,
		presencePenalty: 0.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// The 3B Ministral 3 reasoning checkpoint uses the same conservative
		// low-temperature family recipe for this native-tool screen. Reasoning
		// effort itself is selected by the server experiment flags.
		name:            "ministral3-3b-reasoning",
		promptProfile:   "baseline",
		temperature:     0.05,
		topP:            0.95,
		topK:            0,
		presencePenalty: 0.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// The 3B Ministral 3 Instruct checkpoint uses the same documented
		// low-temperature production/agent recipe as the larger family members.
		name:            "ministral3-3b-instruct",
		promptProfile:   "baseline",
		temperature:     0.05,
		topP:            0.95,
		topK:            0,
		presencePenalty: 0.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// Devstral Small 1.1 Q2 screen profile. The official model card does not
		// publish a sampler recipe, so use a conservative agent-screen setting and
		// record it explicitly rather than relying on the harness default.
		name:            "devstral-small-2507",
		promptProfile:   "baseline",
		temperature:     0.2,
		topP:            0.95,
		topK:            0,
		presencePenalty: 0.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// Qwen2.5-Coder-7B-Instruct GGUF screen profile. Use the same bounded,
		// non-greedy coding sampler as the newer Qwen agent checkpoints so the
		// compact baseline is compared on tool behavior rather than greedy-mode
		// repetition artifacts.
		name:            "qwen2.5-coder-7b",
		promptProfile:   "baseline",
		temperature:     0.7,
		topP:            0.8,
		topK:            20,
		presencePenalty: 1.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// Qwen2.5-Coder-14B-Instruct uses the same bounded coding sampler as the
		// 7B checkpoint; the larger artifact is being screened for tool competence,
		// not given a different decoding contract.
		name:            "qwen2.5-coder-14b",
		promptProfile:   "baseline",
		temperature:     0.7,
		topP:            0.8,
		topK:            20,
		presencePenalty: 1.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// Qwen3.5-4B GGUF screen profile. The model is a compact multimodal Qwen3.5
		// instruct checkpoint; keep a non-greedy coding/agent sampler and let the
		// server experiment choose whether thinking is enabled or bounded.
		name:            "qwen35-4b",
		promptProfile:   "baseline",
		temperature:     0.7,
		topP:            0.8,
		topK:            20,
		presencePenalty: 1.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// IBM Granite 4.2-3B documents temperature 1.0 and top-p 0.95 for tool
		// calling; low reasoning effort is selected by the experiment's server flags.
		name:            "granite4.2-3b",
		promptProfile:   "baseline",
		temperature:     1.0,
		topP:            0.95,
		topK:            0,
		presencePenalty: 0.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// Qwen3-Coder-30B-A3B-Instruct GGUF screen profile. Qwen recommends
		// non-greedy decoding for its agentic function-calling format; the low
		// quantization changes footprint, not the model's sampling contract.
		name:            "qwen3-coder-30b-a3b",
		promptProfile:   "baseline",
		temperature:     0.7,
		topP:            0.8,
		topK:            20,
		presencePenalty: 1.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// DeepSeek-Coder-V2-Lite-Instruct's official examples use deterministic
		// chat sampling (top-k 50, top-p .95); its vLLM example uses temperature
		// .3. Keep that documented coding recipe explicit for the GGUF screen.
		name:            "deepseek-coder-v2-lite",
		promptProfile:   "baseline",
		temperature:     0.3,
		topP:            0.95,
		topK:            50,
		presencePenalty: 0.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// Qwen3.6-27B's official precise-coding thinking recipe: temperature .6,
		// top-p .95, top-k 20, and no presence/repetition penalty. The experiment
		// selects bounded reasoning effort; this profile only fixes decoding.
		name:            "qwen36-27b-a3b-coder",
		promptProfile:   "baseline",
		temperature:     0.6,
		topP:            0.95,
		topK:            20,
		presencePenalty: 0.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// Qwen3.5-9B GGUF screen profile. Use the same compact Qwen coding
		// recipe as the already-tested 4B member until a model-specific GGUF card
		// gives a stronger serving recommendation.
		name:            "qwen35-9b",
		promptProfile:   "baseline",
		temperature:     0.7,
		topP:            0.8,
		topK:            20,
		presencePenalty: 1.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
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
		// Exists to A/B against the non-thinking default: a live agentic run
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
	{
		// Ternary Bonsai 2 27B (PrismML, 2026-09-17), a ternary quant of Qwen3.8-27B.
		// Every published benchmark number for this model (whitepaper Section 4, Appendix B)
		// is thinking mode at reasoning effort xhigh, sampled per the whitepaper's stated
		// protocol: "the sampling recommended by the Qwen3.8 model card (temperature 1.0,
		// top-p 0.95, top-k 20, no presence or repetition penalty)". Requires the server
		// started with thinking enabled (not --reasoning off) and --reasoning-effort xhigh,
		// which this profile does not control -- see experiments-bonsai2-local.json
		// common_flags. Seed 42 matches the Qwen3.8 GSQ-RCO screen this model is compared
		// against (docs/OPTIMIZATION-QWEN38-2026-09-17.md).
		name:            "bonsai2-27b",
		promptProfile:   "baseline",
		temperature:     1.0,
		topP:            0.95,
		topK:            20,
		presencePenalty: 0.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// byteshape/Qwen3.8-27B-GGUF, IQ2_XXS-2.56bpw ("GPU-1"), ByteShape's ShapeLearn
		// per-tensor learned-datatype quant of the same Qwen3.8-27B base as bonsai2-27b.
		// Same Qwen3.8 thinking-mode sampling recommendation as bonsai2-27b; same seed 42
		// for direct comparability against the Bonsai 2 and Qwen3.8 GSQ-RCO screens.
		name:            "byteshape-gpu1",
		promptProfile:   "baseline",
		temperature:     1.0,
		topP:            0.95,
		topK:            20,
		presencePenalty: 0.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// unsloth/Qwen3.8-27B-GGUF, UD-Q2_K_XL, Unsloth Dynamic 2.0 quant of the same
		// Qwen3.8-27B base. Chosen because an independent benchmark (Quesma, see
		// docs/OPTIMIZATION-BONSAI2-2026-09-18.md) found this the last quant level before
		// Qwen3.8-27B's 1-bit collapse, with task-level accuracy (GPQA, IFBench,
		// Terminal-Bench 2.1) near BF16 -- not just a vendor aggregate score. Same
		// sampling/seed as the other two Qwen3.8-27B entries for direct comparability.
		name:            "unsloth-q2kxl",
		promptProfile:   "baseline",
		temperature:     1.0,
		topP:            0.95,
		topK:            20,
		presencePenalty: 0.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// LiquidAI/LFM2.5-8B-A1B-GGUF, Q8_0. MoE (8B total, ~1-1.5B active), official
		// day-one llama.cpp support, no custom fork. Sampling per the model's own card:
		// temp 0.2 / top_k 80 / repeat_penalty 1.05 (card gives no top_p; left at 1.0 to
		// not double-restrict on top of top_k). Model appears to always emit a thinking
		// trace (routed to message.reasoning_content by --jinja + the server's default
		// reasoning-format auto-detection); no documented way to disable it.
		name:            "lfm2.5-8b-a1b",
		promptProfile:   "baseline",
		temperature:     0.2,
		topP:            1.0,
		topK:            80,
		presencePenalty: 0.0,
		repeatPenalty:   1.05,
		seed:            42,
	},
	{
		// JetBrains Mellum2-12B-A2.5B-Thinking. The official card's OpenAI example
		// recommends these values for the thinking checkpoint; tool parsing and the
		// reasoning channel are controlled by the serving backend.
		name:            "mellum2-12b-a2.5b-thinking",
		promptProfile:   "baseline",
		temperature:     0.6,
		topP:            0.95,
		topK:            20,
		presencePenalty: 0.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// Zyphra ZAYA1-8B. The official card recommends a lower temperature for
		// agent/code work and no top-k restriction (the GGUF generation config uses
		// top_k = -1, represented as 0 for llama.cpp's disabled value).
		name:            "zaya1-8b",
		promptProfile:   "baseline",
		temperature:     0.6,
		topP:            0.95,
		topK:            0,
		presencePenalty: 0.0,
		repeatPenalty:   1.0,
		seed:            42,
	},
	{
		// Xing4.0-29B-A4B. Its coding/agent recommendation is temperature 0.8,
		// top-p 0.95, repetition penalty 1.05. The custom Xing fork is required
		// for the local GGUF's model architecture.
		name:            "xing4-29b-a4b",
		promptProfile:   "baseline",
		temperature:     0.8,
		topP:            0.95,
		topK:            0,
		presencePenalty: 0.0,
		repeatPenalty:   1.05,
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
