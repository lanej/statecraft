package domain

import "time"

// SourceEvidence describes GitHub observations, never Statecraft approval or
// normalized infrastructure plans. A snapshot is captured on demand.
type SourceEvidence struct {
	Change          SourceChange
	Files           []SourceFile
	Reviews         []ExternalReviewDecision
	Checks          []SourceCheck
	CapturedAt      time.Time
	ChecksTruncated bool
}

type SourceFile struct {
	ChangedFile
	Patch          string
	PatchAvailable bool
}

type SourceCheck struct {
	Name, Status, Conclusion, URL, EvidenceSource string
}

type SourceChangeList struct {
	Repository RepositoryRef
	Changes    []SourceChange
	Truncated  bool
	CapturedAt time.Time
}
