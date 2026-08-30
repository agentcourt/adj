package adjudicate

import (
	"fmt"
	"sort"
	"strings"
)

func appendCorePromptArgs(args []string, promptDir string, promptFiles PromptFilePaths) []string {
	if promptDir != "" {
		args = append(args, "--prompt-dir", promptDir)
	}
	ids := make([]string, 0, len(promptFiles))
	for id := range promptFiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		args = append(args, "--prompt-file", id+"="+promptFiles[id])
	}
	return args
}

func splitFormalPromptFiles(procedure string, paths PromptFilePaths) (PromptFilePaths, PromptFilePaths, error) {
	core := make(PromptFilePaths)
	mcp := make(PromptFilePaths)
	for id, path := range paths {
		if id == "mcp." {
			return nil, nil, fmt.Errorf("%s MCP prompt ID is empty", procedure)
		}
		if strings.HasPrefix(id, "mcp.") {
			mcp[id] = path
		} else {
			core[id] = path
		}
	}
	return core, mcp, nil
}
