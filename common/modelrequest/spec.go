package modelrequest

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const openRouterMetadataHeader = "X-OpenRouter-Experimental-Metadata"

type ProviderConstraints struct {
	Only              []string `json:"only,omitempty"`
	AllowFallbacks    *bool    `json:"allow_fallbacks,omitempty"`
	RequireParameters *bool    `json:"require_parameters,omitempty"`
	Quantizations     []string `json:"quantizations,omitempty"`
}

type RequestParameters struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"top_p,omitempty"`
	MaxTokens       *int64   `json:"max_tokens,omitempty"`
	MaxOutputTokens *int64   `json:"max_output_tokens,omitempty"`
}

type Spec struct {
	Endpoint        string               `json:"endpoint,omitempty"`
	Model           string               `json:"model,omitempty"`
	Provider        *ProviderConstraints `json:"provider,omitempty"`
	Request         RequestParameters    `json:"request,omitempty"`
	Headers         map[string]string    `json:"headers,omitempty"`
	Persona         string               `json:"persona,omitempty"`
	VariantMetadata map[string]any       `json:"variant_metadata,omitempty"`
}

type ModelRef struct {
	Endpoint string
	Model    string
	Query    string
}

func ParseModelRef(model string) (ModelRef, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return ModelRef{}, errors.New("model must be non-empty string")
	}
	endpoint, rest, ok := strings.Cut(model, "://")
	if !ok {
		return ModelRef{}, fmt.Errorf("model %q must be endpoint://model", model)
	}
	endpoint = strings.TrimSpace(endpoint)
	rest = strings.TrimSpace(rest)
	if endpoint == "" {
		return ModelRef{}, fmt.Errorf("model %q has empty endpoint", model)
	}
	if rest == "" {
		return ModelRef{}, fmt.Errorf("model %q has empty model", model)
	}
	if strings.Contains(endpoint, " ") || strings.ContainsAny(endpoint, "/?#") {
		return ModelRef{}, fmt.Errorf("model %q has invalid endpoint %q", model, endpoint)
	}
	if strings.Contains(rest, "#") {
		return ModelRef{}, fmt.Errorf("model %q must not include a fragment", model)
	}
	modelID, query, _ := strings.Cut(rest, "?")
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ModelRef{}, fmt.Errorf("model %q has empty model id", model)
	}
	if strings.ContainsAny(modelID, " \t\r\n") {
		return ModelRef{}, fmt.Errorf("model %q has whitespace in model id", model)
	}
	return ModelRef{Endpoint: endpoint, Model: modelID, Query: query}, nil
}

func ParseJSON(data []byte) (Spec, error) {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		return Spec{}, err
	}
	return ParseMap(raw)
}

func ParseMap(raw map[string]any) (Spec, error) {
	if len(raw) == 0 {
		return Spec{}, errors.New("request spec JSON object is empty")
	}
	endpoint := stringField(raw, "endpoint")
	model := stringField(raw, "model")
	openRouterModelID := stringField(raw, "openrouter_model_id")
	if model == "" {
		model = openRouterModelID
	}
	if endpoint == "" && openRouterModelID != "" {
		endpoint = "openrouter"
	}
	if endpoint == "" {
		return Spec{}, errors.New("request spec endpoint is required")
	}
	if model == "" {
		return Spec{}, errors.New("request spec model is required")
	}
	if strings.Contains(model, "://") {
		return Spec{}, errors.New("request spec model must not include endpoint:// prefix")
	}
	out := Spec{
		Endpoint:        strings.TrimSpace(endpoint),
		Model:           strings.TrimSpace(model),
		Persona:         personaField(raw),
		Headers:         headersFromRaw(raw["headers"]),
		VariantMetadata: variantMetadata(raw),
	}
	if provider, ok, err := providerFromRaw(raw["provider"]); err != nil {
		return Spec{}, err
	} else if ok {
		out.Provider = provider
	} else if out.Endpoint == "openrouter" {
		out.Provider = deriveOpenRouterProvider(raw)
	}
	out.Request = requestFromRaw(raw)
	if out.Endpoint == "openrouter" && len(out.Headers) == 0 {
		out.Headers = map[string]string{openRouterMetadataHeader: "enabled"}
	} else if out.Endpoint == "openrouter" {
		if _, ok := headerLookup(out.Headers, openRouterMetadataHeader); !ok {
			out.Headers[openRouterMetadataHeader] = "enabled"
		}
	}
	if _, err := ParseModelRef(out.RuntimeModel()); err != nil {
		return Spec{}, fmt.Errorf("invalid request model %q: %w", out.RuntimeModel(), err)
	}
	return out, nil
}

func (s Spec) RuntimeModel() string {
	model := strings.TrimSpace(s.Model)
	if strings.Contains(model, "://") {
		return model
	}
	endpoint := strings.TrimSpace(s.Endpoint)
	if endpoint == "" {
		return model
	}
	return endpoint + "://" + model
}

func (s Spec) UpstreamModel() string {
	model := strings.TrimSpace(s.Model)
	if strings.Contains(model, "://") {
		parsed, err := ParseModelRef(model)
		if err == nil {
			return parsed.Model
		}
	}
	if strings.Contains(model, "?") {
		model, _, _ = strings.Cut(model, "?")
	}
	return model
}

func (s Spec) ProviderBody() map[string]any {
	if s.Provider == nil {
		return nil
	}
	body := map[string]any{}
	if len(s.Provider.Only) > 0 {
		body["only"] = append([]string(nil), s.Provider.Only...)
	}
	if s.Provider.AllowFallbacks != nil {
		body["allow_fallbacks"] = *s.Provider.AllowFallbacks
	}
	if s.Provider.RequireParameters != nil {
		body["require_parameters"] = *s.Provider.RequireParameters
	}
	if len(s.Provider.Quantizations) > 0 {
		body["quantizations"] = append([]string(nil), s.Provider.Quantizations...)
	}
	if len(body) == 0 {
		return nil
	}
	return body
}

func (s Spec) MaxOutputTokens() *int64 {
	if s.Request.MaxOutputTokens != nil {
		return s.Request.MaxOutputTokens
	}
	return s.Request.MaxTokens
}

func (s Spec) WithFallbackMaxOutputTokens(max int64) Spec {
	if max > 0 && s.MaxOutputTokens() == nil {
		s.Request.MaxOutputTokens = &max
	}
	return s
}

func stringField(raw map[string]any, key string) string {
	value, _ := raw[key].(string)
	return strings.TrimSpace(value)
}

func firstStringField(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringField(raw, key); value != "" {
			return value
		}
	}
	return ""
}

func personaField(raw map[string]any) string {
	if value := stringField(raw, "persona_file"); value != "" {
		return value
	}
	switch value := raw["persona"].(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		return firstStringField(value, "path", "file", "persona_file")
	default:
		return ""
	}
}

func boolPtr(value bool) *bool { return &value }

func providerFromRaw(value any) (*ProviderConstraints, bool, error) {
	obj, ok := value.(map[string]any)
	if !ok || obj == nil {
		return nil, false, nil
	}
	provider := &ProviderConstraints{
		Only:          stringList(obj["only"]),
		Quantizations: cleanQuantizations(stringList(obj["quantizations"])),
	}
	if value, ok := boolField(obj, "allow_fallbacks"); ok {
		provider.AllowFallbacks = &value
	}
	if value, ok := boolField(obj, "require_parameters"); ok {
		provider.RequireParameters = &value
	}
	return provider, true, nil
}

func deriveOpenRouterProvider(raw map[string]any) *ProviderConstraints {
	only := firstStringField(raw, "endpoint_tag", "provider_tag", "selected_provider_or_endpoint")
	if only == "" {
		only = stringField(raw, "provider_name")
	}
	if only == "" {
		return nil
	}
	provider := &ProviderConstraints{
		Only:              []string{only},
		AllowFallbacks:    boolPtr(false),
		RequireParameters: boolPtr(true),
	}
	quantization := strings.ToLower(stringField(raw, "quantization"))
	if quantization != "" && quantization != "unknown" {
		provider.Quantizations = []string{quantization}
	}
	return provider
}

func requestFromRaw(raw map[string]any) RequestParameters {
	request := RequestParameters{}
	if obj, ok := raw["request"].(map[string]any); ok {
		applyRequestFields(&request, obj)
	}
	applyRequestFields(&request, raw)
	return request
}

func applyRequestFields(request *RequestParameters, raw map[string]any) {
	if value, ok := floatField(raw, "temperature"); ok {
		request.Temperature = &value
	}
	if value, ok := floatField(raw, "top_p"); ok {
		request.TopP = &value
	}
	if value, ok := int64Field(raw, "max_tokens"); ok {
		request.MaxTokens = &value
	}
	if value, ok := int64Field(raw, "max_output_tokens"); ok {
		request.MaxOutputTokens = &value
	}
}

func headersFromRaw(value any) map[string]string {
	obj, ok := value.(map[string]any)
	if !ok || len(obj) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range obj {
		if str, ok := value.(string); ok && strings.TrimSpace(key) != "" && strings.TrimSpace(str) != "" {
			out[strings.TrimSpace(key)] = strings.TrimSpace(str)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func headerLookup(headers map[string]string, key string) (string, bool) {
	for name, value := range headers {
		if strings.EqualFold(name, key) {
			return value, true
		}
	}
	return "", false
}

func variantMetadata(raw map[string]any) map[string]any {
	keys := []string{
		"catalog_snapshot_id", "snapshot_timestamp_utc", "openrouter_model_id", "canonical_slug",
		"endpoint_index", "endpoint_variant_id", "endpoint_variant_key", "provider_name", "endpoint_name",
		"endpoint_tag", "endpoint_id", "endpoint_model_id", "endpoint_model_name", "endpoint_model_permaslug",
		"quantization", "unknown_quantization_endpoint_variant", "context_length", "max_prompt_tokens",
		"max_completion_tokens", "supported_parameters", "model_supported_parameters", "endpoint_raw_path",
		"raw_endpoint_sha256", "model_raw_path", "raw_model_sha256", "equivalence_key",
		"equivalence_class_size", "representative_source_row", "representative_endpoint_variant_id",
		"equivalent_endpoints",
	}
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func boolField(raw map[string]any, key string) (bool, bool) {
	value, ok := raw[key]
	if !ok {
		return false, false
	}
	b, ok := value.(bool)
	return b, ok
}

func floatField(raw map[string]any, key string) (float64, bool) {
	value, ok := raw[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case float64:
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func int64Field(raw map[string]any, key string) (int64, bool) {
	value, ok := raw[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		if float64(int64(v)) == v {
			return int64(v), true
		}
	case json.Number:
		n, err := v.Int64()
		return n, err == nil
	}
	return 0, false
}

func stringList(value any) []string {
	switch v := value.(type) {
	case []string:
		return compactStrings(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return compactStrings(out)
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{strings.TrimSpace(v)}
	default:
		return nil
	}
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cleanQuantizations(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && value != "unknown" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func EscapeModel(model string) string {
	return url.PathEscape(model)
}
