package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentcourt/adj/adc/runtime/runner"
	"github.com/agentcourt/adj/common/modelrequest"
	"github.com/agentcourt/adj/common/openai"
	"github.com/agentcourt/adj/common/persona"
)

const llmToolCheckName = "submit_juror_vote"

func RunLLM(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("llm", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc llm --prompt <text> [options]\n\n")
		fs.PrintDefaults()
	})
	prompt := fs.String("prompt", "", "Prompt text")
	inputFile := fs.String("input-file", "", "Path to prompt text file")
	promptDir := fs.String("prompt-dir", "", "ADC prompt catalog directory")
	var promptFiles promptFileFlag
	model := fs.String("model", "openrouter://openai/gpt-5", "Model name in endpoint://model form")
	personaRecord := fs.String("persona", "", `Persona record in PROVIDER://MODEL,path/to/persona.txt form, or "random" to sample from the shared personas file`)
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM HTTP timeout in seconds")
	toolCheck := fs.Bool("tool-check", false, "Require one submit_juror_vote tool call and print its arguments")
	fs.Var(&promptFiles, "prompt-file", "ADC prompt override as ID=PATH; repeat as needed")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc llm accepts no positional arguments")
	}
	promptText, err := loadPromptText(strings.TrimSpace(*prompt), strings.TrimSpace(*inputFile))
	if err != nil {
		return err
	}
	if strings.TrimSpace(promptText) == "" {
		return fmt.Errorf("--prompt or --input-file is required")
	}
	promptRenderer, err := newProbePromptRenderer(*promptDir, promptFiles)
	if err != nil {
		return err
	}
	modelName := strings.TrimSpace(*model)
	var requestSpec *modelrequest.Spec
	systemPrompt := ""
	if strings.TrimSpace(*personaRecord) != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get cwd: %w", err)
		}
		spec, sampled, err := resolveLLMPersonaSpec(strings.TrimSpace(*personaRecord), cwd)
		if err != nil {
			return fmt.Errorf("parse --persona: %w", err)
		}
		modelName = spec.Model
		if spec.RequestSpec != nil {
			copied := *spec.RequestSpec
			requestSpec = &copied
			modelName = copied.RuntimeModel()
		}
		systemPrompt, err = promptRenderer.RenderJurorProbeIdentity(spec.Text)
		if err != nil {
			return err
		}
		if sampled {
			if _, err := fmt.Fprintln(stdout, spec.File); err != nil {
				return err
			}
		}
	}
	if modelName == "" {
		return fmt.Errorf("--model is required")
	}
	modelRef, err := modelrequest.ParseModelRef(modelName)
	if err != nil {
		return fmt.Errorf("parse --model: %w", err)
	}
	endpoint := modelRef.Endpoint
	if requestSpec != nil {
		endpoint = requestSpec.Endpoint
	}
	client, err := openai.NewForEndpoint(endpoint, false, time.Duration(*timeoutSeconds)*time.Second)
	if err != nil {
		return err
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(*timeoutSeconds)*time.Second)
	defer cancel()
	input := make([]map[string]any, 0, 2)
	if systemPrompt != "" {
		input = append(input, map[string]any{"role": "system", "content": systemPrompt})
	}
	if *toolCheck {
		toolCheckPrompt, err := promptRenderer.JurorProbeToolCheck()
		if err != nil {
			return err
		}
		input = append(input, map[string]any{
			"role":    "system",
			"content": toolCheckPrompt,
		})
	}
	input = append(input, map[string]any{"role": "user", "content": promptText})
	tools := []map[string]any(nil)
	if *toolCheck {
		tools, err = llmToolCheckTools(promptRenderer)
		if err != nil {
			return err
		}
	}
	var resp openai.Response
	if requestSpec != nil {
		resp, err = client.CreateResponseWithRequestSpec(requestCtx, *requestSpec, input, tools, "")
	} else {
		resp, err = client.CreateResponse(requestCtx, modelRef.Model, input, tools, "", nil)
	}
	if err != nil {
		return err
	}
	output := strings.TrimSpace(resp.Text)
	if *toolCheck {
		output, err = extractToolCheckArguments(resp)
		if err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(stdout, output); err != nil {
		return err
	}
	return nil
}

func llmToolCheckTools(promptRenderer *runner.PromptRenderer) ([]map[string]any, error) {
	return promptRenderer.BuildTools([]string{llmToolCheckName})
}

func extractToolCheckArguments(resp openai.Response) (string, error) {
	if len(resp.ToolCalls) != 1 {
		return "", fmt.Errorf("model did not call required tool %s", llmToolCheckName)
	}
	call := resp.ToolCalls[0]
	if strings.TrimSpace(call.Name) != llmToolCheckName {
		return "", fmt.Errorf("model called %s, want %s", strings.TrimSpace(call.Name), llmToolCheckName)
	}
	if strings.TrimSpace(call.ArgumentsError) != "" {
		return "", fmt.Errorf("required tool %s returned malformed arguments: %s", llmToolCheckName, call.ArgumentsError)
	}
	if strings.TrimSpace(stringArg(call.Arguments, "juror_id")) == "" {
		return "", fmt.Errorf("required tool %s missing juror_id", llmToolCheckName)
	}
	if strings.TrimSpace(stringArg(call.Arguments, "vote")) == "" {
		return "", fmt.Errorf("required tool %s missing vote", llmToolCheckName)
	}
	if strings.TrimSpace(stringArg(call.Arguments, "confidence")) == "" {
		return "", fmt.Errorf("required tool %s missing confidence", llmToolCheckName)
	}
	if strings.TrimSpace(stringArg(call.Arguments, "explanation")) == "" {
		return "", fmt.Errorf("required tool %s missing explanation", llmToolCheckName)
	}
	if _, ok := call.Arguments["damages"]; !ok {
		return "", fmt.Errorf("required tool %s missing damages", llmToolCheckName)
	}
	encoded, err := json.Marshal(call.Arguments)
	if err != nil {
		return "", fmt.Errorf("marshal tool arguments: %w", err)
	}
	return string(encoded), nil
}

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return value
}

func resolveLLMPersonaSpec(record string, cwd string) (persona.Spec, bool, error) {
	record = strings.TrimSpace(record)
	if record == "" {
		return persona.Spec{}, false, nil
	}
	if record == "random" {
		spec, err := persona.SampleRecordFile(defaultPersonaRecordsPathFor(cwd), cwd)
		if err != nil {
			return persona.Spec{}, false, err
		}
		return spec, true, nil
	}
	spec, err := persona.ParseRecord(record, cwd)
	if err == nil {
		return spec, false, nil
	}
	spec, fallbackErr := persona.ParseRecord(record, filepath.Join(cwd, "etc"))
	if fallbackErr == nil {
		return spec, false, nil
	}
	return persona.Spec{}, false, err
}
