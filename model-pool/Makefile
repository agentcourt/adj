.DEFAULT_GOAL := pool

UV ?= uv

ROOT_SEED ?= 0
QUESTIONS ?= sets/core20/questions.jsonl
PROMPT ?= prompts/juror-single.md
EVAL_TRIALS ?= 3
EVAL_PROCESSES ?= 8
SCREEN_PROCESSES ?= 8
SCREEN_DIRECT_TIMEOUT ?= 20
SCREEN_PI_TIMEOUT ?= 300
SCREEN_MAX_ATTEMPTS ?= 3
SCREEN_COMMAND ?= ../.bin/model-config-screen
PODMAN_COMMAND ?= podman
PI_IMAGE ?= agentcourt-pi-sandbox
PI_MCP_ADAPTER ?= /opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter
PI_MCP_HOST ?= 127.0.0.1
FILTER_PROVIDER_ERROR_COUNT ?= 0
FILTER_MIN_DELIBERATION_SCORE ?= 0.90
GENES ?= genes.json
GENE_COUNT ?= 14
PERSONA ?= ../common/etc/personas/generic.md
PERSONA_RECORD_PATH ?= personas/generic.md
SAMPLES_PER_GENE ?= 5
PCA_DIMENSIONS ?= 8
MIN_K ?= 2
MAX_K ?= 20
POOL_SIZE ?= 100
POOL_SEED ?= 0
TIMEOUT ?= 120
EVAL_NO_PROGRESS_TIMEOUT ?= 240

.PHONY: check pool screen-command

check:
	$(UV) run --no-cache python -m unittest discover -s tests -p 'test_*.py'

screen-command:
	mkdir -p ../.bin
	CGO_ENABLED=0 go -C .. build -buildvcs=false -o .bin/model-config-screen ./cmd/model-config-screen

pool: screen-command
	test -n "$(strip $(RUN_ID))" || { echo "RUN_ID is required" >&2; exit 2; }
	$(UV) run --no-cache --script tools/run_end_to_end.py \
		--run-id "$(RUN_ID)" \
		--full-catalog \
		--root-seed "$(ROOT_SEED)" \
		--questions "$(QUESTIONS)" \
		--prompt "$(PROMPT)" \
		--screen-command "$(SCREEN_COMMAND)" \
		--screen-processes "$(SCREEN_PROCESSES)" \
		--screen-direct-timeout "$(SCREEN_DIRECT_TIMEOUT)" \
		--screen-pi-timeout "$(SCREEN_PI_TIMEOUT)" \
		--screen-max-attempts "$(SCREEN_MAX_ATTEMPTS)" \
		--podman-command "$(PODMAN_COMMAND)" \
		--pi-image "$(PI_IMAGE)" \
		--pi-mcp-adapter "$(PI_MCP_ADAPTER)" \
		--pi-mcp-host "$(PI_MCP_HOST)" \
		--eval-trials "$(EVAL_TRIALS)" \
		--eval-processes "$(EVAL_PROCESSES)" \
		--filter-provider-error-count "$(FILTER_PROVIDER_ERROR_COUNT)" \
		--filter-min-deliberation-score "$(FILTER_MIN_DELIBERATION_SCORE)" \
		--genes "$(GENES)" \
		--gene-count "$(GENE_COUNT)" \
		--persona "$(PERSONA)" \
		--persona-record-path "$(PERSONA_RECORD_PATH)" \
		--samples-per-gene "$(SAMPLES_PER_GENE)" \
		--pca-dimensions "$(PCA_DIMENSIONS)" \
		--strict-pca-dimensions \
		--min-k "$(MIN_K)" \
		--max-k "$(MAX_K)" \
		--pool-size "$(POOL_SIZE)" \
		--pool-seed "$(POOL_SEED)" \
		--without-replacement \
		--one-per-model \
		--timeout "$(TIMEOUT)" \
		--eval-no-progress-timeout "$(EVAL_NO_PROGRESS_TIMEOUT)"
