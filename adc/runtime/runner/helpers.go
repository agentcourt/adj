package runner

import (
	"encoding/json"
	"sort"
	"strings"
)

func marshalString(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func appendIfMissing(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func sortedKeys(items map[string]bool) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}

func stringOrDefault(v any, fallback string) string {
	s, _ := v.(string)
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func floatFromAny(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	default:
		return 0
	}
}

func citationEquivalent(actual string, expected string) bool {
	actualNorm := normalizeCitation(actual)
	expectedNorm := normalizeCitation(expected)
	if actualNorm == expectedNorm {
		return true
	}
	return strings.HasPrefix(actualNorm, expectedNorm) || strings.HasPrefix(expectedNorm, actualNorm)
}

func normalizeCitation(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	return s
}

func hasCaseFile(caseObj map[string]any, fileID string) bool {
	files, _ := caseObj["case_files"].([]any)
	for _, f := range files {
		obj, _ := f.(map[string]any)
		id, _ := obj["file_id"].(string)
		if id == fileID {
			return true
		}
	}
	return false
}

func findCaseFile(caseObj map[string]any, fileID string) map[string]any {
	files, _ := caseObj["case_files"].([]any)
	for _, f := range files {
		obj, _ := f.(map[string]any)
		id, _ := obj["file_id"].(string)
		if id == fileID {
			return obj
		}
	}
	return nil
}

func caseFileDisplayName(fileObj map[string]any) string {
	label := strings.TrimSpace(stringOrDefault(fileObj["label"], ""))
	originalName := strings.TrimSpace(stringOrDefault(fileObj["original_name"], ""))
	switch {
	case label != "" && originalName != "" && label != originalName:
		return label + " / " + originalName
	case label != "":
		return label
	case originalName != "":
		return originalName
	default:
		return strings.TrimSpace(stringOrDefault(fileObj["file_id"], "case file"))
	}
}

func summarizeCaseFileChoice(fileObj map[string]any) string {
	fileID := strings.TrimSpace(stringOrDefault(fileObj["file_id"], ""))
	if fileID == "" {
		return ""
	}
	name := caseFileDisplayName(fileObj)
	if name == "" || name == fileID {
		return fileID
	}
	return fileID + " (" + name + ")"
}

func stringsFromAny(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, _ := item.(string)
		if strings.TrimSpace(s) == "" {
			continue
		}
		out = append(out, strings.ToLower(strings.TrimSpace(s)))
	}
	return out
}

func ternary[T any](cond bool, yes T, no T) T {
	if cond {
		return yes
	}
	return no
}

func cloneMap(input map[string]any) map[string]any {
	raw, _ := json.Marshal(input)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

func normalizePolicy(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = toInt(value)
	}
	return out
}

func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case int32:
		return int(x)
	case float64:
		return int(x)
	case float32:
		return int(x)
	case json.Number:
		i, err := x.Int64()
		if err == nil {
			return int(i)
		}
		f, err := x.Float64()
		if err == nil {
			return int(f)
		}
		return 0
	case bool:
		if x {
			return 1
		}
		return 0
	default:
		return 0
	}
}
