package councilsample

import "testing"

func TestSelectorRepeatsSuccessfulConfiguration(t *testing.T) {
	selector, err := New([]string{"openai"}, Options{Count: 3, MinimumDistinctEndpoints: 1})
	if err != nil {
		t.Fatal(err)
	}
	for seat := 0; seat < 3; seat++ {
		index, err := selector.Draw()
		if err != nil {
			t.Fatal(err)
		}
		if index != 0 {
			t.Fatalf("candidate index = %d, want 0", index)
		}
		if err := selector.Accept(index); err != nil {
			t.Fatal(err)
		}
	}
	if err := selector.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSelectorUsesEachEndpointBeforeRepeatingOne(t *testing.T) {
	selector, err := New([]string{"openai", "anthropic", "google"}, Options{Count: 3, MinimumDistinctEndpoints: 3})
	if err != nil {
		t.Fatal(err)
	}
	selected := make(map[int]bool)
	for seat := 0; seat < 3; seat++ {
		index, err := selector.Draw()
		if err != nil {
			t.Fatal(err)
		}
		selected[index] = true
		if err := selector.Accept(index); err != nil {
			t.Fatal(err)
		}
	}
	if len(selected) != 3 {
		t.Fatalf("selected %d distinct configurations, want 3", len(selected))
	}
	if err := selector.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSelectorRemovesRejectedEndpoint(t *testing.T) {
	selector, err := New([]string{"openai", "openai", "anthropic"}, Options{Count: 2})
	if err != nil {
		t.Fatal(err)
	}
	selected := make([]string, 0, 2)
	for len(selected) < 2 {
		index, err := selector.Draw()
		if err != nil {
			t.Fatal(err)
		}
		if selector.endpoints[index] == "openai" {
			if err := selector.RejectEndpoint("openai"); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := selector.Accept(index); err != nil {
			t.Fatal(err)
		}
		selected = append(selected, selector.endpoints[index])
	}
	for _, endpoint := range selected {
		if endpoint != "anthropic" {
			t.Fatalf("selected endpoints = %v, want only anthropic", selected)
		}
	}
}

func TestSelectorAppliesAllowedEndpoints(t *testing.T) {
	selector, err := New(
		[]string{"openrouter", "openai", "anthropic"},
		Options{Count: 2, AllowedEndpoints: []string{"openai", "anthropic"}, MinimumDistinctEndpoints: 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	for seat := 0; seat < 2; seat++ {
		index, err := selector.Draw()
		if err != nil {
			t.Fatal(err)
		}
		if selector.endpoints[index] == "openrouter" {
			t.Fatal("selected excluded OpenRouter configuration")
		}
		if err := selector.Accept(index); err != nil {
			t.Fatal(err)
		}
	}
	if err := selector.Validate(); err != nil {
		t.Fatal(err)
	}
}
