package store

import (
	"encoding/json"
	"fmt"
)

type PacerCase struct {
	RunID     string                 `json:"run_id"`
	CaseID    string                 `json:"case_id"`
	Caption   string                 `json:"caption"`
	Judge     string                 `json:"judge"`
	Status    string                 `json:"status"`
	TrialMode string                 `json:"trial_mode"`
	Phase     string                 `json:"phase"`
	Case      map[string]any         `json:"-"`
	Meta      map[string]interface{} `json:"meta"`
}

type PacerDocument struct {
	DocumentID   string         `json:"document_id"`
	Source       string         `json:"source"`
	Title        string         `json:"title"`
	DocumentType string         `json:"document_type"`
	FiledAt      string         `json:"filed_at"`
	Description  string         `json:"description,omitempty"`
	Body         string         `json:"body,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

func (s *Store) LoadLatestCase(caseID string) (PacerCase, error) {
	rows, err := s.db.Query(
		`SELECT run_id, scenario_name, final_state_json, started_at, finished_at
		   FROM runs
		  WHERE final_state_json IS NOT NULL
		  ORDER BY COALESCE(finished_at, started_at) DESC`,
	)
	if err != nil {
		return PacerCase{}, fmt.Errorf("query runs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var runID string
		var scenarioName string
		var finalStateJSON string
		var startedAt string
		var finishedAt string
		if err := rows.Scan(&runID, &scenarioName, &finalStateJSON, &startedAt, &finishedAt); err != nil {
			return PacerCase{}, fmt.Errorf("scan run row: %w", err)
		}
		var finalState map[string]any
		if err := json.Unmarshal([]byte(finalStateJSON), &finalState); err != nil {
			continue
		}
		caseObj, _ := finalState["case"].(map[string]any)
		if caseObj == nil {
			continue
		}
		foundCaseID, _ := caseObj["case_id"].(string)
		if caseID != "" && foundCaseID != caseID {
			continue
		}
		caption, _ := caseObj["caption"].(string)
		judge, _ := caseObj["judge"].(string)
		status, _ := caseObj["status"].(string)
		trialMode, _ := caseObj["trial_mode"].(string)
		phase, _ := caseObj["phase"].(string)
		return PacerCase{
			RunID:     runID,
			CaseID:    foundCaseID,
			Caption:   caption,
			Judge:     judge,
			Status:    status,
			TrialMode: trialMode,
			Phase:     phase,
			Case:      caseObj,
			Meta: map[string]interface{}{
				"scenario_name": scenarioName,
				"started_at":    startedAt,
				"finished_at":   finishedAt,
			},
		}, nil
	}
	if err := rows.Err(); err != nil {
		return PacerCase{}, fmt.Errorf("iterate runs: %w", err)
	}
	if caseID == "" {
		return PacerCase{}, fmt.Errorf("no completed run with final state in sqlite")
	}
	return PacerCase{}, fmt.Errorf("case_id %q not found in sqlite final states", caseID)
}

func BuildPacerDocuments(caseObj map[string]any) []PacerDocument {
	docs := make([]PacerDocument, 0)
	docket, _ := caseObj["docket"].([]any)
	for i, entryAny := range docket {
		entry, _ := entryAny.(map[string]any)
		if entry == nil {
			continue
		}
		title, _ := entry["title"].(string)
		desc, _ := entry["description"].(string)
		docs = append(docs, PacerDocument{
			DocumentID:   fmt.Sprintf("docket-%04d", i+1),
			Source:       "docket",
			Title:        title,
			DocumentType: "docket_entry",
			Description:  desc,
			Metadata: map[string]any{
				"docket_index": i,
			},
		})
	}

	filings, _ := caseObj["filing_documents"].([]any)
	for i, filingAny := range filings {
		filing, _ := filingAny.(map[string]any)
		if filing == nil {
			continue
		}
		title, _ := filing["title"].(string)
		filingType, _ := filing["filing_type"].(string)
		filedAt, _ := filing["filed_at"].(string)
		filedBy, _ := filing["filed_by"].(string)
		body, _ := filing["body"].(string)
		docs = append(docs, PacerDocument{
			DocumentID:   fmt.Sprintf("filing-%04d", i+1),
			Source:       "filing_documents",
			Title:        title,
			DocumentType: filingType,
			FiledAt:      filedAt,
			Body:         body,
			Metadata: map[string]any{
				"filing_index": i,
				"filed_by":     filedBy,
			},
		})
	}
	return docs
}

func FindPacerDocument(docs []PacerDocument, documentID string) (PacerDocument, bool) {
	for _, doc := range docs {
		if doc.DocumentID == documentID {
			return doc, true
		}
	}
	return PacerDocument{}, false
}
