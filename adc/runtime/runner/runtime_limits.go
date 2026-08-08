package runner

const (
	DefaultLLMTimeoutSeconds     = 180
	DefaultRoleAPITimeoutSeconds = 480
	DefaultMaxResponseBytes      = 128 * 1024
	DefaultInvalidAttemptLimit   = 3
	DefaultJurorMaxOutputTokens  = 4096
)

type RuntimeLimits struct {
	LLMTimeoutSeconds     int `json:"llm_timeout_seconds"`
	RoleAPITimeoutSeconds int `json:"roleapi_timeout_seconds"`
	MaxResponseBytes      int `json:"max_response_bytes"`
	InvalidAttemptLimit   int `json:"invalid_attempt_limit"`
	JurorMaxOutputTokens  int `json:"juror_max_output_tokens"`
}

func (limits RuntimeLimits) Normalized() RuntimeLimits {
	if limits.LLMTimeoutSeconds <= 0 {
		limits.LLMTimeoutSeconds = DefaultLLMTimeoutSeconds
	}
	if limits.RoleAPITimeoutSeconds <= 0 {
		limits.RoleAPITimeoutSeconds = DefaultRoleAPITimeoutSeconds
	}
	if limits.MaxResponseBytes <= 0 {
		limits.MaxResponseBytes = DefaultMaxResponseBytes
	}
	if limits.InvalidAttemptLimit <= 0 {
		limits.InvalidAttemptLimit = DefaultInvalidAttemptLimit
	}
	if limits.JurorMaxOutputTokens <= 0 {
		limits.JurorMaxOutputTokens = DefaultJurorMaxOutputTokens
	}
	return limits
}
