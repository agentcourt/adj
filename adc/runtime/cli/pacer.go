package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jsmorph/adj/adc/runtime/store"
)

func RunPacer(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("pacer", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc pacer --db <sqlite> [--case-id <id>] [--document-id <id>]\n\n")
		fs.PrintDefaults()
	})
	dbPath := fs.String("db", "out/adc-run.db", "SQLite path")
	caseID := fs.String("case-id", "", "Case ID (optional; latest case if omitted)")
	documentID := fs.String("document-id", "", "Document ID for single-document fetch")
	jsonSummary := fs.Bool("json", true, "Emit JSON output")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc pacer accepts no positional arguments")
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	closeStore := func(err error) error {
		if closeErr := st.Close(); closeErr != nil {
			return errors.Join(err, fmt.Errorf("close sqlite: %w", closeErr))
		}
		return err
	}

	c, err := st.LoadLatestCase(strings.TrimSpace(*caseID))
	if err != nil {
		return closeStore(err)
	}
	if err := closeStore(nil); err != nil {
		return err
	}
	docs := store.BuildPacerDocuments(c.Case)
	if strings.TrimSpace(*documentID) != "" {
		doc, ok := store.FindPacerDocument(docs, strings.TrimSpace(*documentID))
		if !ok {
			return fmt.Errorf("document_id %q not found", strings.TrimSpace(*documentID))
		}
		payload := map[string]any{
			"case": map[string]any{
				"run_id":     c.RunID,
				"case_id":    c.CaseID,
				"caption":    c.Caption,
				"judge":      c.Judge,
				"status":     c.Status,
				"trial_mode": c.TrialMode,
				"phase":      c.Phase,
				"meta":       c.Meta,
			},
			"document": doc,
		}
		return writePacerOutput(stdout, payload, *jsonSummary)
	}
	payload := map[string]any{
		"case": map[string]any{
			"run_id":     c.RunID,
			"case_id":    c.CaseID,
			"caption":    c.Caption,
			"judge":      c.Judge,
			"status":     c.Status,
			"trial_mode": c.TrialMode,
			"phase":      c.Phase,
			"meta":       c.Meta,
		},
		"documents": docs,
	}
	return writePacerOutput(stdout, payload, *jsonSummary)
}

func writePacerOutput(stdout io.Writer, v map[string]any, jsonSummary bool) error {
	if jsonSummary {
		wire, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(wire))
		return err
	}
	caseObj, _ := v["case"].(map[string]any)
	caseID, _ := caseObj["case_id"].(string)
	caption, _ := caseObj["caption"].(string)
	if docObj, ok := v["document"].(store.PacerDocument); ok {
		_, err := fmt.Fprintf(stdout, "case_id=%s caption=%q document_id=%s title=%q type=%s\n", caseID, caption, docObj.DocumentID, docObj.Title, docObj.DocumentType)
		return err
	}
	docs, _ := v["documents"].([]store.PacerDocument)
	_, err := fmt.Fprintf(stdout, "case_id=%s caption=%q documents=%d\n", caseID, caption, len(docs))
	return err
}
