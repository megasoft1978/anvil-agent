// Both supported response formats report reused prompt tokens. Do not infer cache hits
// from prompt length, or turn an absent/malformed counter into a measured zero.
export function cachedPromptTokens(response) {
  const timings = response.timings ?? response.Timings ?? {};
  const usage = response.usage ?? response.Usage ?? {};
  const details = usage.prompt_tokens_details ?? usage.PromptTokensDetails ?? {};
  for (const value of [timings.cache_n, timings.cache_tokens, timings.cached_tokens, details.cached_tokens]) {
    if (typeof value === "number" && Number.isSafeInteger(value) && value >= 0) return value;
  }
  return null;
}
