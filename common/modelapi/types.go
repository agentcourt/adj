package modelapi

import (
	"context"
	"errors"
	"sync"
)

type ToolCall struct {
	CallID         string
	Name           string
	Arguments      map[string]any
	RawArguments   string
	ArgumentsError string
}

type WebSearchCall struct {
	ID      string            `json:"id"`
	Status  string            `json:"status"`
	Action  string            `json:"action"`
	Queries []string          `json:"queries,omitempty"`
	URL     string            `json:"url,omitempty"`
	Pattern string            `json:"pattern,omitempty"`
	Sources []WebSearchSource `json:"sources,omitempty"`
}

type WebSearchSource struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type URLCitation struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	StartIndex int64  `json:"start_index"`
	EndIndex   int64  `json:"end_index"`
}

type Response struct {
	Text                      string
	ToolCalls                 []ToolCall
	WebSearchCalls            []WebSearchCall
	URLCitations              []URLCitation
	ResponseID                string
	ReturnedModel             string
	RawJSON                   string
	Usage                     Usage
	UsageKnown                bool
	OpenRouterMetadata        map[string]any
	OpenRouterGeneration      map[string]any
	OpenRouterGenerationError string
	OpenRouterCostUSD         float64
	OpenRouterCostKnown       bool
}

type Usage struct {
	InputTokens       int64 `json:"input_tokens"`
	CachedInputTokens int64 `json:"cached_input_tokens,omitempty"`
	OutputTokens      int64 `json:"output_tokens"`
	ReasoningTokens   int64 `json:"reasoning_tokens,omitempty"`
	TotalTokens       int64 `json:"total_tokens"`
}

func (r Response) CostUSD() *float64 {
	if !r.OpenRouterCostKnown {
		return nil
	}
	cost := r.OpenRouterCostUSD
	return &cost
}

func (r Response) TokenUsage() *Usage {
	if !r.UsageKnown {
		return nil
	}
	usage := r.Usage
	return &usage
}

type ProviderErrorClass string

const (
	ProviderErrorTransient      ProviderErrorClass = "provider_transient"
	ProviderErrorAuthentication ProviderErrorClass = "provider_authentication"
	ProviderErrorRequest        ProviderErrorClass = "provider_request"
	ProviderErrorProtocol       ProviderErrorClass = "provider_protocol"
)

type ProviderError struct {
	Class ProviderErrorClass
	Err   error
}

func (e *ProviderError) Error() string { return e.Err.Error() }
func (e *ProviderError) Unwrap() error { return e.Err }

func ErrorClass(err error) ProviderErrorClass {
	if errors.Is(err, context.Canceled) {
		return ""
	}
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Class
	}
	return ""
}

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
