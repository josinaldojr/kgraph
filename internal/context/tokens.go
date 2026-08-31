package context

// EstimateTokens is a conservative (deliberately over-, not under-,
// counting) approximation of LLM token count, used for budget-aware
// truncation since kgraph has no Anthropic-compatible tokenizer available
// in Go. Code text tends to run closer to ~3 chars/token than prose's ~4
// (more punctuation/identifiers), so dividing by 3 biases the estimate up
// — per design.md's "bias the estimate conservative (round up)" trade-off.
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return len(s)/3 + 1
}
