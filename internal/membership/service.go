package membership

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/eligibility"
	"github.com/ixxet/apollo/internal/store"
)

const (
	StatusJoined    = "joined"
	StatusNotJoined = "not_joined"
)

var (
	ErrNotFound      = errors.New("member not found")
	ErrAlreadyJoined = errors.New("member is already joined")
	ErrNotJoined     = errors.New("member is not currently joined")
	ErrIneligible    = errors.New("member is not eligible for lobby membership")
)

type Clock func() time.Time

type EligibilityReader interface {
	GetLobbyEligibility(ctx context.Context, userID uuid.UUID) (eligibility.LobbyEligibility, error)
}

type Store interface {
	GetUserByID(ctx context.Context, userID uuid.UUID) (*store.ApolloUser, error)
	GetLobbyMembershipByUserID(ctx context.Context, userID uuid.UUID) (*store.ApolloLobbyMembership, error)
	UpsertLobbyMembershipJoin(ctx context.Context, userID uuid.UUID, joinedAt time.Time) (*store.ApolloLobbyMembership, error)
	LeaveLobbyMembership(ctx context.Context, userID uuid.UUID, leftAt time.Time) (*store.ApolloLobbyMembership, error)
}

type Service struct {
	repository        Store
	eligibilityReader EligibilityReader
	now               Clock
}

type LobbyMembership struct {
	Status   string     `json:"status"`
	JoinedAt *time.Time `json:"joined_at,omitempty"`
	LeftAt   *time.Time `json:"left_at,omitempty"`
}

type IneligibleError struct {
	Reason string
}

func (e IneligibleError) Error() string {
	if e.Reason == "" {
		return ErrIneligible.Error()
	}

	return ErrIneligible.Error() + ": " + e.Reason
}

func (e IneligibleError) Unwrap() error {
	return ErrIneligible
}

func NewService(repository Store, eligibilityReader EligibilityReader) *Service {
	return &Service{
		repository:        repository,
		eligibilityReader: eligibilityReader,
		now:               time.Now,
	}
}

func (s *Service) GetLobbyMembership(ctx context.Context, userID uuid.UUID) (LobbyMembership, error) {
	if _, err := s.lookupUser(ctx, userID); err != nil {
		return LobbyMembership{}, err
	}

	record, err := s.repository.GetLobbyMembershipByUserID(ctx, userID)
	if err != nil {
		return LobbyMembership{}, err
	}

	return buildMembership(record), nil
}

func (s *Service) JoinLobbyMembership(ctx context.Context, userID uuid.UUID) (LobbyMembership, error) {
	if _, err := s.lookupUser(ctx, userID); err != nil {
		return LobbyMembership{}, err
	}

	current, err := s.repository.GetLobbyMembershipByUserID(ctx, userID)
	if err != nil {
		return LobbyMembership{}, err
	}
	if current != nil && current.Status == StatusJoined {
		return LobbyMembership{}, ErrAlreadyJoined
	}

	lobbyEligibility, err := s.eligibilityReader.GetLobbyEligibility(ctx, userID)
	if err != nil {
		if errors.Is(err, eligibility.ErrNotFound) {
			return LobbyMembership{}, ErrNotFound
		}
		return LobbyMembership{}, err
	}
	if !lobbyEligibility.Eligible {
		return LobbyMembership{}, IneligibleError{Reason: lobbyEligibility.Reason}
	}

	joinedAt := s.now().UTC()
	record, err := s.repository.UpsertLobbyMembershipJoin(ctx, userID, joinedAt)
	if err != nil {
		return LobbyMembership{}, err
	}
	if record == nil {
		latest, latestErr := s.repository.GetLobbyMembershipByUserID(ctx, userID)
		if latestErr != nil {
			return LobbyMembership{}, latestErr
		}
		if latest != nil && latest.Status == StatusJoined {
			return LobbyMembership{}, ErrAlreadyJoined
		}
		return LobbyMembership{}, ErrAlreadyJoined
	}

	return buildMembership(record), nil
}

func (s *Service) LeaveLobbyMembership(ctx context.Context, userID uuid.UUID) (LobbyMembership, error) {
	if _, err := s.lookupUser(ctx, userID); err != nil {
		return LobbyMembership{}, err
	}

	current, err := s.repository.GetLobbyMembershipByUserID(ctx, userID)
	if err != nil {
		return LobbyMembership{}, err
	}
	if current == nil || current.Status != StatusJoined {
		return LobbyMembership{}, ErrNotJoined
	}

	leftAt := s.now().UTC()
	record, err := s.repository.LeaveLobbyMembership(ctx, userID, leftAt)
	if err != nil {
		return LobbyMembership{}, err
	}
	if record == nil {
		latest, latestErr := s.repository.GetLobbyMembershipByUserID(ctx, userID)
		if latestErr != nil {
			return LobbyMembership{}, latestErr
		}
		if latest == nil || latest.Status != StatusJoined {
			return LobbyMembership{}, ErrNotJoined
		}
		return LobbyMembership{}, ErrNotJoined
	}

	return buildMembership(record), nil
}

func (s *Service) lookupUser(ctx context.Context, userID uuid.UUID) (*store.ApolloUser, error) {
	user, err := s.repository.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}

	return user, nil
}

func buildMembership(record *store.ApolloLobbyMembership) LobbyMembership {
	if record == nil {
		return LobbyMembership{Status: StatusNotJoined}
	}

	return LobbyMembership{
		Status:   record.Status,
		JoinedAt: timePtrFromPg(record.JoinedAt),
		LeftAt:   timePtrFromPg(record.LeftAt),
	}
}

func timePtrFromPg(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}

	timestamp := value.Time.UTC()
	return &timestamp
}
