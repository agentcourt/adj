package openai

import "testing"

func TestAccountingRecorderPreservesPartialObservations(t *testing.T) {
	recorder := &AccountingRecorder{}
	recorder.Record(Response{
		Usage:               Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
		UsageKnown:          true,
		OpenRouterCostUSD:   0.25,
		OpenRouterCostKnown: true,
	})
	recorder.Record(Response{Usage: Usage{InputTokens: 4, OutputTokens: 2, TotalTokens: 6}, UsageKnown: true})
	recorder.Record(Response{})

	got := recorder.Snapshot()
	if got.RequestCount != 3 || got.UsageObservedCount != 2 || got.CostObservedCount != 1 {
		t.Fatalf("counts = %#v", got)
	}
	wantUsage := Usage{InputTokens: 14, OutputTokens: 7, TotalTokens: 21}
	if got.Usage == nil || *got.Usage != wantUsage {
		t.Fatalf("usage = %#v, want %#v", got.Usage, wantUsage)
	}
	if got.CostUSD == nil || *got.CostUSD != 0.25 {
		t.Fatalf("cost = %v, want 0.25", got.CostUSD)
	}

	copy := recorder.Snapshot()
	copy.Usage.InputTokens = 99
	copyCost := 1.0
	copy.CostUSD = &copyCost
	again := recorder.Snapshot()
	if again.Usage == nil || again.Usage.InputTokens != 14 || again.CostUSD == nil || *again.CostUSD != 0.25 {
		t.Fatalf("snapshot aliases recorder state: %#v", again)
	}
}

func TestNilAccountingRecorder(t *testing.T) {
	var recorder *AccountingRecorder
	recorder.Record(Response{})
	if got := recorder.Snapshot(); got != (Accounting{}) {
		t.Fatalf("nil snapshot = %#v", got)
	}
}

func TestMergeAccounting(t *testing.T) {
	firstUsage := Usage{InputTokens: 10, TotalTokens: 10}
	secondUsage := Usage{OutputTokens: 4, TotalTokens: 4}
	firstCost := 0.125
	secondCost := 0.25
	got := MergeAccounting(
		Accounting{RequestCount: 2, UsageObservedCount: 1, Usage: &firstUsage, CostObservedCount: 1, CostUSD: &firstCost},
		Accounting{RequestCount: 3, UsageObservedCount: 2, Usage: &secondUsage, CostObservedCount: 2, CostUSD: &secondCost},
	)
	if got.RequestCount != 5 || got.UsageObservedCount != 3 || got.CostObservedCount != 3 {
		t.Fatalf("counts = %#v", got)
	}
	wantUsage := Usage{InputTokens: 10, OutputTokens: 4, TotalTokens: 14}
	if got.Usage == nil || *got.Usage != wantUsage || got.CostUSD == nil || *got.CostUSD != 0.375 {
		t.Fatalf("merged accounting = %#v", got)
	}
}
