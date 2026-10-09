package ports

import (
	"context"

	"github.com/lanej/statecraft/internal/domain"
)

// SourceEvidenceReader is intentionally incapable of issuing provider commands.
type SourceEvidenceReader interface {
	ListChanges(context.Context) (domain.SourceChangeList, error)
	GetSourceEvidence(context.Context, int64) (domain.SourceEvidence, error)
}
