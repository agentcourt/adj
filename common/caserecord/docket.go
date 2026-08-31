package caserecord

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type recordedEvent struct {
	line            int
	timestamp       time.Time
	timestampSource string
	value           map[string]any
	used            bool
}

func (b *builder) buildDocket(includeWorkNotes bool) error {
	b.addComplaint()
	if err := b.addDocumentManifest(); err != nil {
		return err
	}
	b.addInputFiles()
	if err := b.addEvidenceManifest(); err != nil {
		return err
	}

	var err error
	switch b.layout.manifest.Procedure {
	case "simple":
		err = b.addSimpleDocket()
	case "quick":
		err = b.addQuickDocket()
	case "arb", "arbd":
		err = b.addArbitrationDocket()
	case "adc":
		err = b.addADCDocket()
	}
	if err != nil {
		return err
	}
	if includeWorkNotes {
		return b.addWorkNotes()
	}
	return nil
}

func (b *builder) addComplaint() {
	path := b.coreRelative("complaint.md")
	if _, ok := b.artifacts[artifactKey("record", path)]; !ok {
		return
	}
	b.addDocket(DocketItem{
		ID:              "complaint",
		Timestamp:       b.layout.manifest.StartedAt.UTC(),
		TimestampSource: "case_manifest",
		Phase:           "initiation",
		Kind:            "complaint",
		Title:           "Complaint",
		Source:          b.coreReference("complaint.md"),
		ArtifactRefs:    b.artifactReference(path),
	})
}

func (b *builder) addDocumentManifest() error {
	manifestRelative := b.coreRelative("documents.json")
	manifestPath := filepath.Join(b.layout.recordRoot, filepath.FromSlash(manifestRelative))
	var manifest map[string]any
	if err := readJSON(manifestPath, &manifest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if _, ok := b.artifacts[artifactKey("record", b.coreRelative("evidence-manifest.json"))]; ok {
				return nil
			}
			manifestRelative = "inputs/documents.json"
			manifestPath = filepath.Join(b.layout.recordRoot, filepath.FromSlash(manifestRelative))
			if err := readJSON(manifestPath, &manifest); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil
				}
				return fmt.Errorf("read document manifest: %w", err)
			}
		} else {
			return fmt.Errorf("read document manifest: %w", err)
		}
	}
	files := mapList(manifest["files"])
	pointerField := "files"
	if len(files) == 0 {
		files = mapList(manifest["documents"])
		pointerField = "documents"
	}
	for index, file := range files {
		path := stringValue(file["path"])
		if path == "" {
			continue
		}
		refs := b.documentArtifactRefs(path)
		sha := stringValue(file["sha256"])
		for _, ref := range refs {
			key := artifactKey(ref.SourceID, ref.Path)
			artifact := b.artifacts[key]
			artifact.RecordedSHA256 = sha
			if mediaType := stringValue(file["media_type"]); mediaType != "" {
				artifact.MediaType = mediaType
			}
			b.artifacts[key] = artifact
		}
		b.addDocket(DocketItem{
			ID:              fmt.Sprintf("initial-document-%d", index+1),
			Timestamp:       b.layout.manifest.StartedAt.UTC(),
			TimestampSource: "case_manifest",
			Phase:           "initiation",
			Kind:            "evidence",
			Title:           filepath.Base(path),
			Source: Reference{
				SourceID:    "record",
				Path:        manifestRelative,
				JSONPointer: fmt.Sprintf("/%s/%d", pointerField, index),
			},
			ArtifactRefs: refs,
		})
	}
	return nil
}

func (b *builder) documentArtifactRefs(path string) []Reference {
	candidates := []string{
		b.coreRelative(filepath.Join("documents", filepath.FromSlash(path))),
		filepath.ToSlash(filepath.Join("inputs", "documents", filepath.FromSlash(path))),
	}
	var refs []Reference
	for _, candidate := range candidates {
		if _, ok := b.artifacts[artifactKey("record", candidate)]; ok {
			refs = append(refs, Reference{SourceID: "record", Path: candidate})
		}
	}
	return refs
}

func (b *builder) addInputFiles() {
	prefix := strings.TrimSuffix(b.coreRelative("input-files"), "/") + "/"
	var paths []string
	for _, artifact := range b.artifacts {
		if artifact.SourceID == "record" && strings.HasPrefix(artifact.Path, prefix) {
			paths = append(paths, artifact.Path)
		}
	}
	sort.Strings(paths)
	for index, path := range paths {
		b.addDocket(DocketItem{
			ID:              fmt.Sprintf("complaint-attachment-%d", index+1),
			Timestamp:       b.layout.manifest.StartedAt.UTC(),
			TimestampSource: "case_manifest",
			Phase:           "initiation",
			Kind:            "evidence",
			Title:           filepath.Base(path),
			Source:          Reference{SourceID: "record", Path: path},
			ArtifactRefs:    b.artifactReference(path),
		})
	}
}

func (b *builder) addEvidenceManifest() error {
	relative := b.coreRelative("evidence-manifest.json")
	path := filepath.Join(b.layout.recordRoot, filepath.FromSlash(relative))
	var manifest map[string]any
	if err := readJSON(path, &manifest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read evidence manifest: %w", err)
	}
	createdAt, _ := parseTime(stringValue(manifest["created_at"]))
	for index, evidence := range mapList(manifest["evidence"]) {
		evidenceID := firstString(evidence, "evidence_id", "file_id")
		title := firstString(evidence, "title", "original_name", "name", "label")
		if title == "" {
			title = evidenceID
		}
		timestamp, source := firstTimestamp(evidence, "created_at", "imported_at", "retrieval_timestamp")
		if timestamp.IsZero() {
			timestamp = createdAt
			source = "record"
		}
		if timestamp.IsZero() {
			timestamp = b.layout.manifest.StartedAt.UTC()
			source = "case_manifest"
		}
		refs := b.evidenceArtifactRefs(evidence)
		sha := stringValue(evidence["sha256"])
		for _, ref := range refs {
			key := artifactKey(ref.SourceID, ref.Path)
			artifact := b.artifacts[key]
			artifact.RecordedSHA256 = sha
			if mediaType := firstString(evidence, "mime_type", "media_type"); mediaType != "" {
				artifact.MediaType = mediaType
			}
			b.artifacts[key] = artifact
		}
		b.addDocket(DocketItem{
			ID:              fmt.Sprintf("evidence-%s-%d", safeID(evidenceID), index+1),
			Timestamp:       timestamp,
			TimestampSource: source,
			Phase:           firstString(evidence, "submitted_phase"),
			Actor:           firstString(evidence, "submitted_by_role", "imported_by"),
			Kind:            "evidence",
			Title:           title,
			Source: Reference{
				SourceID:    "record",
				Path:        relative,
				JSONPointer: fmt.Sprintf("/evidence/%d", index),
			},
			ArtifactRefs: refs,
		})
	}
	return nil
}

func (b *builder) evidenceArtifactRefs(evidence map[string]any) []Reference {
	var candidates []string
	if storage := stringValue(evidence["storage_name"]); storage != "" && !filepath.IsAbs(storage) {
		candidates = append(candidates, b.coreRelative(filepath.Join("evidence-store", filepath.FromSlash(storage))))
	}
	if storage := stringValue(evidence["storage_relpath"]); storage != "" && !filepath.IsAbs(storage) {
		candidates = append(candidates, b.coreRelative(storage))
	}
	if name := stringValue(evidence["name"]); name != "" {
		candidates = append(candidates, b.coreRelative(filepath.Join("submitted-evidence", filepath.Base(name))))
	}
	var refs []Reference
	for _, candidate := range candidates {
		candidate = filepath.ToSlash(candidate)
		if _, ok := b.artifacts[artifactKey("record", candidate)]; ok {
			refs = append(refs, Reference{SourceID: "record", Path: candidate})
		}
	}
	return refs
}

func (b *builder) addSimpleDocket() error {
	relative := b.coreRelative("run.json")
	var run map[string]any
	if err := readJSON(filepath.Join(b.layout.recordRoot, filepath.FromSlash(relative)), &run); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read Simple run record: %w", err)
	}
	decision := mapValue(run["decision"])
	if len(decision) == 0 {
		return nil
	}
	timestamp, source := firstTimestamp(run, "finished_at")
	if timestamp.IsZero() {
		timestamp, source = b.artifactTime(relative)
	}
	value := firstString(decision, "value", "decision")
	b.addDocket(DocketItem{
		ID:              "simple-decision",
		Timestamp:       timestamp,
		TimestampSource: source,
		Phase:           "decision",
		Actor:           "adjudicator",
		Kind:            "decision",
		Title:           "Decision: " + value,
		Source:          Reference{SourceID: "record", Path: relative, JSONPointer: "/decision"},
	})
	return nil
}

func (b *builder) addQuickDocket() error {
	relative := b.coreRelative("run.json")
	var run map[string]any
	if err := readJSON(filepath.Join(b.layout.recordRoot, filepath.FromSlash(relative)), &run); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read Quick run record: %w", err)
	}
	for index, argument := range mapList(run["arguments"]) {
		timestamp, source := firstTimestamp(argument, "submitted_at")
		if timestamp.IsZero() {
			timestamp, source = b.artifactTime(relative)
		}
		role := stringValue(argument["role"])
		b.addDocket(DocketItem{
			ID:              fmt.Sprintf("quick-argument-%d", index+1),
			Timestamp:       timestamp,
			TimestampSource: source,
			Phase:           "arguments",
			Actor:           role,
			Kind:            "argument",
			Title:           titleCase(role) + " argument",
			Source:          Reference{SourceID: "record", Path: relative, JSONPointer: fmt.Sprintf("/arguments/%d", index)},
		})
	}
	for index, vote := range mapList(run["votes"]) {
		timestamp, source := firstTimestamp(vote, "submitted_at")
		if timestamp.IsZero() {
			timestamp, source = b.artifactTime(relative)
		}
		member := stringValue(vote["member_id"])
		b.addDocket(DocketItem{
			ID:              fmt.Sprintf("quick-vote-%d", index+1),
			Timestamp:       timestamp,
			TimestampSource: source,
			Phase:           "council",
			Actor:           member,
			Kind:            "council_vote",
			Title:           fmt.Sprintf("Council vote by %s: %s", member, stringValue(vote["vote"])),
			Source:          Reference{SourceID: "record", Path: relative, JSONPointer: fmt.Sprintf("/votes/%d", index)},
		})
	}
	for index, failure := range mapList(run["council_failures"]) {
		timestamp, source := firstTimestamp(failure, "failed_at")
		if timestamp.IsZero() {
			timestamp, source = b.artifactTime(relative)
		}
		member := stringValue(failure["member_id"])
		b.addDocket(DocketItem{
			ID:              fmt.Sprintf("quick-council-failure-%d", index+1),
			Timestamp:       timestamp,
			TimestampSource: source,
			Phase:           "council",
			Actor:           member,
			Kind:            "council_failure",
			Title:           fmt.Sprintf("Council member %s failed", member),
			Source:          Reference{SourceID: "record", Path: relative, JSONPointer: fmt.Sprintf("/council_failures/%d", index)},
		})
	}
	resolution := stringValue(run["resolution"])
	if resolution != "" {
		timestamp, source := firstTimestamp(run, "finished_at")
		if timestamp.IsZero() {
			timestamp, source = b.artifactTime(relative)
		}
		b.addDocket(DocketItem{
			ID:              "quick-decision",
			Timestamp:       timestamp,
			TimestampSource: source,
			Phase:           "decision",
			Actor:           "council",
			Kind:            "decision",
			Title:           "Decision: " + resolution,
			Source:          Reference{SourceID: "record", Path: relative, JSONPointer: "/resolution"},
		})
	}
	return nil
}

func (b *builder) addArbitrationDocket() error {
	events, err := b.readCoreEvents()
	if err != nil {
		return err
	}
	stateRelative := b.coreRelative("state.json")
	var state map[string]any
	if err := readJSON(filepath.Join(b.layout.recordRoot, filepath.FromSlash(stateRelative)), &state); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read arbitration state: %w", err)
		}
		return b.addArbitrationEvents(events)
	}
	caseState := mapValue(state["case"])
	filingAction := map[string]string{
		"openings":     "record_opening_statement",
		"arguments":    "submit_argument",
		"rebuttals":    "submit_rebuttal",
		"surrebuttals": "submit_surrebuttal",
		"closings":     "deliver_closing_statement",
	}
	for _, phase := range []string{"openings", "arguments", "rebuttals", "surrebuttals", "closings"} {
		for index, filing := range mapList(caseState[phase]) {
			event := matchAttorneyEvent(events, phase, filingAction[phase], filing)
			timestamp, source := b.stateTimestamp(stateRelative)
			if event != nil {
				timestamp, source = event.timestamp, event.timestampSource
				event.used = true
			}
			role := stringValue(filing["role"])
			b.addDocket(DocketItem{
				ID:              fmt.Sprintf("%s-%s-%d", b.layout.manifest.Procedure, phase, index+1),
				Timestamp:       timestamp,
				TimestampSource: source,
				Phase:           phase,
				Actor:           role,
				Kind:            filingKind(phase),
				Title:           filingTitle(phase, role),
				Source:          Reference{SourceID: "record", Path: stateRelative, JSONPointer: fmt.Sprintf("/case/%s/%d", phase, index)},
			})
		}
	}
	b.addArbitrationReports(caseState, stateRelative, events)
	b.addArbitrationOffers(caseState, stateRelative, events)
	b.addArbitrationSubmittedEvidence(caseState, stateRelative, events)
	b.addArbitrationCouncil(caseState, stateRelative, events)
	if err := b.addArbitrationEvents(events); err != nil {
		return err
	}
	return b.addArbitrationResult(caseState)
}

func (b *builder) addArbitrationResult(caseState map[string]any) error {
	relative := b.coreRelative("run.json")
	stateRelative := b.coreRelative("state.json")
	var run map[string]any
	if err := readJSON(filepath.Join(b.layout.recordRoot, filepath.FromSlash(relative)), &run); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read arbitration run record: %w", err)
	}
	timestamp, source := firstTimestamp(run, "finished_at")
	if timestamp.IsZero() {
		timestamp, source = b.artifactTime(relative)
	}
	if b.layout.manifest.Procedure == "arb" {
		resolution := firstString(run, "resolution")
		reference := Reference{SourceID: "record", Path: relative, JSONPointer: "/resolution"}
		if resolution == "" {
			resolution = stringValue(caseState["resolution"])
			reference = Reference{SourceID: "record", Path: stateRelative, JSONPointer: "/case/resolution"}
		}
		if resolution == "" {
			return nil
		}
		b.addDocket(DocketItem{
			ID:              "arb-decision",
			Timestamp:       timestamp,
			TimestampSource: source,
			Phase:           "decision",
			Actor:           "council",
			Kind:            "decision",
			Title:           "Decision: " + resolution,
			Source:          reference,
		})
		return nil
	}
	reference := Reference{SourceID: "record", Path: relative, JSONPointer: "/answers"}
	if len(mapValue(run["answers"])) == 0 {
		if len(mapList(caseState["council_answers"])) == 0 {
			return nil
		}
		reference = Reference{SourceID: "record", Path: stateRelative, JSONPointer: "/case/council_answers"}
	}
	b.addDocket(DocketItem{
		ID:              "arbd-decision",
		Timestamp:       timestamp,
		TimestampSource: source,
		Phase:           "decision",
		Actor:           "council",
		Kind:            "decision",
		Title:           "Degree decision entered",
		Source:          reference,
	})
	return nil
}

func (b *builder) addArbitrationReports(caseState map[string]any, stateRelative string, events []*recordedEvent) {
	for index, report := range mapList(caseState["technical_reports"]) {
		event := matchNestedAttorneyEvent(events, report, "technical_reports", "title")
		timestamp, source := b.stateTimestamp(stateRelative)
		if event != nil {
			timestamp, source = event.timestamp, event.timestampSource
		}
		b.addDocket(DocketItem{
			ID:              fmt.Sprintf("%s-technical-report-%d", b.layout.manifest.Procedure, index+1),
			Timestamp:       timestamp,
			TimestampSource: source,
			Phase:           stringValue(report["phase"]),
			Actor:           stringValue(report["role"]),
			Kind:            "technical_report",
			Title:           firstString(report, "title"),
			Source:          Reference{SourceID: "record", Path: stateRelative, JSONPointer: fmt.Sprintf("/case/technical_reports/%d", index)},
		})
	}
}

func (b *builder) addArbitrationOffers(caseState map[string]any, stateRelative string, events []*recordedEvent) {
	for index, offer := range mapList(caseState["offered_evidence"]) {
		event := matchNestedAttorneyEvent(events, offer, "offered_evidence", "evidence_id")
		timestamp, source := b.stateTimestamp(stateRelative)
		if event != nil {
			timestamp, source = event.timestamp, event.timestampSource
		}
		b.addDocket(DocketItem{
			ID:              fmt.Sprintf("%s-evidence-offer-%d", b.layout.manifest.Procedure, index+1),
			Timestamp:       timestamp,
			TimestampSource: source,
			Phase:           stringValue(offer["phase"]),
			Actor:           stringValue(offer["role"]),
			Kind:            "evidence_offer",
			Title:           "Evidence offered: " + firstString(offer, "label", "evidence_id"),
			Source:          Reference{SourceID: "record", Path: stateRelative, JSONPointer: fmt.Sprintf("/case/offered_evidence/%d", index)},
		})
	}
}

func (b *builder) addArbitrationSubmittedEvidence(caseState map[string]any, stateRelative string, events []*recordedEvent) {
	for index, evidence := range mapList(caseState["submitted_evidence"]) {
		event := matchEventValue(events, "submitted_evidence", "evidence_id", stringValue(evidence["evidence_id"]))
		timestamp, source := firstTimestamp(evidence, "retrieval_timestamp")
		if event != nil {
			timestamp, source = event.timestamp, event.timestampSource
			event.used = true
		}
		if timestamp.IsZero() {
			timestamp, source = b.stateTimestamp(stateRelative)
		}
		b.addDocket(DocketItem{
			ID:              fmt.Sprintf("%s-submitted-evidence-%d", b.layout.manifest.Procedure, index+1),
			Timestamp:       timestamp,
			TimestampSource: source,
			Phase:           stringValue(evidence["phase"]),
			Actor:           stringValue(evidence["role"]),
			Kind:            "submitted_evidence",
			Title:           firstString(evidence, "title", "name", "evidence_id"),
			Source:          Reference{SourceID: "record", Path: stateRelative, JSONPointer: fmt.Sprintf("/case/submitted_evidence/%d", index)},
		})
	}
}

func (b *builder) addArbitrationCouncil(caseState map[string]any, stateRelative string, events []*recordedEvent) {
	field, eventType, valueField, kind := "council_votes", "council_vote", "vote", "council_vote"
	if b.layout.manifest.Procedure == "arbd" {
		field, eventType, valueField, kind = "council_answers", "council_answer", "answer", "council_answer"
	}
	for index, decision := range mapList(caseState[field]) {
		member := stringValue(decision["member_id"])
		event := matchEventValue(events, eventType, "member_id", member)
		timestamp, source := b.stateTimestamp(stateRelative)
		if event != nil {
			timestamp, source = event.timestamp, event.timestampSource
			event.used = true
		}
		b.addDocket(DocketItem{
			ID:              fmt.Sprintf("%s-council-%d", b.layout.manifest.Procedure, index+1),
			Timestamp:       timestamp,
			TimestampSource: source,
			Phase:           "council",
			Actor:           member,
			Kind:            kind,
			Title:           fmt.Sprintf("Council %s by %s: %s", strings.TrimPrefix(kind, "council_"), member, displayValue(decision[valueField])),
			Source:          Reference{SourceID: "record", Path: stateRelative, JSONPointer: fmt.Sprintf("/case/%s/%d", field, index)},
		})
	}
}

func (b *builder) addArbitrationEvents(events []*recordedEvent) error {
	eventsRelative := b.coreRelative("events.ndjson")
	for _, event := range events {
		if event.used {
			continue
		}
		eventType := firstString(event.value, "type", "event_type")
		role := stringValue(event.value["role"])
		phase := stringValue(event.value["phase"])
		payload := mapValue(event.value["payload"])
		switch eventType {
		case "attorney_action":
			action := stringValue(payload["action_type"])
			b.addDocket(DocketItem{
				ID:              fmt.Sprintf("%s-event-filing-%d", b.layout.manifest.Procedure, event.line),
				Timestamp:       event.timestamp,
				TimestampSource: event.timestampSource,
				Phase:           phase,
				Actor:           role,
				Kind:            filingKind(phase),
				Title:           humanize(action),
				Source:          Reference{SourceID: "record", Path: eventsRelative, Line: event.line},
			})
		case "submitted_evidence":
			b.addDocket(DocketItem{
				ID:              fmt.Sprintf("%s-event-evidence-%d", b.layout.manifest.Procedure, event.line),
				Timestamp:       event.timestamp,
				TimestampSource: event.timestampSource,
				Phase:           phase,
				Actor:           role,
				Kind:            "submitted_evidence",
				Title:           firstString(payload, "title", "evidence_id"),
				Source:          Reference{SourceID: "record", Path: eventsRelative, Line: event.line},
			})
		case "council_vote", "council_answer":
			decision := mapValue(payload["payload"])
			if len(decision) == 0 {
				decision = payload
			}
			member := stringValue(payload["member_id"])
			value := displayValue(decision["vote"])
			if value == "" {
				value = displayValue(decision["answer"])
			}
			b.addDocket(DocketItem{
				ID:              fmt.Sprintf("%s-event-council-%d", b.layout.manifest.Procedure, event.line),
				Timestamp:       event.timestamp,
				TimestampSource: event.timestampSource,
				Phase:           "council",
				Actor:           member,
				Kind:            eventType,
				Title:           fmt.Sprintf("Council decision by %s: %s", member, value),
				Source:          Reference{SourceID: "record", Path: eventsRelative, Line: event.line},
			})
		}
	}
	return nil
}

func (b *builder) addADCDocket() error {
	events, err := b.readCoreEvents()
	if err != nil {
		return err
	}
	stateRelative := b.coreRelative("state.json")
	var finalState map[string]any
	stateErr := readJSON(filepath.Join(b.layout.recordRoot, filepath.FromSlash(stateRelative)), &finalState)
	if stateErr != nil && !errors.Is(stateErr, os.ErrNotExist) {
		return fmt.Errorf("read ADC state: %w", stateErr)
	}
	finalDocket := mapList(mapValue(finalState["case"])["docket"])
	emitted := 0
	if stateErr == nil {
		emitted, err = b.adcInitialDocket(finalDocket, stateRelative)
		if err != nil {
			return err
		}
	}
	for _, event := range events {
		if stringValue(event.value["action"]) == "" {
			continue
		}
		response := mapValue(event.value["response"])
		state := mapValue(response["state"])
		if len(state) == 0 {
			state = response
		}
		caseState := mapValue(state["case"])
		if _, ok := caseState["docket"]; !ok {
			continue
		}
		docket := mapList(caseState["docket"])
		if len(docket) < emitted {
			return fmt.Errorf("ADC event line %d reduces the docket from %d entries to %d", event.line, emitted, len(docket))
		}
		for index := emitted; index < len(docket); index++ {
			entry := docket[index]
			reference := Reference{SourceID: "record", Path: stateRelative, JSONPointer: fmt.Sprintf("/case/docket/%d", index)}
			if stateErr != nil {
				reference = Reference{SourceID: "record", Path: b.coreRelative("events.ndjson"), Line: event.line}
			}
			b.addDocket(DocketItem{
				ID:              fmt.Sprintf("adc-docket-%d", index+1),
				Timestamp:       event.timestamp,
				TimestampSource: event.timestampSource,
				Phase:           stringValue(caseState["phase"]),
				Actor:           stringValue(event.value["role"]),
				Kind:            "docket_entry",
				Title:           stringValue(entry["title"]),
				Description:     stringValue(entry["description"]),
				Source:          reference,
			})
		}
		emitted = len(docket)
	}
	timestamp, source := b.stateTimestamp(stateRelative)
	for index := emitted; index < len(finalDocket); index++ {
		entry := finalDocket[index]
		b.addDocket(DocketItem{
			ID:              fmt.Sprintf("adc-docket-%d", index+1),
			Timestamp:       timestamp,
			TimestampSource: source,
			Kind:            "docket_entry",
			Title:           stringValue(entry["title"]),
			Description:     stringValue(entry["description"]),
			Source:          Reference{SourceID: "record", Path: stateRelative, JSONPointer: fmt.Sprintf("/case/docket/%d", index)},
		})
	}
	return nil
}

func (b *builder) adcInitialDocket(finalDocket []map[string]any, stateRelative string) (int, error) {
	certificateRelative := b.coreRelative("certificate.json")
	var certificate map[string]any
	if err := readJSON(filepath.Join(b.layout.recordRoot, filepath.FromSlash(certificateRelative)), &certificate); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("read ADC certificate: %w", err)
	}
	initializeCase := mapValue(mapValue(certificate["initialize_request"])["initialize_case"])
	if len(initializeCase) == 0 {
		return 0, nil
	}
	count := 1 + len(mapList(initializeCase["attachments"]))
	if count > len(finalDocket) {
		return 0, fmt.Errorf("ADC final docket has %d entries, fewer than the %d initialization entries", len(finalDocket), count)
	}
	for index := 0; index < count; index++ {
		entry := finalDocket[index]
		logicalID := "complaint"
		if index > 0 {
			logicalID = fmt.Sprintf("complaint-attachment-%d", index)
		}
		if b.docketKeys[logicalID] {
			continue
		}
		b.addDocket(DocketItem{
			ID:              fmt.Sprintf("adc-docket-%d", index+1),
			Timestamp:       b.layout.manifest.StartedAt.UTC(),
			TimestampSource: "case_manifest",
			Phase:           "initiation",
			Actor:           "plaintiff",
			Kind:            "docket_entry",
			Title:           stringValue(entry["title"]),
			Description:     stringValue(entry["description"]),
			Source:          Reference{SourceID: "record", Path: stateRelative, JSONPointer: fmt.Sprintf("/case/docket/%d", index)},
		})
	}
	return count, nil
}

func (b *builder) addWorkNotes() error {
	var paths []string
	for _, artifact := range b.artifacts {
		if artifact.SourceID == "record" && artifact.Category == "work_notes" {
			paths = append(paths, artifact.Path)
		}
	}
	sort.Strings(paths)
	for _, relative := range paths {
		notes, err := readJSONLines(filepath.Join(b.layout.recordRoot, filepath.FromSlash(relative)))
		if err != nil {
			return fmt.Errorf("read work notes: %w", err)
		}
		for index, note := range notes {
			timestamp, source := firstTimestamp(note, "timestamp", "time")
			if timestamp.IsZero() {
				timestamp, source = b.artifactTime(relative)
			}
			role := firstString(note, "role", "role_id")
			b.addDocket(DocketItem{
				ID:              fmt.Sprintf("work-note-%s-%d", safeID(relative), index+1),
				Timestamp:       timestamp,
				TimestampSource: source,
				Phase:           stringValue(note["phase"]),
				Actor:           role,
				Kind:            "work_note",
				Title:           "Work note from " + role,
				Access:          "work_notes",
				Source:          Reference{SourceID: "record", Path: relative, Line: index + 1},
				ArtifactRefs:    b.artifactReference(relative),
			})
		}
	}
	return nil
}

func (b *builder) readCoreEvents() ([]*recordedEvent, error) {
	relative := b.coreRelative("events.ndjson")
	values, err := readJSONLines(filepath.Join(b.layout.recordRoot, filepath.FromSlash(relative)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read procedure events: %w", err)
	}
	events := make([]*recordedEvent, 0, len(values))
	for index, value := range values {
		timestamp, source := firstTimestamp(value, "timestamp", "time")
		if !timestamp.IsZero() {
			source = "event"
		}
		if timestamp.IsZero() {
			timestamp, source = b.artifactTime(relative)
		}
		events = append(events, &recordedEvent{line: index + 1, timestamp: timestamp, timestampSource: source, value: value})
	}
	return events, nil
}

func (b *builder) stateTimestamp(relative string) (time.Time, string) {
	return b.artifactTime(relative)
}

func matchAttorneyEvent(events []*recordedEvent, phase, action string, filing map[string]any) *recordedEvent {
	for _, event := range events {
		if event.used || firstString(event.value, "type", "event_type") != "attorney_action" || stringValue(event.value["phase"]) != phase {
			continue
		}
		payload := mapValue(event.value["payload"])
		if stringValue(payload["action_type"]) != action || stringValue(event.value["role"]) != stringValue(filing["role"]) {
			continue
		}
		if stringValue(mapValue(payload["payload"])["text"]) == stringValue(filing["text"]) {
			return event
		}
	}
	return nil
}

func matchNestedAttorneyEvent(events []*recordedEvent, item map[string]any, listField, matchField string) *recordedEvent {
	for _, event := range events {
		if firstString(event.value, "type", "event_type") != "attorney_action" {
			continue
		}
		if phase := stringValue(item["phase"]); phase != "" && phase != stringValue(event.value["phase"]) {
			continue
		}
		if role := stringValue(item["role"]); role != "" && role != stringValue(event.value["role"]) {
			continue
		}
		payload := mapValue(mapValue(event.value["payload"])["payload"])
		for _, nested := range mapList(payload[listField]) {
			if stringValue(nested[matchField]) == stringValue(item[matchField]) {
				return event
			}
		}
	}
	return nil
}

func matchEventValue(events []*recordedEvent, eventType, field, value string) *recordedEvent {
	for _, event := range events {
		if event.used || firstString(event.value, "type", "event_type") != eventType {
			continue
		}
		payload := mapValue(event.value["payload"])
		if stringValue(payload[field]) == value {
			return event
		}
	}
	return nil
}

func firstTimestamp(value map[string]any, fields ...string) (time.Time, string) {
	for _, field := range fields {
		if timestamp, ok := parseTime(stringValue(value[field])); ok {
			return timestamp, "record"
		}
	}
	return time.Time{}, ""
}

func parseTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, format := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.000"} {
		var timestamp time.Time
		var err error
		if format == "2006-01-02 15:04:05.000" {
			timestamp, err = time.ParseInLocation(format, value, time.Local)
		} else {
			timestamp, err = time.Parse(format, value)
		}
		if err == nil {
			return timestamp.UTC(), true
		}
	}
	return time.Time{}, false
}

func mapValue(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func mapList(value any) []map[string]any {
	values, _ := value.([]any)
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if item := mapValue(value); item != nil {
			result = append(result, item)
		}
	}
	return result
}

func stringValue(value any) string {
	valueString, _ := value.(string)
	return strings.TrimSpace(valueString)
}

func displayValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return ""
	}
}

func firstString(value map[string]any, fields ...string) string {
	for _, field := range fields {
		if result := stringValue(value[field]); result != "" {
			return result
		}
	}
	return ""
}

func filingKind(phase string) string {
	switch phase {
	case "openings":
		return "opening_statement"
	case "rebuttals":
		return "rebuttal"
	case "surrebuttals":
		return "surrebuttal"
	case "closings":
		return "closing_statement"
	default:
		return "argument"
	}
}

func filingTitle(phase, role string) string {
	return titleCase(role) + " " + strings.ReplaceAll(filingKind(phase), "_", " ")
}

func titleCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func humanize(value string) string {
	words := strings.ReplaceAll(strings.TrimSpace(value), "_", " ")
	return titleCase(words)
}

func safeID(value string) string {
	var b strings.Builder
	lastDash := false
	for _, char := range strings.ToLower(value) {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			b.WriteRune(char)
			lastDash = false
			continue
		}
		if b.Len() > 0 && !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return strconv.Itoa(len(value))
	}
	return result
}
