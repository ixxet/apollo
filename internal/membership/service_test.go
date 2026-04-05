package membership

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/eligibility"
	"github.com/ixxet/apollo/internal/store"
)

type stubStore struct {
	user             *store.ApolloUser
	membership       *store.ApolloLobbyMembership
	getUserErr       error
	getMembershipErr error
	joinResult       *store.ApolloLobbyMembership
	joinErr          error
	leaveResult      *store.ApolloLobbyMembership
	leaveErr         error
	joinedAtRecorded *time.Time
	leftAtRecorded   *time.Time
}

func (s *stubStore) GetUserByID(context.Context, uuid.UUID) (*store.ApolloUser, error) {
	return s.user, s.getUserErr
}

func (s *stubStore) GetLobbyMembershipByUserID(context.Context, uuid.UUID) (*store.ApolloLobbyMembership, error) {
	return s.membership, s.getMembershipErr
}

func (s *stubStore) UpsertLobbyMembershipJoin(_ context.Context, _ uuid.UUID, joinedAt time.Time) (*store.ApolloLobbyMembership, error) {
	s.joinedAtRecorded = &joinedAt
	return s.joinResult, s.joinErr
}

func (s *stubStore) LeaveLobbyMembership(_ context.Context, _ uuid.UUID, leftAt time.Time) (*store.ApolloLobbyMembership, error) {
	s.leftAtRecorded = &leftAt
	return s.leaveResult, s.leaveErr
}

type stubEligibilityReader struct {
	response eligibility.LobbyEligibility
	err      error
}

func (s stubEligibilityReader) GetLobbyEligibility(context.Context, uuid.UUID) (eligibility.LobbyEligibility, error) {
	if s.err != nil {
		return eligibility.LobbyEligibility{}, s.err
	}
	return s.response, nil
}

func TestGetLobbyMembershipReturnsExplicitNotJoinedWhenNoRecordExists(t *testing.T) {
	svc := NewService(&stubStore{
		user: &store.ApolloUser{ID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")},
	}, stubEligibilityReader{})

	membership, err := svc.GetLobbyMembership(context.Background(), uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"))
	if err != nil {
		t.Fatalf("GetLobbyMembership() error = %v", err)
	}
	if membership.Status != StatusNotJoined {
		t.Fatalf("membership.Status = %q, want %q", membership.Status, StatusNotJoined)
	}
	if membership.JoinedAt != nil {
		t.Fatalf("membership.JoinedAt = %#v, want nil", membership.JoinedAt)
	}
}

func TestJoinLobbyMembershipRequiresEligibleMemberAndJoinedTransition(t *testing.T) {
	now := time.Date(2026, 4, 5, 14, 0, 0, 0, time.UTC)
	storeStub := &stubStore{
		user: &store.ApolloUser{ID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")},
		joinResult: &store.ApolloLobbyMembership{
			UserID:   uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
			Status:   StatusJoined,
			JoinedAt: pgTime(now),
		},
	}
	svc := NewService(storeStub, stubEligibilityReader{
		response: eligibility.LobbyEligibility{
			Eligible: true,
			Reason:   eligibility.ReasonEligible,
		},
	})
	svc.now = func() time.Time { return now }

	membership, err := svc.JoinLobbyMembership(context.Background(), uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"))
	if err != nil {
		t.Fatalf("JoinLobbyMembership() error = %v", err)
	}
	if membership.Status != StatusJoined {
		t.Fatalf("membership.Status = %q, want %q", membership.Status, StatusJoined)
	}
	if storeStub.joinedAtRecorded == nil || !storeStub.joinedAtRecorded.Equal(now) {
		t.Fatalf("storeStub.joinedAtRecorded = %#v, want %s", storeStub.joinedAtRecorded, now)
	}
}

func TestJoinLobbyMembershipRejectsIneligibleAndAlreadyJoinedStates(t *testing.T) {
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	t.Run("already joined is deterministic conflict", func(t *testing.T) {
		svc := NewService(&stubStore{
			user: &store.ApolloUser{ID: userID},
			membership: &store.ApolloLobbyMembership{
				UserID:   userID,
				Status:   StatusJoined,
				JoinedAt: pgTime(time.Date(2026, 4, 5, 14, 0, 0, 0, time.UTC)),
			},
		}, stubEligibilityReader{})

		_, err := svc.JoinLobbyMembership(context.Background(), userID)
		if !errors.Is(err, ErrAlreadyJoined) {
			t.Fatalf("JoinLobbyMembership() error = %v, want %v", err, ErrAlreadyJoined)
		}
	})

	t.Run("ineligible member returns explicit ineligible error", func(t *testing.T) {
		svc := NewService(&stubStore{
			user: &store.ApolloUser{ID: userID},
		}, stubEligibilityReader{
			response: eligibility.LobbyEligibility{
				Eligible: false,
				Reason:   eligibility.ReasonVisibilityGhost,
			},
		})

		_, err := svc.JoinLobbyMembership(context.Background(), userID)
		if !errors.Is(err, ErrIneligible) {
			t.Fatalf("JoinLobbyMembership() error = %v, want %v", err, ErrIneligible)
		}
		if err == nil || err.Error() != "member is not eligible for lobby membership: visibility_ghost" {
			t.Fatalf("JoinLobbyMembership() error = %v, want explicit reason", err)
		}
	})
}

func TestLeaveLobbyMembershipTransitionsOnceAndRejectsRepeatedLeave(t *testing.T) {
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	now := time.Date(2026, 4, 5, 15, 0, 0, 0, time.UTC)

	t.Run("joined member can leave", func(t *testing.T) {
		storeStub := &stubStore{
			user: &store.ApolloUser{ID: userID},
			membership: &store.ApolloLobbyMembership{
				UserID:   userID,
				Status:   StatusJoined,
				JoinedAt: pgTime(time.Date(2026, 4, 5, 14, 0, 0, 0, time.UTC)),
			},
			leaveResult: &store.ApolloLobbyMembership{
				UserID:   userID,
				Status:   StatusNotJoined,
				JoinedAt: pgTime(time.Date(2026, 4, 5, 14, 0, 0, 0, time.UTC)),
				LeftAt:   pgTime(now),
			},
		}
		svc := NewService(storeStub, stubEligibilityReader{})
		svc.now = func() time.Time { return now }

		membership, err := svc.LeaveLobbyMembership(context.Background(), userID)
		if err != nil {
			t.Fatalf("LeaveLobbyMembership() error = %v", err)
		}
		if membership.Status != StatusNotJoined {
			t.Fatalf("membership.Status = %q, want %q", membership.Status, StatusNotJoined)
		}
		if storeStub.leftAtRecorded == nil || !storeStub.leftAtRecorded.Equal(now) {
			t.Fatalf("storeStub.leftAtRecorded = %#v, want %s", storeStub.leftAtRecorded, now)
		}
	})

	t.Run("repeat leave is deterministic conflict", func(t *testing.T) {
		svc := NewService(&stubStore{
			user: &store.ApolloUser{ID: userID},
		}, stubEligibilityReader{})

		_, err := svc.LeaveLobbyMembership(context.Background(), userID)
		if !errors.Is(err, ErrNotJoined) {
			t.Fatalf("LeaveLobbyMembership() error = %v, want %v", err, ErrNotJoined)
		}
	})
}

func pgTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}
