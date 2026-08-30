package localrun

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/agentcourt/adj/internal/launcherprompt"
	headless "github.com/agentcourt/adj/runtime/agent"
	"github.com/agentcourt/adj/runtime/modelrequest"
)

var pairedCoreBinDir = flag.String("core-bin-dir", "", "Directory containing paired core executables")
var pairedCoreRoot = flag.String("core-root", "", "Paired core checkout root")

func mustADCLauncherPrompts(t *testing.T, overrides map[string]string) launcherprompt.Sources {
	t.Helper()
	prompts, err := launcherprompt.Resolve("adc", "", overrides)
	if err != nil {
		t.Fatal(err)
	}
	return prompts
}

func TestWriteRemoteLawyerSkillReturnsLogFailure(t *testing.T) {
	state := &runState{
		opts: Options{
			CaseID:    "case-1",
			OutputDir: t.TempDir(),
			Log:       failedLocalRunWriter{},
		},
		launcherPrompts: mustADCLauncherPrompts(t, nil),
		mcpPublicBase:   "http://adc.example:8001",
		signingKey:      []byte("01234567890123456789012345678901"),
	}
	if err := state.writeRemoteLawyerSkill("plaintiff"); err == nil || !strings.Contains(err.Error(), "write remote lawyer skill log") {
		t.Fatalf("log error = %v", err)
	}
	path := filepath.Join(state.opts.OutputDir, "openclaw-plaintiff-lawyer-skill.md")
	if err := state.cleanupSecrets(); err != nil {
		t.Fatalf("cleanup secrets: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remote lawyer skill remains after cleanup: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("unchanged\n"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatalf("create skill symlink: %v", err)
	}
	symlinkState := &runState{opts: state.opts, launcherPrompts: state.launcherPrompts, mcpPublicBase: state.mcpPublicBase, signingKey: state.signingKey}
	symlinkState.opts.Log = nil
	if err := symlinkState.writeRemoteLawyerSkill("plaintiff"); err == nil {
		t.Fatal("remote lawyer skill write followed a symlink")
	}
	raw, err := os.ReadFile(outside)
	if err != nil || string(raw) != "unchanged\n" {
		t.Fatalf("outside file = %q, error = %v", raw, err)
	}
	if err := symlinkState.cleanupSecrets(); err != nil {
		t.Fatalf("cleanup rejected skill: %v", err)
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("rejected skill path changed: info=%v error=%v", info, err)
	}
}

func TestStartPiJurorRejectsExistingHome(t *testing.T) {
	outputDir := t.TempDir()
	outside := t.TempDir()
	active := activeJurorOpportunity{principalID: "C1", opportunityID: "opportunity-1", phase: "deliberation"}
	home := filepath.Join(outputDir, jurorProcessName(active))
	if err := os.Symlink(outside, home); err != nil {
		t.Fatalf("create Pi home symlink: %v", err)
	}
	state := &runState{
		opts: Options{
			CaseID:        "case-1",
			OutputDir:     outputDir,
			PodmanMCPHost: "127.0.0.1",
		},
		launcherPrompts: mustADCLauncherPrompts(t, nil),
		signingKey:      []byte("01234567890123456789012345678901"),
	}
	if err := state.startPiJuror(context.Background(), active, "19780"); err == nil || !strings.Contains(err.Error(), "create Pi home") {
		t.Fatalf("symlink Pi home error = %v", err)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside Pi home entries = %#v, error = %v", entries, err)
	}
}

func TestWriteRunSummaryIncludesResolution(t *testing.T) {
	dir := t.TempDir()
	result := Result{
		Scenario: "proposition",
		FinalState: map[string]any{
			"case": map[string]any{
				"status":     "judgment_entered",
				"resolution": "demonstrated",
			},
		},
	}
	if err := writeRunSummary(dir, result, Options{CaseID: "case-1", RunID: "run-1"}); err != nil {
		t.Fatal(err)
	}
	summary := readJSONMap(t, filepath.Join(dir, "local-run.json"))
	if summary["case_status"] != "judgment_entered" || summary["resolution"] != "demonstrated" {
		t.Fatalf("summary = %#v", summary)
	}
	result.FinalState["case"] = map[string]any{"status": "changed", "resolution": "changed"}
	if err := writeRunSummary(dir, result, Options{CaseID: "case-1", RunID: "run-1"}); err == nil {
		t.Fatal("writeRunSummary replaced an existing summary")
	}
	summary = readJSONMap(t, filepath.Join(dir, "local-run.json"))
	if summary["case_status"] != "judgment_entered" || summary["resolution"] != "demonstrated" {
		t.Fatalf("summary changed after rejected replacement: %#v", summary)
	}
}

type failedLocalRunWriter struct{}

func (failedLocalRunWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestCoreCaseArgsUseProcessInterface(t *testing.T) {
	propositionArgs := coreCaseArgs(Options{
		Proposition:            "The sky is blue.",
		DocumentsDir:           "/case/documents",
		EvidenceStandard:       "clear_and_convincing",
		MaxDocumentFiles:       12,
		MaxDocumentFileBytes:   2048,
		MaxDocumentsTotalBytes: 8192,
		OutputDir:              "/out",
		CoreOutputDir:          "/out/adc-output",
		JurorPersonasPath:      "/case/jurors.jsonl",
		TrialMode:              "bench",
		JurorCount:             5,
		MinimumConcurring:      3,
		RunID:                  "run-proposition",
		CaseID:                 "case-proposition",
	}, "127.0.0.1:9000")
	joined := strings.Join(propositionArgs, "\x00")
	for _, want := range []string{
		"case", "--proposition\x00The sky is blue.", "--documents\x00/case/documents",
		"--evidence-standard\x00clear_and_convincing", "--out-dir\x00/out/adc-output",
		"--case-id\x00case-proposition", "--run-id\x00run-proposition",
		"--caseapi-addr\x00127.0.0.1:9000", "--max-document-files\x0012",
		"--max-document-file-bytes\x002048", "--max-documents-total-bytes\x008192",
		"--juror-personas\x00/case/jurors.jsonl", "--trial-mode\x00bench",
		"--juror-count\x005", "--minimum-concurring\x003",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("proposition core args lack %q: %#v", want, propositionArgs)
		}
	}
	for _, absent := range []string{"--complaint", "--scenario"} {
		if strings.Contains(joined, absent) {
			t.Fatalf("proposition core args contain %q: %#v", absent, propositionArgs)
		}
	}
	if got := strings.Count(joined, "--external-role\x00"); got != 3 {
		t.Fatalf("proposition external role count = %d, want 3: %#v", got, propositionArgs)
	}

	complaintArgs := coreCaseArgs(Options{
		ComplaintPath:        "/case/complaint.md",
		OutputDir:            "/out",
		CoreOutputDir:        "/out/adc-output",
		Court:                "court.json",
		Model:                "runtime-model",
		DigestModel:          "report-model",
		NonJurorModel:        "non-juror-model",
		PlaintiffModel:       "plaintiff-model",
		DefendantModel:       "defendant-model",
		JudgeModel:           "judge-model",
		ClerkModel:           "clerk-model",
		PlannerModel:         "planner-model",
		Temperature:          "0.2",
		NonJurorTemperature:  "0.3",
		JurorTemperature:     "0.4",
		JurorPersonasPath:    "/case/jurors.jsonl",
		TrialMode:            "jury",
		SkipVoirDire:         true,
		JurorCount:           8,
		MinimumConcurring:    6,
		UnanimousRequired:    "false",
		Online:               true,
		LawyerTimeoutSeconds: 90,
		JurorTimeoutSeconds:  120,
		TimeoutSeconds:       30,
		MaxResponseBytes:     4096,
		InvalidAttemptLimit:  2,
		EnginePath:           "/bin/adcengine",
		RunID:                "run-1",
		CaseID:               "case-1",
	}, "127.0.0.1:9001")
	joined = strings.Join(complaintArgs, "\x00")
	for _, want := range []string{
		"case", "--complaint\x00/case/complaint.md", "--out-dir\x00/out/adc-output",
		"--case-id\x00case-1", "--run-id\x00run-1", "--caseapi-addr\x00127.0.0.1:9001",
		"--court\x00court.json", "--model\x00runtime-model", "--report-model\x00report-model",
		"--non-juror-model\x00non-juror-model", "--plaintiff-model\x00plaintiff-model",
		"--defendant-model\x00defendant-model", "--judge-model\x00judge-model",
		"--clerk-model\x00clerk-model", "--planner-model\x00planner-model",
		"--temperature\x000.2", "--non-juror-temperature\x000.3", "--juror-temperature\x000.4",
		"--juror-personas\x00/case/jurors.jsonl", "--trial-mode\x00jury", "--skip-voir-dire",
		"--juror-count\x008", "--minimum-concurring\x006", "--unanimous-required\x00false",
		"--roleapi-timeout-seconds\x00120", "--timeout-seconds\x0030",
		"--max-response-bytes\x004096", "--invalid-attempt-limit\x002",
		"--engine\x00/bin/adcengine", "--online",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("complaint core args lack %q: %#v", want, complaintArgs)
		}
	}
	if got := strings.Count(joined, "--external-role\x00"); got != 3 {
		t.Fatalf("complaint external role count = %d, want 3: %#v", got, complaintArgs)
	}

	scenarioArgs := coreCaseArgs(Options{
		ScenarioPath:      "/case/scenario.json",
		OutputDir:         "/out",
		CoreOutputDir:     "/out/adc-output",
		DigestModel:       "report-model",
		Offline:           true,
		RunID:             "run-2",
		CaseID:            "case-2",
		EnginePath:        "/bin/adcengine",
		JurorCount:        6,
		MinimumConcurring: 6,
	}, "127.0.0.1:9002")
	joined = strings.Join(scenarioArgs, "\x00")
	for _, want := range []string{
		"scenario", "--scenario\x00/case/scenario.json", "--output\x00/out/adc-output/run.json",
		"--runtime\x00/out/adc-output/runtime.json", "--events\x00/out/adc-output/events.ndjson",
		"--db\x00/out/adc-output/run.db", "--transcript\x00/out/adc-output/transcript.md", "--digest\x00/out/adc-output/digest.md",
		"--allow-assertion-failures", "--report-model\x00report-model", "--offline",
		"--case-id\x00case-2", "--run-id\x00run-2", "--caseapi-addr\x00127.0.0.1:9002",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("scenario core args lack %q: %#v", want, scenarioArgs)
		}
	}
	if got := strings.Count(joined, "--external-role\x00"); got != 3 {
		t.Fatalf("scenario external role count = %d, want 3: %#v", got, scenarioArgs)
	}
}

func TestStartCoreCaseReadsFreshResult(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	coreOutputDir := filepath.Join(dir, "adc-output")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	core := filepath.Join(dir, "adc-core")
	script := `#!/bin/sh
set -eu
out_dir=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --out-dir) out_dir=$2; shift 2 ;;
    *) shift ;;
  esac
done
mkdir -p "$out_dir"
printf '{"scenario":"fake","assertions":[],"turn_logs":[],"final_state":{"status":"ok"},"extra":"preserved"}\n' > "$out_dir/run.json"
`
	if err := os.WriteFile(core, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake core: %v", err)
	}
	done, err := startCoreCase(context.Background(), Options{
		CoreCommand:   core,
		ComplaintPath: filepath.Join(dir, "complaint.md"),
		OutputDir:     dir,
		CoreOutputDir: coreOutputDir,
		CaseID:        "case-1",
		RunID:         "run-1",
	}, "127.0.0.1:9001", logDir)
	if err != nil {
		t.Fatalf("start core case: %v", err)
	}
	outcome := <-done
	if outcome.err != nil {
		t.Fatalf("core outcome: %v", outcome.err)
	}
	if outcome.result.Scenario != "fake" || outcome.result.FinalState["status"] != "ok" {
		t.Fatalf("result = %#v", outcome.result)
	}
	raw, err := json.Marshal(outcome.result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if !strings.Contains(string(raw), `"extra":"preserved"`) {
		t.Fatalf("marshaled result lost core fields: %s", raw)
	}
}

func TestPairedCoreCaseAPI(t *testing.T) {
	binDir := strings.TrimSpace(*pairedCoreBinDir)
	coreRoot := strings.TrimSpace(*pairedCoreRoot)
	if binDir == "" || coreRoot == "" {
		t.Skip("-core-bin-dir and -core-root are not set")
	}
	coreCommand := filepath.Join(binDir, "adc")
	enginePath := filepath.Join(coreRoot, "adc", "engine", ".lake", "build", "bin", "adcengine")
	for _, path := range []string{coreCommand, enginePath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stat paired core path %s: %v", path, err)
		}
	}
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	coreOutputDir := filepath.Join(dir, "adc-output")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	scenarioPath := filepath.Join(dir, "scenario.json")
	if err := writeJSONFile(scenarioPath, map[string]any{
		"name":       "paired-adc",
		"court_name": "United States District",
		"roles": []map[string]any{{
			"name": "plaintiff", "instructions": "Paired role.", "allowed_actions": []string{"get_case"},
		}},
		"turns": []map[string]any{{
			"role": "plaintiff", "prompt": "Wait for the paired client.", "max_steps": 1,
			"allowed_actions": []string{"get_case"},
		}},
	}); err != nil {
		t.Fatalf("write scenario: %v", err)
	}
	t.Setenv("OPENAI_API_KEY", "paired-key")
	caseAPIAddr, err := resolveListenAddr("127.0.0.1:0", "127.0.0.1")
	if err != nil {
		t.Fatalf("resolve case API address: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	done, err := startCoreCase(ctx, Options{
		CoreCommand:    coreCommand,
		CoreWorkingDir: filepath.Join(coreRoot, "adc"),
		ScenarioPath:   scenarioPath,
		OutputDir:      dir,
		CoreOutputDir:  coreOutputDir,
		EnginePath:     enginePath,
		RunID:          "run-paired-adc",
		CaseID:         "paired-adc",
	}, caseAPIAddr, logDir)
	if err != nil {
		cancel()
		t.Fatalf("start paired core: %v", err)
	}
	baseURL := "http://" + caseAPIAddr
	if err := waitForCaseHealth(ctx, baseURL+"/health", "paired-adc", "run-paired-adc", 20*time.Second); err != nil {
		cancel()
		outcome := <-done
		t.Fatalf("wait for paired core API: %v; core outcome: %v", err, outcome.err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/roleapi/v1/status?case_id=paired-adc&role_id=plaintiff", nil)
	if err != nil {
		cancel()
		t.Fatalf("build status request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("read paired core status: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		cancel()
		t.Fatalf("close paired core status: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("paired core status HTTP = %d", resp.StatusCode)
	}
	completions := launcherCompletions{caseDone: done, casePending: true}
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- completions.shutdown(cancel)
	}()
	select {
	case shutdownErr := <-shutdownDone:
		if shutdownErr == nil {
			t.Fatalf("canceled paired core returned no process error")
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("launcher shutdown did not drain paired core")
	}
}

func TestPrepareOutputLayoutRejectsSymlinkChildren(t *testing.T) {
	for _, child := range []string{"adc-output", "logs"} {
		t.Run(child, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			if err := os.Symlink(outside, filepath.Join(root, child)); err != nil {
				t.Fatalf("create symlink: %v", err)
			}
			err := prepareOutputLayout(root, filepath.Join(root, "adc-output"), filepath.Join(root, "logs"))
			if err == nil {
				t.Fatal("output preparation accepted a symlink child")
			}
		})
	}
}

func TestMCPEndpointHasNoIdentityQuery(t *testing.T) {
	t.Parallel()

	raw := mcpEndpoint("http://127.0.0.1:8001/")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse MCP URL: %v", err)
	}
	if parsed.Scheme != "http" || parsed.Host != "127.0.0.1:8001" || parsed.Path != "/mcp" {
		t.Fatalf("URL = %q", raw)
	}
	if parsed.RawQuery != "" {
		t.Fatalf("MCP endpoint query = %q", parsed.RawQuery)
	}
}

func TestProcessExitPreservesFinalizationErrors(t *testing.T) {
	waitErr := errors.New("wait failed")
	stdoutErr := errors.New("stdout failed")
	stderrErr := errors.New("stderr failed")
	recordErr := errors.New("record failed")
	exit := processExit{waitErr: waitErr, stdoutErr: stdoutErr, stderrErr: stderrErr, recordErr: recordErr}
	for _, want := range []error{waitErr, stdoutErr, stderrErr, recordErr} {
		if !errors.Is(exit.err(), want) {
			t.Fatalf("combined error %v does not preserve %v", exit.err(), want)
		}
	}
	if errors.Is(exit.finalizationErr(), waitErr) {
		t.Fatalf("finalization error includes wait error: %v", exit.finalizationErr())
	}
	for _, want := range []error{stdoutErr, stderrErr, recordErr} {
		if !errors.Is(exit.finalizationErr(), want) {
			t.Fatalf("finalization error %v does not preserve %v", exit.finalizationErr(), want)
		}
	}
}

func TestPiContainerOwnership(t *testing.T) {
	active := activeJurorOpportunity{principalID: "J1@" + strings.Repeat("x", 100), opportunityID: "opportunity-1"}
	name := piContainerName("case with spaces", active)
	if len(name) > 63 || strings.ContainsAny(name, "/ @") {
		t.Fatalf("container name = %q", name)
	}
	longPrefix := "case-" + strings.Repeat("x", 100)
	if piContainerName(longPrefix+"-a", active) == piContainerName(longPrefix+"-b", active) {
		t.Fatal("distinct long container inputs have the same name")
	}
	args := piRunArgs(Options{PiImage: "pi-image", PiMCPAdapter: "adapter"}, name, "/home", "model", "instructions")
	if !strings.Contains(strings.Join(args, "\n"), "--name\n"+name) {
		t.Fatalf("Pi args do not name container %q: %#v", name, args)
	}
	args, err := addContainerIDPath(args, "/owned/container.cid")
	if err != nil {
		t.Fatalf("add container ID path: %v", err)
	}
	if len(args) < 3 || args[0] != "run" || args[1] != "--cidfile" || args[2] != "/owned/container.cid" {
		t.Fatalf("Pi args do not contain the container ID path: %#v", args)
	}
	if !canonicalContainerID(strings.Repeat("a", 64)) || canonicalContainerID("abc123") || canonicalContainerID(strings.Repeat("A", 64)) {
		t.Fatal("container ID validation accepted a noncanonical value")
	}

	dir := t.TempDir()
	stdout, err := os.Create(filepath.Join(dir, "pi.stdout"))
	if err != nil {
		t.Fatalf("create stdout: %v", err)
	}
	stderr, err := os.Create(filepath.Join(dir, "pi.stderr"))
	if err != nil {
		t.Fatalf("create stderr: %v", err)
	}
	cmd := exec.Command("sleep", "60")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	runtimeLog := filepath.Join(dir, "runtime.log")
	runtimePath := filepath.Join(dir, "podman")
	t.Setenv("FAKE_CONTAINER_PID", fmt.Sprintf("%d", cmd.Process.Pid))
	t.Setenv("FAKE_CONTAINER_LOG", runtimeLog)
	runtimeScript := "#!/bin/sh\nprintf '%s\\n' \"$*\" > \"$FAKE_CONTAINER_LOG\"\nkill \"$FAKE_CONTAINER_PID\"\n"
	if err := os.WriteFile(runtimePath, []byte(runtimeScript), 0o755); err != nil {
		t.Fatalf("write fake container runtime: %v", err)
	}
	containerID := strings.Repeat("a", 64)
	containerIDPath, containerIDDir, err := createContainerIDPath(dir, name)
	if err != nil {
		t.Fatalf("create container ID path: %v", err)
	}
	if err := os.WriteFile(containerIDPath, []byte(containerID+"\n"), 0o600); err != nil {
		t.Fatalf("write container ID: %v", err)
	}
	proc := &processRecord{
		name:            "pi-J1",
		kind:            "podman",
		command:         cmd,
		done:            make(chan processExit, 1),
		stopCommand:     runtimePath,
		containerIDPath: containerIDPath,
		containerIDDir:  containerIDDir,
		finished:        make(chan struct{}),
	}
	go func() {
		exit := processExit{waitErr: cmd.Wait(), stdoutErr: stdout.Close(), stderrErr: stderr.Close()}
		proc.markExited()
		proc.done <- exit
	}()
	t.Cleanup(func() {
		if !proc.isExited() {
			_ = cmd.Process.Kill()
			<-proc.finished
		}
	})
	if err := (&runState{processes: []*processRecord{proc}}).stopAgents(); err != nil {
		t.Fatalf("stop agents: %v", err)
	}
	select {
	case <-proc.finished:
	default:
		t.Fatal("stopAgents returned before the process finished")
	}
	raw, err := os.ReadFile(runtimeLog)
	if err != nil {
		t.Fatalf("read fake container runtime log: %v", err)
	}
	if strings.TrimSpace(string(raw)) != "container rm -f "+containerID {
		t.Fatalf("container runtime args = %q", raw)
	}

	collisionIDPath, collisionIDDir, err := createContainerIDPath(dir, name)
	if err != nil {
		t.Fatalf("reserve collision container ID path: %v", err)
	}
	collisionCmd := exec.Command("sleep", "60")
	if err := collisionCmd.Start(); err != nil {
		t.Fatalf("start collision client: %v", err)
	}
	collision := &processRecord{
		name:            "pi-J1-collision",
		kind:            "podman",
		command:         collisionCmd,
		done:            make(chan processExit, 1),
		stopCommand:     runtimePath,
		containerIDPath: collisionIDPath,
		containerIDDir:  collisionIDDir,
		finished:        make(chan struct{}),
	}
	go func() {
		exit := processExit{waitErr: collisionCmd.Wait()}
		collision.markExited()
		collision.done <- exit
	}()
	if err := stopContainerProcess(collision); err == nil || !strings.Contains(err.Error(), "container ID") {
		t.Fatalf("stop collision client error = %v", err)
	}
	select {
	case <-collision.finished:
	default:
		t.Fatal("collision client was not reaped")
	}
	raw, err = os.ReadFile(runtimeLog)
	if err != nil {
		t.Fatalf("read fake container runtime log after collision: %v", err)
	}
	if strings.TrimSpace(string(raw)) != "container rm -f "+containerID {
		t.Fatalf("name collision caused an unowned container removal: %q", raw)
	}
}

func TestAutoLawyerRoles(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"both":      {"plaintiff", "defendant"},
		"plaintiff": {"plaintiff"},
		"defendant": {"defendant"},
		"none":      nil,
	}
	for mode, want := range cases {
		got, err := autoLawyerRoles(mode)
		if err != nil {
			t.Fatalf("autoLawyerRoles(%q): %v", mode, err)
		}
		if len(got) != len(want) {
			t.Fatalf("autoLawyerRoles(%q) len = %d, want %d", mode, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("autoLawyerRoles(%q)[%d] = %q, want %q", mode, i, got[i], want[i])
			}
		}
	}
	if _, err := autoLawyerRoles("other"); err == nil {
		t.Fatalf("autoLawyerRoles accepted invalid mode")
	}
	if got := strings.Join(manualLawyerRoles("none"), ","); got != "plaintiff,defendant" {
		t.Fatalf("manual roles = %q", got)
	}
}

func TestApplyDefaultsUsesSelectedEnvironmentsAndLawyers(t *testing.T) {
	participantEnvironment := []string{"HOME=/selected/home", "PI_CONTAINER_IMAGE=selected-pi"}
	coreEnvironment := []string{"OPENAI_API_KEY=core"}
	opts := applyDefaults(Options{
		CoreEnvironment:        coreEnvironment,
		ParticipantEnvironment: participantEnvironment,
		PlaintiffLawyer:        LawyerProfile{Runner: LawyerCodex},
		DefendantLawyer:        LawyerProfile{Runner: LawyerClaude},
	})
	participantEnvironment[0] = "HOME=/changed"
	coreEnvironment[0] = "OPENAI_API_KEY=changed"
	if opts.OpenClawCodexAuthPath != "/selected/home/.codex/auth.json" {
		t.Fatalf("Codex auth path = %q", opts.OpenClawCodexAuthPath)
	}
	if opts.PiImage != "selected-pi" {
		t.Fatalf("Pi image = %q", opts.PiImage)
	}
	if got, _ := environmentValue(opts.ParticipantEnvironment, "HOME"); got != "/selected/home" {
		t.Fatalf("copied HOME = %q", got)
	}
	if got, _ := environmentValue(opts.CoreEnvironment, "OPENAI_API_KEY"); got != "core" {
		t.Fatalf("copied core credential = %q", got)
	}
	if opts.PlaintiffLawyer.Runner != LawyerCodex || opts.DefendantLawyer.Runner != LawyerClaude {
		t.Fatalf("lawyer profiles = %#v, %#v", opts.PlaintiffLawyer, opts.DefendantLawyer)
	}
}

func TestValidateLawyerProfileAuthentication(t *testing.T) {
	home := t.TempDir()
	codexAuth := filepath.Join(home, "codex-auth.json")
	claudeAuth := filepath.Join(home, "claude-credentials.json")
	if err := os.WriteFile(codexAuth, []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeAuth, []byte(`{"claudeAiOauth":{"accessToken":"token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := applyDefaults(Options{ParticipantEnvironment: []string{"HOME=" + home}})
	if err := validateLawyerProfile(LawyerProfile{Runner: LawyerCodex, CredentialsFile: codexAuth}, opts, []string{"HOME=" + home}); err != nil {
		t.Fatalf("Codex profile: %v", err)
	}
	if err := validateLawyerProfile(LawyerProfile{Runner: LawyerClaude, CredentialsFile: claudeAuth}, opts, []string{"HOME=" + home}); err != nil {
		t.Fatalf("Claude profile: %v", err)
	}
	if err := validateLawyerProfile(LawyerProfile{
		Runner:    LawyerPi,
		Model:     "openrouter/anthropic/claude-sonnet-4",
		AuthMode:  headless.AuthAPIKey,
		APIKeyEnv: "SELECTED_PI_KEY",
	}, opts, []string{"HOME=" + home, "SELECTED_PI_KEY=selected"}); err != nil {
		t.Fatalf("Pi profile: %v", err)
	}
	if err := validateLawyerProfile(LawyerProfile{Runner: LawyerPi}, opts, []string{"HOME=" + home, "OPENROUTER_API_KEY=present"}); err == nil || !strings.Contains(err.Error(), "explicit API-key") {
		t.Fatalf("implicit Pi authentication error = %v", err)
	}
}

func TestValidatePropositionOptions(t *testing.T) {
	base := applyDefaults(Options{
		Proposition:            "The sky is blue.",
		EvidenceStandard:       "preponderance_of_the_evidence",
		MaxDocumentFiles:       10,
		MaxDocumentFileBytes:   100,
		MaxDocumentsTotalBytes: 500,
		OutputDir:              t.TempDir(),
		CaseID:                 "case-1",
		TrialMode:              "bench",
		AutoLawyers:            "plaintiff",
		PlaintiffLawyer:        LawyerProfile{Runner: LawyerOpenClaw, AuthMode: headless.AuthAPIKey, APIKeyEnv: "LAWYER_KEY"},
		CoreEnvironment:        []string{},
		ParticipantEnvironment: []string{"LAWYER_KEY=selected"},
	})
	if err := validateOptions(base); err != nil {
		t.Fatalf("valid bench proposition: %v", err)
	}
	invalidStandard := base
	invalidStandard.EvidenceStandard = "more_likely_than_not"
	if err := validateOptions(invalidStandard); err == nil || !strings.Contains(err.Error(), "evidence standard must be") {
		t.Fatalf("invalid evidence-standard error = %v", err)
	}
	jury := base
	jury.TrialMode = "jury"
	if err := validateOptions(jury); err == nil || !strings.Contains(err.Error(), "OPENROUTER_API_KEY") {
		t.Fatalf("missing jury credential error = %v", err)
	}
	jury.CoreEnvironment = append(jury.CoreEnvironment, "OPENROUTER_API_KEY=selected")
	if err := validateOptions(jury); err != nil {
		t.Fatalf("valid jury proposition: %v", err)
	}
}

func TestHandleLawyerExitUsesADCStatus(t *testing.T) {
	responses := make(chan string, 2)
	responses <- `{"status":"waiting","case_status":{"status":"deliberation"}}`
	responses <- `{"status":"done","case_status":{"status":"judgment_entered"}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/roleapi/v1/status" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if request.URL.Query().Get("case_id") != "case-1" || request.URL.Query().Get("role_id") != "plaintiff" {
			t.Errorf("query = %q", request.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(<-responses)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()
	state := &runState{opts: Options{CaseID: "case-1"}, caseBase: server.URL}
	if err := state.handleLawyerExit(context.Background(), "plaintiff", "codex-plaintiff"); err == nil || !strings.Contains(err.Error(), "before case completion") {
		t.Fatalf("premature exit error = %v", err)
	}
	if err := state.handleLawyerExit(context.Background(), "plaintiff", "codex-plaintiff"); err != nil {
		t.Fatalf("terminal exit: %v", err)
	}
}

func TestResolveOpenClawAuthDefaultsToCodexAuth(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "api-key")
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(path, []byte(`{"tokens":{"access_token":"token-1"}}`), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	auth, err := resolveOpenClawAuth(Options{OpenClawCodexAuthPath: path})
	if err != nil {
		t.Fatalf("resolve OpenClaw auth: %v", err)
	}
	if auth.Mode != "codex" || auth.CodexAuthPath != path {
		t.Fatalf("auth = %#v", auth)
	}
}

func TestResolveOpenClawAuthDoesNotFallBackToAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "api-key")
	_, err := resolveOpenClawAuth(Options{
		OpenClawCodexAuthPath: filepath.Join(t.TempDir(), "missing-auth.json"),
	})
	if err == nil || !strings.Contains(err.Error(), "read Codex auth file") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveOpenClawAuthRequiresExplicitAPIKeyMode(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := resolveOpenClawAuth(Options{OpenClawAuth: "api-key"})
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("api-key error = %v", err)
	}
	_, err = resolveOpenClawAuth(Options{OpenClawAuth: "auto"})
	if err == nil || !strings.Contains(err.Error(), "expected codex or api-key") {
		t.Fatalf("auto error = %v", err)
	}
}

func TestLawyerEnvironmentSeparatesRoleCredentials(t *testing.T) {
	opts := Options{
		PlaintiffLawyer: LawyerProfile{Runner: LawyerCodex, AuthMode: headless.AuthAPIKey, APIKeyEnv: "PLAINTIFF_KEY"},
		DefendantLawyer: LawyerProfile{Runner: LawyerClaude, AuthMode: headless.AuthAPIKey, APIKeyEnv: "DEFENDANT_KEY"},
	}
	base := []string{"PATH=/bin", "PLAINTIFF_KEY=plaintiff", "DEFENDANT_KEY=defendant", "OPENROUTER_API_KEY=jurors"}
	plaintiff := lawyerEnvironment(opts.PlaintiffLawyer, opts, base)
	if value, ok := environmentValue(plaintiff, "PLAINTIFF_KEY"); !ok || value != "plaintiff" {
		t.Fatalf("plaintiff credential = %q, %t", value, ok)
	}
	for _, name := range []string{"DEFENDANT_KEY", "OPENROUTER_API_KEY"} {
		if _, ok := environmentValue(plaintiff, name); ok {
			t.Fatalf("plaintiff environment contains %s", name)
		}
	}
}

func TestApplyDefaultsOpenClawNetworkHostMCPHost(t *testing.T) {
	opts := applyDefaults(Options{OpenClawNetwork: "host"})
	if opts.DockerMCPHost != "127.0.0.1" {
		t.Fatalf("DockerMCPHost = %q", opts.DockerMCPHost)
	}
	opts = applyDefaults(Options{OpenClawNetwork: "host", DockerMCPHost: "custom"})
	if opts.DockerMCPHost != "custom" {
		t.Fatalf("custom DockerMCPHost = %q", opts.DockerMCPHost)
	}
	opts = applyDefaults(Options{})
	if opts.DockerMCPHost != "host.docker.internal" {
		t.Fatalf("default DockerMCPHost = %q", opts.DockerMCPHost)
	}
}

func TestValidateOptionsRejectsInvalidOpenClawNetwork(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "input.md")
	if err := os.WriteFile(file, []byte("input"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := validateOptions(applyDefaults(Options{
		ScenarioPath:      file,
		OutputDir:         dir,
		CaseID:            "case",
		JurorPersonasPath: file,
		AutoLawyers:       DefaultAutoLawyers,
		OpenClawNetwork:   "bridge",
		CoreEnvironment:   []string{"OPENROUTER_API_KEY=key"},
	}))
	if err == nil || !strings.Contains(err.Error(), "invalid OpenClaw network") {
		t.Fatalf("validateOptions error = %v", err)
	}
}

func TestIsConnectionRefused(t *testing.T) {
	t.Parallel()

	err := &url.Error{
		Op:  "Get",
		URL: "http://127.0.0.1:1/roleapi/v1/status",
		Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED},
	}
	if !isConnectionRefused(err) {
		t.Fatalf("isConnectionRefused returned false for ECONNREFUSED")
	}
	if isConnectionRefused(os.ErrNotExist) {
		t.Fatalf("isConnectionRefused returned true for unrelated error")
	}
}

func TestWritePiConfigUsesFullOpenRouterSpec(t *testing.T) {
	t.Parallel()

	spec, err := modelrequest.ParseJSON([]byte(`{
		"endpoint":"openrouter",
		"model":"anthropic/claude-3.5-sonnet",
		"provider":{"only":["deepinfra"],"allow_fallbacks":false,"require_parameters":true,"quantizations":["bf16"]},
		"request":{"temperature":0.2,"top_p":0.8},
		"headers":{"X-Test-Request":"adc"},
		"persona":"personas/j1.txt"
	}`))
	if err != nil {
		t.Fatalf("parse request spec: %v", err)
	}
	home := t.TempDir()
	model, err := writePiConfig(home, activeJurorOpportunity{
		principalID: "J1",
		requestSpec: &spec,
	}, "adc", "http://host/mcp", "adjmcp1.test.signature")
	if err != nil {
		t.Fatalf("writePiConfig: %v", err)
	}
	if model != "anthropic/claude-3.5-sonnet" {
		t.Fatalf("model = %q", model)
	}

	models := readJSONMap(t, filepath.Join(home, ".pi", "agent", "models.json"))
	openrouter := models["providers"].(map[string]any)["openrouter"].(map[string]any)
	entries := openrouter["models"].([]any)
	entry := entries[0].(map[string]any)
	if entry["maxTokens"].(float64) != float64(DefaultJurorMaxOutputTokens) {
		t.Fatalf("maxTokens = %#v", entry["maxTokens"])
	}
	sampling := entry["samplingParams"].(map[string]any)
	if sampling["temperature"] != 0.2 || sampling["top_p"] != 0.8 {
		t.Fatalf("samplingParams = %#v", sampling)
	}
	routing := entry["compat"].(map[string]any)["openRouterRouting"].(map[string]any)
	if routing["allow_fallbacks"] != false {
		t.Fatalf("allow_fallbacks = %#v", routing["allow_fallbacks"])
	}
	if routing["require_parameters"] != true {
		t.Fatalf("require_parameters = %#v", routing["require_parameters"])
	}
	if routing["only"].([]any)[0].(string) != "deepinfra" {
		t.Fatalf("provider.only = %#v", routing["only"])
	}
	if routing["quantizations"].([]any)[0].(string) != "bf16" {
		t.Fatalf("provider.quantizations = %#v", routing["quantizations"])
	}
	requestHeaders := openrouter["headers"].(map[string]any)
	if requestHeaders["X-Test-Request"] != "adc" || requestHeaders["X-OpenRouter-Experimental-Metadata"] != "enabled" {
		t.Fatalf("provider headers = %#v", requestHeaders)
	}

	mcpPath := filepath.Join(home, ".mcp.json")
	mcp := readJSONMap(t, mcpPath)
	mcpInfo, err := os.Stat(mcpPath)
	if err != nil {
		t.Fatalf("stat MCP config: %v", err)
	}
	if mcpInfo.Mode().Perm() != 0o600 {
		t.Fatalf("MCP config mode = %04o", mcpInfo.Mode().Perm())
	}
	server := mcp["mcpServers"].(map[string]any)["adc"].(map[string]any)
	if server["transport"].(string) != "streamable-http" {
		t.Fatalf("MCP transport = %#v", server["transport"])
	}
	if server["headers"].(map[string]any)["Authorization"].(string) != "Bearer adjmcp1.test.signature" {
		t.Fatalf("MCP auth header = %#v", server["headers"])
	}
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return out
}
