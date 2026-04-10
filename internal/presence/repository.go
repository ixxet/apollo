package presence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ixxet/apollo/internal/store"
)

type Repository struct {
	db      *pgxpool.Pool
	queries *store.Queries
}

type LinkedVisit struct {
	Visit   store.ApolloVisit
	TapLink store.ApolloVisitTapLink
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db:      db,
		queries: store.New(db),
	}
}

func (r *Repository) ListLinkedVisitsByUserID(ctx context.Context, userID uuid.UUID) ([]LinkedVisit, error) {
	visits, err := r.queries.ListLinkedVisitsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	tapLinks, err := r.queries.ListVisitTapLinksByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	tapLinksByVisitID := make(map[uuid.UUID]store.ApolloVisitTapLink, len(tapLinks))
	for _, tapLink := range tapLinks {
		tapLinksByVisitID[tapLink.VisitID] = tapLink
	}

	result := make([]LinkedVisit, 0, len(visits))
	for _, visit := range visits {
		tapLink, ok := tapLinksByVisitID[visit.ID]
		if !ok {
			return nil, fmt.Errorf("linked visit %s is missing tap-link metadata", visit.ID)
		}
		result = append(result, LinkedVisit{
			Visit:   visit,
			TapLink: tapLink,
		})
	}

	return result, nil
}

func (r *Repository) ListFacilityStreaksByUserID(ctx context.Context, userID uuid.UUID) ([]store.ApolloMemberPresenceStreak, error) {
	return r.queries.ListFacilityPresenceStreaksByUserID(ctx, userID)
}

func (r *Repository) ListLatestFacilityStreakEventsByUserID(ctx context.Context, userID uuid.UUID) ([]store.ApolloMemberPresenceStreakEvent, error) {
	return r.queries.ListLatestMemberPresenceStreakEventsByUserID(ctx, userID)
}

func (r *Repository) EnsureLinkedVisitAndCredit(ctx context.Context, visit store.ApolloVisit, tagHash string, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := store.New(tx)
	_, err = queries.GetVisitTapLinkByVisitID(ctx, visit.ID)
	switch {
	case err == nil:
		// The visit is already explicitly linked; preserve that truth and keep the
		// replay path idempotent even if claimed-tag state changed later.
	case errors.Is(err, pgx.ErrNoRows):
		claimedTag, claimErr := queries.GetActiveClaimedTagByHash(ctx, tagHash)
		if claimErr != nil {
			if errors.Is(claimErr, pgx.ErrNoRows) {
				return fmt.Errorf("presence tap-link requires active claimed tag %q", tagHash)
			}
			return claimErr
		}
		if claimedTag.UserID != visit.UserID {
			return fmt.Errorf("presence tap-link user mismatch for visit %s", visit.ID)
		}

		inserted, insertErr := queries.InsertVisitTapLink(ctx, store.InsertVisitTapLinkParams{
			VisitID: visit.ID,
			ID:      claimedTag.ID,
			UserID:  visit.UserID,
		})
		if insertErr != nil {
			return insertErr
		}
		if inserted == 0 {
			return fmt.Errorf("presence tap-link was not ensured for visit %s", visit.ID)
		}
	case err != nil:
		return err
	}

	if err := creditFacilityStreak(ctx, queries, visit, now.UTC()); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func creditFacilityStreak(ctx context.Context, queries *store.Queries, visit store.ApolloVisit, now time.Time) error {
	if !visit.ArrivedAt.Valid {
		return fmt.Errorf("visit %s is missing arrived_at", visit.ID)
	}

	creditDay := normalizeUTCDay(visit.ArrivedAt.Time)
	state, err := queries.GetMemberPresenceStreakByUserIDAndFacilityKeyForUpdate(ctx, store.GetMemberPresenceStreakByUserIDAndFacilityKeyForUpdateParams{
		UserID:      visit.UserID,
		FacilityKey: visit.FacilityKey,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	eventKind := "started"
	countBefore := int32(0)
	countAfter := int32(1)
	currentStartDay := creditDay

	if err == nil {
		lastCreditedDay := state.LastCreditedDay.Time.UTC()
		switch {
		case sameUTCDay(lastCreditedDay, creditDay):
			return nil
		case lastCreditedDay.After(creditDay):
			return nil
		case normalizeUTCDay(lastCreditedDay).AddDate(0, 0, 1).Equal(creditDay):
			eventKind = "continued"
			countBefore = state.CurrentCount
			countAfter = state.CurrentCount + 1
			currentStartDay = state.CurrentStartDay.Time.UTC()
		default:
			eventKind = "reset"
			countBefore = state.CurrentCount
		}
	}

	inserted, err := queries.InsertMemberPresenceStreakEvent(ctx, store.InsertMemberPresenceStreakEventParams{
		UserID:      visit.UserID,
		FacilityKey: visit.FacilityKey,
		EventKind:   eventKind,
		CountBefore: countBefore,
		CountAfter:  countAfter,
		StreakDay:   pgDate(creditDay),
		VisitID:     visit.ID,
	})
	if err != nil {
		return err
	}
	if inserted == 0 {
		return nil
	}

	_, err = queries.UpsertMemberPresenceStreak(ctx, store.UpsertMemberPresenceStreakParams{
		UserID:            visit.UserID,
		FacilityKey:       visit.FacilityKey,
		CurrentCount:      countAfter,
		CurrentStartDay:   pgDate(currentStartDay),
		LastCreditedDay:   pgDate(creditDay),
		LastLinkedVisitID: visit.ID,
		UpdatedAt:         pgTimestamp(now),
	})
	return err
}

func normalizeUTCDay(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func sameUTCDay(left time.Time, right time.Time) bool {
	return normalizeUTCDay(left).Equal(normalizeUTCDay(right))
}

func pgDate(value time.Time) pgtype.Date {
	return pgtype.Date{Time: normalizeUTCDay(value), Valid: true}
}

func pgTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}
