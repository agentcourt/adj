package runner

func mergeLocalCaseExtensions(prev map[string]any, next map[string]any) map[string]any {
	prevCase, _ := prev["case"].(map[string]any)
	nextCase, _ := next["case"].(map[string]any)
	if prevCase == nil || nextCase == nil {
		return next
	}
	keys := []string{
		"case_files",
		"file_events",
	}
	for _, key := range keys {
		prevVal, prevOK := prevCase[key]
		nextVal, nextOK := nextCase[key]
		if !prevOK {
			continue
		}
		if !nextOK || nextVal == nil {
			nextCase[key] = prevVal
		}
	}
	next["case"] = nextCase
	return next
}
