package pimodel

import (
	"encoding/json"
	"strconv"
	"strings"
)

func OpenRouterCompat(routing map[string]any, metadata map[string]any) map[string]any {
	compat := map[string]any{}
	if len(routing) > 0 {
		compat["openRouterRouting"] = routing
	}
	parameters, known := supportedParameters(metadata)
	if known {
		if _, ok := parameters["store"]; !ok {
			compat["supportsStore"] = false
		}
		_, maxTokens := parameters["max_tokens"]
		_, maxCompletionTokens := parameters["max_completion_tokens"]
		if maxTokens && !maxCompletionTokens {
			compat["maxTokensField"] = "max_tokens"
		}
	}
	if len(compat) == 0 {
		return nil
	}
	return compat
}

func OpenRouterCost(metadata map[string]any) map[string]any {
	input, inputOK := price(metadata["pricing_prompt"])
	output, outputOK := price(metadata["pricing_completion"])
	if !inputOK || !outputOK {
		return nil
	}
	cacheRead, ok := price(metadata["pricing_input_cache_read"])
	if !ok {
		cacheRead = input
	}
	cacheWrite, ok := price(metadata["pricing_input_cache_write"])
	if !ok {
		cacheWrite = input
	}
	return map[string]any{
		"input": input * 1_000_000, "output": output * 1_000_000,
		"cacheRead": cacheRead * 1_000_000, "cacheWrite": cacheWrite * 1_000_000,
	}
}

func price(value any) (float64, bool) {
	var parsed float64
	var err error
	switch value := value.(type) {
	case string:
		parsed, err = strconv.ParseFloat(strings.TrimSpace(value), 64)
	case float64:
		parsed = value
	case json.Number:
		parsed, err = value.Float64()
	default:
		return 0, false
	}
	return parsed, err == nil && parsed >= 0
}

func supportedParameters(metadata map[string]any) (map[string]struct{}, bool) {
	value, ok := metadata["supported_parameters"]
	if !ok {
		return nil, false
	}
	parameters := map[string]struct{}{}
	switch values := value.(type) {
	case []string:
		for _, value := range values {
			if value = strings.ToLower(strings.TrimSpace(value)); value != "" {
				parameters[value] = struct{}{}
			}
		}
	case []any:
		for _, value := range values {
			text, ok := value.(string)
			if !ok {
				return nil, false
			}
			if text = strings.ToLower(strings.TrimSpace(text)); text != "" {
				parameters[text] = struct{}{}
			}
		}
	default:
		return nil, false
	}
	return parameters, true
}
