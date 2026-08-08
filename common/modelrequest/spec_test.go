package modelrequest

import (
	"encoding/json"
	"testing"
)

func TestParseJSONDerivesOpenRouterProviderFromInventoryRow(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"catalog_snapshot_id":"snapshot-1",
		"openrouter_model_id":"deepseek/deepseek-v4-flash",
		"endpoint_tag":"deepinfra/fp4",
		"provider_name":"DeepInfra",
		"quantization":"fp4",
		"raw_endpoint_sha256":"abc123",
		"equivalence_key":{"openrouter_model_id":"deepseek/deepseek-v4-flash","quantization":"fp4"},
		"equivalence_class_size":2,
		"representative_endpoint_variant_id":"variant-1",
		"equivalent_endpoints":[{"provider_name":"DeepInfra"},{"provider_name":"Novita"}],
		"request":{"temperature":0,"top_p":1,"max_tokens":1024},
		"persona":{"id":"d715074-5","path":"personas/persons/d715074-5.txt"}
	}`)
	spec, err := ParseJSON(raw)
	if err != nil {
		t.Fatalf("ParseJSON error = %v", err)
	}
	if spec.RuntimeModel() != "openrouter://deepseek/deepseek-v4-flash" {
		t.Fatalf("RuntimeModel = %q", spec.RuntimeModel())
	}
	if spec.Persona != "personas/persons/d715074-5.txt" {
		t.Fatalf("Persona = %q", spec.Persona)
	}
	provider := spec.ProviderBody()
	if got := provider["only"].([]string)[0]; got != "deepinfra/fp4" {
		t.Fatalf("provider.only[0] = %q", got)
	}
	if got := provider["allow_fallbacks"]; got != false {
		t.Fatalf("allow_fallbacks = %#v, want false", got)
	}
	if got := provider["require_parameters"]; got != true {
		t.Fatalf("require_parameters = %#v, want true", got)
	}
	if got := provider["quantizations"].([]string)[0]; got != "fp4" {
		t.Fatalf("provider.quantizations[0] = %q", got)
	}
	if spec.Request.Temperature == nil || *spec.Request.Temperature != 0 {
		t.Fatalf("temperature = %v, want 0", spec.Request.Temperature)
	}
	if spec.Request.TopP == nil || *spec.Request.TopP != 1 {
		t.Fatalf("top_p = %v, want 1", spec.Request.TopP)
	}
	if spec.MaxOutputTokens() == nil || *spec.MaxOutputTokens() != 1024 {
		t.Fatalf("MaxOutputTokens = %v, want 1024", spec.MaxOutputTokens())
	}
	if spec.Headers[openRouterMetadataHeader] != "enabled" {
		t.Fatalf("metadata header = %q", spec.Headers[openRouterMetadataHeader])
	}
	if spec.VariantMetadata["raw_endpoint_sha256"] != "abc123" {
		t.Fatalf("variant metadata did not preserve raw_endpoint_sha256: %#v", spec.VariantMetadata)
	}
	equivalenceClassSize, ok := spec.VariantMetadata["equivalence_class_size"].(json.Number)
	if !ok || equivalenceClassSize.String() != "2" {
		t.Fatalf("variant metadata did not preserve equivalence_class_size: %#v", spec.VariantMetadata)
	}
	if spec.VariantMetadata["representative_endpoint_variant_id"] != "variant-1" {
		t.Fatalf("variant metadata did not preserve representative_endpoint_variant_id: %#v", spec.VariantMetadata)
	}
}

func TestParseJSONOmitsUnknownQuantizationConstraint(t *testing.T) {
	t.Parallel()

	spec, err := ParseJSON([]byte(`{
		"openrouter_model_id":"deepseek/deepseek-v4-flash",
		"endpoint_tag":"alibaba",
		"quantization":"unknown",
		"persona":"p.txt"
	}`))
	if err != nil {
		t.Fatalf("ParseJSON error = %v", err)
	}
	provider := spec.ProviderBody()
	if provider == nil {
		t.Fatalf("ProviderBody = nil")
	}
	if _, ok := provider["quantizations"]; ok {
		raw, _ := json.Marshal(provider)
		t.Fatalf("ProviderBody includes unknown quantization: %s", raw)
	}
}

func TestParseJSONAcceptsExplicitProvider(t *testing.T) {
	t.Parallel()

	spec, err := ParseJSON([]byte(`{
		"endpoint":"openrouter",
		"model":"meta-llama/llama-3.3-70b-instruct",
		"provider":{"only":["deepinfra/turbo"],"allow_fallbacks":false,"require_parameters":true,"quantizations":["bf16","unknown"]},
		"headers":{"X-Test":"ok"},
		"persona":"p.txt"
	}`))
	if err != nil {
		t.Fatalf("ParseJSON error = %v", err)
	}
	provider := spec.ProviderBody()
	quantizations := provider["quantizations"].([]string)
	if len(quantizations) != 1 || quantizations[0] != "bf16" {
		t.Fatalf("quantizations = %#v, want [bf16]", quantizations)
	}
	if spec.Headers["X-Test"] != "ok" || spec.Headers[openRouterMetadataHeader] != "enabled" {
		t.Fatalf("headers = %#v", spec.Headers)
	}
}

func TestParseJSONEmptyProviderDisablesOpenRouterDerivation(t *testing.T) {
	t.Parallel()

	spec, err := ParseJSON([]byte(`{
		"openrouter_model_id":"deepseek/deepseek-r1",
		"endpoint_tag":"novita/fp8",
		"provider_name":"Novita",
		"quantization":"fp8",
		"provider":{}
	}`))
	if err != nil {
		t.Fatalf("ParseJSON error = %v", err)
	}
	if provider := spec.ProviderBody(); provider != nil {
		raw, _ := json.Marshal(provider)
		t.Fatalf("ProviderBody = %s, want nil", raw)
	}
	if spec.RuntimeModel() != "openrouter://deepseek/deepseek-r1" {
		t.Fatalf("RuntimeModel = %q", spec.RuntimeModel())
	}
}

func TestParseJSONRejectsEndpointModelString(t *testing.T) {
	t.Parallel()

	_, err := ParseJSON([]byte(`{"endpoint":"openrouter","model":"openrouter://openai/gpt-5","persona":"p.txt"}`))
	if err == nil {
		t.Fatalf("ParseJSON accepted endpoint-prefixed model")
	}
	if err.Error() != "request spec model must not include endpoint:// prefix" {
		t.Fatalf("error = %v", err)
	}
}
