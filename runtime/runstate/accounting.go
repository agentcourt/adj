package runstate

type TokenUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens,omitempty"`
	CacheWriteInputTokens int64 `json:"cache_write_input_tokens,omitempty"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningTokens       int64 `json:"reasoning_tokens,omitempty"`
	TotalTokens           int64 `json:"total_tokens"`
}

type ProviderAccounting struct {
	RequestCount       int         `json:"request_count"`
	UsageObservedCount int         `json:"usage_observed_count"`
	Usage              *TokenUsage `json:"usage,omitempty"`
	CostObservedCount  int         `json:"cost_observed_count"`
	CostUSD            *float64    `json:"cost_usd,omitempty"`
}
