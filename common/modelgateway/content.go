package modelgateway

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type dataContent struct {
	mediaType string
	data      string
}

func parseDataURL(raw string) (dataContent, error) {
	raw = strings.TrimSpace(raw)
	prefix, data, ok := strings.Cut(raw, ",")
	if !ok || !strings.HasPrefix(prefix, "data:") || !strings.HasSuffix(strings.ToLower(prefix), ";base64") {
		return dataContent{}, fmt.Errorf("content must be a base64 data URL")
	}
	mediaType := strings.TrimSuffix(strings.TrimPrefix(prefix, "data:"), ";base64")
	mediaType = strings.TrimSpace(mediaType)
	if mediaType == "" {
		return dataContent{}, fmt.Errorf("data URL media type is required")
	}
	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return dataContent{}, fmt.Errorf("decode data URL: %w", err)
	}
	return dataContent{mediaType: mediaType, data: data}, nil
}

func contentItems(value any) ([]map[string]any, error) {
	switch value := value.(type) {
	case []map[string]any:
		return value, nil
	case []any:
		out := make([]map[string]any, 0, len(value))
		for _, entry := range value {
			item, ok := entry.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("content item must be an object")
			}
			out = append(out, item)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("content items must be an array")
	}
}

func copyObject(value any) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("value must be an object")
	}
	wire, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	var copied map[string]any
	if err := json.Unmarshal(wire, &copied); err != nil {
		return nil, err
	}
	return copied, nil
}

func newResponseID(prefix string) (string, error) {
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate response id: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(raw), nil
}
