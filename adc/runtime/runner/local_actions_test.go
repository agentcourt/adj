package runner

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jsmorph/adj/adc/runtime/lean"
)

func TestNextExhibitIDForParty(t *testing.T) {
	t.Parallel()

	caseObj := map[string]any{
		"file_events": []any{
			map[string]any{
				"action":  "offer_case_file_as_exhibit",
				"actor":   "plaintiff",
				"file_id": "file-0001",
			},
			map[string]any{
				"action":  "offer_case_file_as_exhibit",
				"actor":   "plaintiff",
				"file_id": "file-0002",
			},
			map[string]any{
				"action":  "offer_case_file_as_exhibit",
				"actor":   "defendant",
				"file_id": "file-0003",
			},
		},
	}

	if got := nextExhibitIDForParty(caseObj, "plaintiff"); got != "PX-3" {
		t.Fatalf("next plaintiff exhibit id = %q, want %q", got, "PX-3")
	}
	if got := nextExhibitIDForParty(caseObj, "defendant"); got != "DX-2" {
		t.Fatalf("next defendant exhibit id = %q, want %q", got, "DX-2")
	}
	if !caseFileAlreadyOfferedByParty(caseObj, "plaintiff", "file-0001") {
		t.Fatalf("expected file-0001 to count as already offered by plaintiff")
	}
	if caseFileAlreadyOfferedByParty(caseObj, "defendant", "file-0001") {
		t.Fatalf("did not expect file-0001 to count as already offered by defendant")
	}
}

func TestOfferCaseFileAsExhibitPayloadCarriesFileEventFields(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	requestPath := filepath.Join(tmpDir, "request.json")
	enginePath := filepath.Join(tmpDir, "engine.sh")
	script := `#!/bin/sh
cat > "$1"
printf '%s\n' '{"ok":true,"state":{"case":{"case_files":[{"file_id":"file-0001","original_name":"record.txt"}],"file_events":[]}}}'
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write engine script: %v", err)
	}
	r := &Runner{
		lean: lean.New([]string{enginePath, requestPath}),
		state: map[string]any{
			"case": map[string]any{
				"case_files": []any{
					map[string]any{
						"file_id":       "file-0001",
						"original_name": "record.txt",
					},
				},
				"file_events": []any{},
			},
		},
	}

	res, handled, err := r.executeLocalAction("plaintiff", "offer_case_file_as_exhibit", map[string]any{
		"file_id":    "file-0001",
		"exhibit_id": "PX-7",
		"admitted":   false,
	})
	if err != nil {
		t.Fatalf("executeLocalAction returned error: %v", err)
	}
	if !handled {
		t.Fatalf("offer_case_file_as_exhibit was not handled")
	}
	if ok, _ := res.Result["ok"].(bool); !ok {
		t.Fatalf("result = %#v", res.Result)
	}
	raw, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("read engine request: %v", err)
	}
	var req map[string]any
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	action, _ := req["action"].(map[string]any)
	payload, _ := action["payload"].(map[string]any)
	if action["action_type"] != "offer_exhibit" || action["actor_role"] != "plaintiff" {
		t.Fatalf("action = %#v", action)
	}
	if payload["file_id"] != "file-0001" || payload["party"] != "plaintiff" || payload["exhibit_id"] != "PX-7" || payload["admitted"] != false {
		t.Fatalf("payload = %#v", payload)
	}
	if _, err := time.Parse(time.RFC3339, stringOrDefault(payload["offered_at"], "")); err != nil {
		t.Fatalf("offered_at = %#v: %v", payload["offered_at"], err)
	}
}

func TestJurorContextPayloadFiltersPrivateFields(t *testing.T) {
	t.Parallel()

	caseObj := map[string]any{
		"jurors": []any{
			map[string]any{
				"juror_id":         "J1",
				"name":             "Juror One",
				"status":           "candidate",
				"note":             "private",
				"model":            "gpt-x",
				"persona_filename": "j1.txt",
			},
			map[string]any{
				"juror_id": "J2",
				"name":     "Juror Two",
				"status":   "candidate",
			},
		},
		"juror_questionnaire": []any{map[string]any{"question_id": "q1", "question": "Can you follow instructions?"}},
		"juror_questionnaire_responses": []any{
			map[string]any{"juror_id": "J1", "answers": []any{map[string]any{"question_id": "q1", "answer": "yes"}}},
			map[string]any{"juror_id": "J2", "answers": []any{map[string]any{"question_id": "q1", "answer": "no"}}},
		},
		"voir_dire_exchanges": []any{
			map[string]any{"juror_id": "J1", "question": "q1"},
			map[string]any{"juror_id": "J2", "question": "q2"},
		},
		"for_cause_challenges": []any{
			map[string]any{"juror_id": "J1", "grounds": "g1"},
			map[string]any{"juror_id": "J2", "grounds": "g2"},
		},
	}

	ctx := jurorContextPayload(caseObj, "J1")
	juror, _ := ctx["juror"].(map[string]any)
	if juror == nil {
		t.Fatalf("missing juror payload")
	}
	if _, ok := juror["model"]; ok {
		t.Fatalf("juror payload leaked model: %#v", juror)
	}
	if _, ok := juror["persona_filename"]; ok {
		t.Fatalf("juror payload leaked persona filename: %#v", juror)
	}
	if _, ok := ctx["jurors"]; ok {
		t.Fatalf("juror context leaked full juror list: %#v", ctx)
	}
	if got := juror["juror_id"]; got != "J1" {
		t.Fatalf("juror_id = %#v, want J1", got)
	}
	if got := len(ctx["juror_questionnaire_responses"].([]any)); got != 1 {
		t.Fatalf("filtered questionnaire response count = %d, want 1", got)
	}
	if got := len(ctx["voir_dire_exchanges"].([]any)); got != 1 {
		t.Fatalf("filtered voir dire exchange count = %d, want 1", got)
	}
	if got := len(ctx["for_cause_challenges"].([]any)); got != 1 {
		t.Fatalf("filtered for-cause challenge count = %d, want 1", got)
	}
}

func TestUploadedCaseFilePayload(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"original_name":  "memo.txt",
		"label":          "Memo",
		"content_base64": base64.StdEncoding.EncodeToString([]byte("hello")),
	}
	name, label, raw, err := uploadedCaseFilePayload(payload)
	if err != nil {
		t.Fatalf("uploadedCaseFilePayload returned error: %v", err)
	}
	if name != "memo.txt" {
		t.Fatalf("name = %q, want memo.txt", name)
	}
	if label != "Memo" {
		t.Fatalf("label = %q, want Memo", label)
	}
	if string(raw) != "hello" {
		t.Fatalf("raw = %q, want hello", string(raw))
	}
}

func TestStoreUploadedCaseFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	r := &Runner{cfg: Config{OutputPath: filepath.Join(tmpDir, "run.json")}}
	absPath, storedName, err := r.storeUploadedCaseFile("file-0007", "memo.txt", []byte("hello"))
	if err != nil {
		t.Fatalf("storeUploadedCaseFile returned error: %v", err)
	}
	if storedName != "file-0007-memo.txt" {
		t.Fatalf("storedName = %q, want file-0007-memo.txt", storedName)
	}
	if filepath.Dir(absPath) != filepath.Join(tmpDir, "uploaded-case-files") {
		t.Fatalf("stored dir = %q", filepath.Dir(absPath))
	}
	raw, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("read stored upload: %v", err)
	}
	if string(raw) != "hello" {
		t.Fatalf("stored contents = %q, want hello", string(raw))
	}
}

func TestIsReadableCaseTextExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		extension string
		want      bool
	}{
		{name: "markdown", extension: ".md", want: true},
		{name: "text", extension: ".txt", want: true},
		{name: "pem", extension: ".pem", want: true},
		{name: "base64", extension: ".b64", want: true},
		{name: "signature", extension: ".sig", want: false},
		{name: "empty", extension: "", want: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isReadableCaseTextExtension(tt.extension); got != tt.want {
				t.Fatalf("isReadableCaseTextExtension(%q) = %v, want %v", tt.extension, got, tt.want)
			}
		})
	}
}
