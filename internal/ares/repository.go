package ares

import (
	"context"

	"github.com/ixxet/apollo/internal/store"
)

type Repository struct {
	queries *store.Queries
}

func NewRepository(db store.DBTX) *Repository {
	return &Repository{
		queries: store.New(db),
	}
}

func (r *Repository) ListJoinedLobbyMatchPreviewCandidates(ctx context.Context) ([]JoinedLobbyCandidate, error) {
	rows, err := r.queries.ListJoinedLobbyMatchPreviewCandidates(ctx)
	if err != nil {
		return nil, err
	}

	candidates := make([]JoinedLobbyCandidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, JoinedLobbyCandidate{
			UserID:              row.UserID,
			Preferences:         row.Preferences,
			UserUpdatedAt:       row.UserUpdatedAt.Time.UTC(),
			JoinedAt:            row.JoinedAt.Time.UTC(),
			MembershipUpdatedAt: row.MembershipUpdatedAt.Time.UTC(),
		})
	}

	return candidates, nil
}
