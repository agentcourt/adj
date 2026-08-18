package openai

import "sync"

type Accounting struct {
	RequestCount       int      `json:"request_count"`
	UsageObservedCount int      `json:"usage_observed_count"`
	Usage              *Usage   `json:"usage,omitempty"`
	CostObservedCount  int      `json:"cost_observed_count"`
	CostUSD            *float64 `json:"cost_usd,omitempty"`
}

type AccountingRecorder struct {
	mu sync.Mutex

	requestCount       int
	usageObservedCount int
	usage              Usage
	costObservedCount  int
	costUSD            float64
}

func (r *AccountingRecorder) Record(response Response) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requestCount++
	if usage := response.TokenUsage(); usage != nil {
		r.usageObservedCount++
		r.usage.InputTokens += usage.InputTokens
		r.usage.CachedInputTokens += usage.CachedInputTokens
		r.usage.OutputTokens += usage.OutputTokens
		r.usage.ReasoningTokens += usage.ReasoningTokens
		r.usage.TotalTokens += usage.TotalTokens
	}
	if cost := response.CostUSD(); cost != nil {
		r.costObservedCount++
		r.costUSD += *cost
	}
}

func (r *AccountingRecorder) Snapshot() Accounting {
	if r == nil {
		return Accounting{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	accounting := Accounting{
		RequestCount:       r.requestCount,
		UsageObservedCount: r.usageObservedCount,
		CostObservedCount:  r.costObservedCount,
	}
	if r.usageObservedCount > 0 {
		usage := r.usage
		accounting.Usage = &usage
	}
	if r.costObservedCount > 0 {
		cost := r.costUSD
		accounting.CostUSD = &cost
	}
	return accounting
}

func MergeAccounting(values ...Accounting) Accounting {
	var recorder AccountingRecorder
	for _, value := range values {
		recorder.requestCount += value.RequestCount
		recorder.usageObservedCount += value.UsageObservedCount
		if value.Usage != nil {
			recorder.usage.InputTokens += value.Usage.InputTokens
			recorder.usage.CachedInputTokens += value.Usage.CachedInputTokens
			recorder.usage.OutputTokens += value.Usage.OutputTokens
			recorder.usage.ReasoningTokens += value.Usage.ReasoningTokens
			recorder.usage.TotalTokens += value.Usage.TotalTokens
		}
		recorder.costObservedCount += value.CostObservedCount
		if value.CostUSD != nil {
			recorder.costUSD += *value.CostUSD
		}
	}
	return recorder.Snapshot()
}
