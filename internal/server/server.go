package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/ares"
	"github.com/ixxet/apollo/internal/athena"
	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/booking"
	"github.com/ixxet/apollo/internal/coaching"
	"github.com/ixxet/apollo/internal/competition"
	"github.com/ixxet/apollo/internal/eligibility"
	"github.com/ixxet/apollo/internal/exercises"
	"github.com/ixxet/apollo/internal/helper"
	"github.com/ixxet/apollo/internal/membership"
	"github.com/ixxet/apollo/internal/nutrition"
	"github.com/ixxet/apollo/internal/ops"
	"github.com/ixxet/apollo/internal/planner"
	"github.com/ixxet/apollo/internal/presence"
	"github.com/ixxet/apollo/internal/profile"
	"github.com/ixxet/apollo/internal/recommendations"
	"github.com/ixxet/apollo/internal/schedule"
	"github.com/ixxet/apollo/internal/workouts"
)

type Authenticator interface {
	StartVerification(ctx context.Context, input auth.StartVerificationInput) error
	VerifyToken(ctx context.Context, rawToken string) (auth.VerifiedSession, error)
	AuthenticateSession(ctx context.Context, cookieValue string) (auth.Principal, error)
	LogoutSession(ctx context.Context, cookieValue string) error
	SessionCookie(sessionID uuid.UUID, expiresAt time.Time) *http.Cookie
	ExpiredSessionCookie() *http.Cookie
	SessionCookieName() string
}

type Profiler interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (profile.MemberProfile, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, input profile.UpdateInput) (profile.MemberProfile, error)
}

type PresenceReader interface {
	GetSummary(ctx context.Context, userID uuid.UUID) (presence.Summary, error)
}

type PresenceClaimManager interface {
	ListClaims(ctx context.Context, userID uuid.UUID) ([]presence.Claim, error)
	Claim(ctx context.Context, userID uuid.UUID, input presence.ClaimInput) (presence.Claim, error)
}

type MemberFacilityReader interface {
	ListMemberFacilities(ctx context.Context, userID uuid.UUID) ([]presence.MemberFacility, error)
	GetMemberFacilityCalendar(ctx context.Context, userID uuid.UUID, facilityKey string, window schedule.CalendarWindow) (presence.MemberFacilityCalendar, error)
}

type ExerciseCatalogReader interface {
	ListExercises(ctx context.Context) ([]exercises.ExerciseDefinition, error)
	ListEquipment(ctx context.Context) ([]exercises.EquipmentDefinition, error)
}

type PlannerManager interface {
	ListTemplates(ctx context.Context, userID uuid.UUID) ([]planner.Template, error)
	GetTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID) (planner.Template, error)
	CreateTemplate(ctx context.Context, userID uuid.UUID, input planner.TemplateInput) (planner.Template, error)
	UpdateTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID, input planner.TemplateInput) (planner.Template, error)
	GetWeek(ctx context.Context, userID uuid.UUID, weekStart string) (planner.Week, error)
	PutWeek(ctx context.Context, userID uuid.UUID, weekStart string, input planner.WeekInput) (planner.Week, error)
}

type CompetitionManager interface {
	ListSessions(ctx context.Context) ([]competition.SessionSummary, error)
	GetSession(ctx context.Context, sessionID uuid.UUID) (competition.Session, error)
	ListMemberStats(ctx context.Context, userID uuid.UUID) ([]competition.MemberStat, error)
	CompetitionReadiness(actor competition.StaffActor) competition.CompetitionCommandReadiness
	ExecuteCommand(ctx context.Context, command competition.CompetitionCommand) (competition.CompetitionCommandOutcome, error)
	CreateSession(ctx context.Context, actor competition.StaffActor, input competition.CreateSessionInput) (competition.Session, error)
	OpenQueue(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID) (competition.Session, error)
	AddQueueMember(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID, input competition.QueueMemberInput) (competition.Session, error)
	RemoveQueueMember(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID, userID uuid.UUID) (competition.Session, error)
	AssignQueue(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID, input competition.AssignSessionInput) (competition.Session, error)
	StartSession(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID) (competition.Session, error)
	ArchiveSession(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID) (competition.Session, error)
	CreateTeam(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID, input competition.CreateTeamInput) (competition.Team, error)
	RemoveTeam(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID, teamID uuid.UUID) error
	AddRosterMember(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID, teamID uuid.UUID, input competition.AddRosterMemberInput) (competition.Team, error)
	RemoveRosterMember(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID, teamID uuid.UUID, userID uuid.UUID) error
	CreateMatch(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID, input competition.CreateMatchInput) (competition.Match, error)
	ArchiveMatch(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID, matchID uuid.UUID) (competition.Match, error)
	RecordMatchResult(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID, matchID uuid.UUID, input competition.RecordMatchResultInput) (competition.Session, error)
	FinalizeMatchResult(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID, matchID uuid.UUID, expectedResultVersion int) (competition.Session, error)
	DisputeMatchResult(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID, matchID uuid.UUID, expectedResultVersion int) (competition.Session, error)
	CorrectMatchResult(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID, matchID uuid.UUID, input competition.RecordMatchResultInput) (competition.Session, error)
	VoidMatchResult(ctx context.Context, actor competition.StaffActor, sessionID uuid.UUID, matchID uuid.UUID, expectedResultVersion int) (competition.Session, error)
}

type CompetitionHistoryReader interface {
	ListMemberHistory(ctx context.Context, userID uuid.UUID) ([]competition.MemberHistoryEntry, error)
}

type EligibilityReader interface {
	GetLobbyEligibility(ctx context.Context, userID uuid.UUID) (eligibility.LobbyEligibility, error)
}

type MembershipManager interface {
	GetLobbyMembership(ctx context.Context, userID uuid.UUID) (membership.LobbyMembership, error)
	JoinLobbyMembership(ctx context.Context, userID uuid.UUID) (membership.LobbyMembership, error)
	LeaveLobbyMembership(ctx context.Context, userID uuid.UUID) (membership.LobbyMembership, error)
}

type MatchPreviewReader interface {
	GetLobbyMatchPreview(ctx context.Context) (ares.MatchPreview, error)
}

type RecommendationReader interface {
	GetWorkoutRecommendation(ctx context.Context, userID uuid.UUID) (recommendations.WorkoutRecommendation, error)
}

type ScheduleManager interface {
	ListBlocks(ctx context.Context, facilityKey string) ([]schedule.Block, error)
	GetCalendar(ctx context.Context, facilityKey string, window schedule.CalendarWindow) ([]schedule.Occurrence, error)
	CreateBlock(ctx context.Context, actor schedule.StaffActor, input schedule.BlockInput) (schedule.Block, error)
	AddException(ctx context.Context, actor schedule.StaffActor, blockID uuid.UUID, expectedVersion int, input schedule.BlockExceptionInput) (schedule.Block, error)
	CancelBlock(ctx context.Context, actor schedule.StaffActor, blockID uuid.UUID, expectedVersion int) (schedule.Block, error)
}

type BookingManager interface {
	ListRequests(ctx context.Context, facilityKey string) ([]booking.Request, error)
	GetRequest(ctx context.Context, requestID uuid.UUID) (booking.Request, error)
	CreateRequestWithIdempotency(ctx context.Context, actor booking.StaffActor, idempotencyKey string, input booking.RequestInput) (booking.Request, error)
	UpdateRequest(ctx context.Context, actor booking.StaffActor, requestID uuid.UUID, input booking.RequestEditInput) (booking.Request, error)
	RebookRequestWithIdempotency(ctx context.Context, actor booking.StaffActor, requestID uuid.UUID, idempotencyKey string, input booking.RequestEditInput) (booking.Request, error)
	ListPublicOptions(ctx context.Context) ([]booking.PublicOption, error)
	GetPublicAvailability(ctx context.Context, input booking.PublicAvailabilityInput) (booking.PublicAvailability, error)
	CreatePublicRequest(ctx context.Context, channel string, idempotencyKey string, input booking.PublicRequestInput) (booking.PublicReceipt, error)
	GetPublicStatus(ctx context.Context, receiptCode string) (booking.PublicStatus, error)
	UpdatePublicMessage(ctx context.Context, actor booking.StaffActor, requestID uuid.UUID, input booking.PublicMessageInput) (booking.Request, error)
	StartReview(ctx context.Context, actor booking.StaffActor, requestID uuid.UUID, input booking.TransitionInput) (booking.Request, error)
	NeedsChanges(ctx context.Context, actor booking.StaffActor, requestID uuid.UUID, input booking.TransitionInput) (booking.Request, error)
	Reject(ctx context.Context, actor booking.StaffActor, requestID uuid.UUID, input booking.TransitionInput) (booking.Request, error)
	Cancel(ctx context.Context, actor booking.StaffActor, requestID uuid.UUID, input booking.TransitionInput) (booking.Request, error)
	Approve(ctx context.Context, actor booking.StaffActor, requestID uuid.UUID, input booking.TransitionInput) (booking.Request, error)
}

type OpsOverviewReader interface {
	GetFacilityOverview(ctx context.Context, input ops.FacilityOverviewInput) (ops.FacilityOverview, error)
}

type CoachingManager interface {
	GetCoachingRecommendation(ctx context.Context, userID uuid.UUID, weekStart string) (coaching.CoachingRecommendation, error)
	GetHelperRead(ctx context.Context, userID uuid.UUID, weekStart string) (coaching.CoachingHelperRead, error)
	AskWhy(ctx context.Context, userID uuid.UUID, weekStart string, topic string) (coaching.CoachingHelperWhy, error)
	PreviewVariation(ctx context.Context, userID uuid.UUID, weekStart string, variation string) (coaching.CoachingVariationPreview, error)
	PutEffortFeedback(ctx context.Context, userID uuid.UUID, workoutID uuid.UUID, input coaching.EffortFeedbackInput) (coaching.EffortFeedback, error)
	PutRecoveryFeedback(ctx context.Context, userID uuid.UUID, workoutID uuid.UUID, input coaching.RecoveryFeedbackInput) (coaching.RecoveryFeedback, error)
}

type NutritionManager interface {
	ListMealLogs(ctx context.Context, userID uuid.UUID) ([]nutrition.MealLog, error)
	CreateMealLog(ctx context.Context, userID uuid.UUID, input nutrition.MealLogInput) (nutrition.MealLog, error)
	UpdateMealLog(ctx context.Context, userID uuid.UUID, mealLogID uuid.UUID, input nutrition.MealLogInput) (nutrition.MealLog, error)
	ListMealTemplates(ctx context.Context, userID uuid.UUID) ([]nutrition.MealTemplate, error)
	CreateMealTemplate(ctx context.Context, userID uuid.UUID, input nutrition.MealTemplateInput) (nutrition.MealTemplate, error)
	UpdateMealTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID, input nutrition.MealTemplateInput) (nutrition.MealTemplate, error)
	GetRecommendation(ctx context.Context, userID uuid.UUID) (nutrition.Recommendation, error)
	GetHelperRead(ctx context.Context, userID uuid.UUID) (nutrition.HelperRead, error)
	AskWhy(ctx context.Context, userID uuid.UUID, topic string) (nutrition.HelperWhy, error)
	PreviewVariation(ctx context.Context, userID uuid.UUID, variation string) (nutrition.VariationPreview, error)
}

type WorkoutManager interface {
	CreateWorkout(ctx context.Context, userID uuid.UUID, input workouts.CreateInput) (workouts.Workout, error)
	ListWorkouts(ctx context.Context, userID uuid.UUID) ([]workouts.Workout, error)
	GetWorkout(ctx context.Context, userID uuid.UUID, workoutID uuid.UUID) (workouts.Workout, error)
	UpdateWorkout(ctx context.Context, userID uuid.UUID, workoutID uuid.UUID, input workouts.UpdateInput) (workouts.Workout, error)
	FinishWorkout(ctx context.Context, userID uuid.UUID, workoutID uuid.UUID) (workouts.Workout, error)
}

type Dependencies struct {
	ConsumerEnabled    bool
	Auth               Authenticator
	Competition        CompetitionManager
	CompetitionHistory CompetitionHistoryReader
	Profile            Profiler
	Presence           PresenceReader
	PresenceClaims     PresenceClaimManager
	MemberFacilities   MemberFacilityReader
	Exercises          ExerciseCatalogReader
	Planner            PlannerManager
	Eligibility        EligibilityReader
	Membership         MembershipManager
	MatchPreview       MatchPreviewReader
	Recommendations    RecommendationReader
	Schedule           ScheduleManager
	Booking            BookingManager
	Ops                OpsOverviewReader
	Coaching           CoachingManager
	Nutrition          NutritionManager
	Workouts           WorkoutManager
}

type healthResponse struct {
	Service         string `json:"service"`
	Status          string `json:"status"`
	ConsumerEnabled bool   `json:"consumer_enabled"`
}

type statusResponse struct {
	Status string `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type startVerificationRequest struct {
	StudentID string `json:"student_id"`
	Email     string `json:"email"`
}

type verifyRequest struct {
	Token string `json:"token"`
}

type presenceClaimRequest struct {
	TagHash string  `json:"tag_hash"`
	Label   *string `json:"label"`
}

type createCompetitionSessionRequest struct {
	DisplayName         string  `json:"display_name"`
	SportKey            string  `json:"sport_key"`
	FacilityKey         string  `json:"facility_key"`
	ZoneKey             *string `json:"zone_key"`
	ParticipantsPerSide int     `json:"participants_per_side"`
}

type createCompetitionTeamRequest struct {
	SideIndex int `json:"side_index"`
}

type addCompetitionQueueMemberRequest struct {
	UserID uuid.UUID `json:"user_id"`
	Tier   string    `json:"tier,omitempty"`
}

type addCompetitionRosterMemberRequest struct {
	UserID    uuid.UUID `json:"user_id"`
	SlotIndex int       `json:"slot_index"`
}

type assignCompetitionSessionRequest struct {
	ExpectedQueueVersion int `json:"expected_queue_version"`
}

type createCompetitionMatchRequest struct {
	MatchIndex int                          `json:"match_index"`
	SideSlots  []competition.MatchSideInput `json:"side_slots"`
}

type recordCompetitionMatchResultRequest struct {
	ExpectedResultVersion *int                               `json:"expected_result_version"`
	Sides                 []competition.MatchResultSideInput `json:"sides"`
}

type competitionMatchResultTransitionRequest struct {
	ExpectedResultVersion *int `json:"expected_result_version"`
}

type scheduleBlockMutationRequest struct {
	ExpectedVersion int                 `json:"expected_version"`
	ExceptionDate   string              `json:"exception_date,omitempty"`
	Block           schedule.BlockInput `json:"block,omitempty"`
}

type createWorkoutRequest struct {
	Notes *string `json:"notes"`
}

type updateWorkoutRequest struct {
	Notes     *string                   `json:"notes"`
	Exercises *[]workouts.ExerciseInput `json:"exercises"`
}

type putEffortFeedbackRequest struct {
	EffortLevel string `json:"effort_level"`
}

type putRecoveryFeedbackRequest struct {
	RecoveryLevel string `json:"recovery_level"`
}

type contextKey string

const principalContextKey contextKey = "session_principal"

const maxJSONBodyBytes int64 = 1 << 20
const maxPublicBookingJSONBodyBytes int64 = 16 << 10

func NewHandler(deps Dependencies) http.Handler {
	router := chi.NewRouter()
	trustedSurfaceVerifier := authz.NewTrustedSurfaceVerifierFromEnv()
	registerWebUIRoutes(router, deps)
	router.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{
			Service:         "apollo",
			Status:          "ok",
			ConsumerEnabled: deps.ConsumerEnabled,
		})
	})

	router.Post("/api/v1/auth/verification/start", func(w http.ResponseWriter, r *http.Request) {
		var request startVerificationRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		err := deps.Auth.StartVerification(r.Context(), auth.StartVerificationInput{
			StudentID: request.StudentID,
			Email:     request.Email,
		})
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrInvalidStudentID), errors.Is(err, auth.ErrInvalidEmail):
				writeError(w, http.StatusBadRequest, err)
			case errors.Is(err, auth.ErrDuplicateStudentID), errors.Is(err, auth.ErrDuplicateEmail), errors.Is(err, auth.ErrRegistrationConflict):
				writeError(w, http.StatusConflict, err)
			default:
				writeError(w, http.StatusInternalServerError, err)
			}
			return
		}

		writeJSON(w, http.StatusAccepted, statusResponse{Status: "verification_started"})
	})

	verifyHandler := func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" && r.Method == http.MethodPost {
			var request verifyRequest
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			token = request.Token
		}

		session, err := deps.Auth.VerifyToken(r.Context(), token)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrMissingToken), errors.Is(err, auth.ErrVerificationTokenInvalid):
				writeError(w, http.StatusBadRequest, err)
			case errors.Is(err, auth.ErrVerificationTokenUsed):
				writeError(w, http.StatusConflict, err)
			case errors.Is(err, auth.ErrVerificationTokenExpired):
				writeError(w, http.StatusGone, err)
			case errors.Is(err, auth.ErrVerificationUnknownUser):
				writeError(w, http.StatusNotFound, err)
			default:
				writeError(w, http.StatusInternalServerError, err)
			}
			return
		}

		http.SetCookie(w, deps.Auth.SessionCookie(session.SessionID, session.ExpiresAt))
		writeJSON(w, http.StatusOK, statusResponse{Status: "verified"})
	}
	router.Get("/api/v1/auth/verify", verifyHandler)
	router.Post("/api/v1/auth/verify", verifyHandler)

	router.Get("/api/v1/public/booking/options", func(w http.ResponseWriter, r *http.Request) {
		if deps.Booking == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("booking request dependency is unavailable"))
			return
		}

		options, err := deps.Booking.ListPublicOptions(r.Context())
		if err != nil {
			writePublicBookingError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, options)
	})
	router.Get("/api/v1/public/booking/options/{optionID}/availability", func(w http.ResponseWriter, r *http.Request) {
		if deps.Booking == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("booking request dependency is unavailable"))
			return
		}

		optionID, err := parseUUIDParam(r, "optionID")
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		from, err := parseScheduleWindowBoundary(r.URL.Query().Get("from"), false)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		until, err := parseScheduleWindowBoundary(r.URL.Query().Get("until"), true)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		availability, err := deps.Booking.GetPublicAvailability(r.Context(), booking.PublicAvailabilityInput{
			OptionID: optionID,
			From:     from,
			Until:    until,
		})
		if err != nil {
			writePublicBookingError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, availability)
	})
	router.Post("/api/v1/public/booking/requests", func(w http.ResponseWriter, r *http.Request) {
		if deps.Booking == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("booking request dependency is unavailable"))
			return
		}

		var request booking.PublicRequestInput
		if err := decodeJSONBodyWithLimit(w, r, &request, maxPublicBookingJSONBodyBytes); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		receipt, err := deps.Booking.CreatePublicRequest(
			r.Context(),
			r.Header.Get("X-Apollo-Public-Intake-Channel"),
			r.Header.Get("Idempotency-Key"),
			request,
		)
		if err != nil {
			writePublicBookingError(w, err)
			return
		}

		writeJSON(w, http.StatusAccepted, receipt)
	})
	router.Get("/api/v1/public/booking/requests/status", func(w http.ResponseWriter, r *http.Request) {
		if deps.Booking == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("booking request dependency is unavailable"))
			return
		}

		status, err := deps.Booking.GetPublicStatus(r.Context(), r.URL.Query().Get("receipt_code"))
		if err != nil {
			writePublicBookingError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, status)
	})

	router.Group(func(authenticated chi.Router) {
		authenticated.Use(sessionMiddleware(deps.Auth))
		authenticated.Get("/api/v1/profile", func(w http.ResponseWriter, r *http.Request) {
			principal := principalFromContext(r.Context())
			memberProfile, err := deps.Profile.GetProfile(r.Context(), principal.UserID)
			if err != nil {
				switch {
				case errors.Is(err, profile.ErrNotFound):
					writeError(w, http.StatusNotFound, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			writeJSON(w, http.StatusOK, memberProfile)
		})
		authenticated.Get("/api/v1/presence", func(w http.ResponseWriter, r *http.Request) {
			if deps.Presence == nil {
				writeError(w, http.StatusInternalServerError, errors.New("presence dependency is unavailable"))
				return
			}

			principal := principalFromContext(r.Context())
			summary, err := deps.Presence.GetSummary(r.Context(), principal.UserID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}

			writeJSON(w, http.StatusOK, summary)
		})
		authenticated.Get("/api/v1/presence/claims", func(w http.ResponseWriter, r *http.Request) {
			if deps.PresenceClaims == nil {
				writeError(w, http.StatusInternalServerError, errors.New("presence claim dependency is unavailable"))
				return
			}

			principal := principalFromContext(r.Context())
			claims, err := deps.PresenceClaims.ListClaims(r.Context(), principal.UserID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}

			writeJSON(w, http.StatusOK, claims)
		})
		authenticated.Post("/api/v1/presence/claims", func(w http.ResponseWriter, r *http.Request) {
			if deps.PresenceClaims == nil {
				writeError(w, http.StatusInternalServerError, errors.New("presence claim dependency is unavailable"))
				return
			}

			var request presenceClaimRequest
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			claim, err := deps.PresenceClaims.Claim(r.Context(), principal.UserID, presence.ClaimInput{
				TagHash: request.TagHash,
				Label:   request.Label,
			})
			if err != nil {
				switch {
				case errors.Is(err, presence.ErrClaimTagHashRequired), errors.Is(err, presence.ErrClaimTagHashInvalid):
					writeError(w, http.StatusBadRequest, err)
				case errors.Is(err, presence.ErrClaimAlreadyActive), errors.Is(err, presence.ErrClaimInactive), errors.Is(err, presence.ErrClaimOwnedByAnotherMember):
					writeError(w, http.StatusConflict, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			writeJSON(w, http.StatusCreated, claim)
		})
		authenticated.Get("/api/v1/presence/facilities", func(w http.ResponseWriter, r *http.Request) {
			if deps.MemberFacilities == nil {
				writeError(w, http.StatusInternalServerError, errors.New("member facility dependency is unavailable"))
				return
			}

			principal := principalFromContext(r.Context())
			facilities, err := deps.MemberFacilities.ListMemberFacilities(r.Context(), principal.UserID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}

			writeJSON(w, http.StatusOK, facilities)
		})
		authenticated.Get("/api/v1/presence/facilities/{facilityKey}/calendar", func(w http.ResponseWriter, r *http.Request) {
			if deps.MemberFacilities == nil {
				writeError(w, http.StatusInternalServerError, errors.New("member facility dependency is unavailable"))
				return
			}

			facilityKey := strings.TrimSpace(chi.URLParam(r, "facilityKey"))
			from, err := parseScheduleWindowBoundary(r.URL.Query().Get("from"), false)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			until, err := parseScheduleWindowBoundary(r.URL.Query().Get("until"), true)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			calendar, err := deps.MemberFacilities.GetMemberFacilityCalendar(r.Context(), principal.UserID, facilityKey, schedule.CalendarWindow{
				From:  from,
				Until: until,
			})
			if err != nil {
				switch {
				case errors.Is(err, presence.ErrMemberFacilityKeyRequired),
					errors.Is(err, presence.ErrMemberCalendarWindowInvalid),
					errors.Is(err, presence.ErrMemberCalendarWindowTooWide):
					writeError(w, http.StatusBadRequest, err)
				case errors.Is(err, presence.ErrMemberFacilityNotFound):
					writeError(w, http.StatusNotFound, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			writeJSON(w, http.StatusOK, calendar)
		})
		authenticated.Get("/api/v1/planner/exercises", func(w http.ResponseWriter, r *http.Request) {
			items, err := deps.Exercises.ListExercises(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}

			writeJSON(w, http.StatusOK, items)
		})
		authenticated.Get("/api/v1/planner/equipment", func(w http.ResponseWriter, r *http.Request) {
			items, err := deps.Exercises.ListEquipment(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}

			writeJSON(w, http.StatusOK, items)
		})
		authenticated.Get("/api/v1/planner/templates", func(w http.ResponseWriter, r *http.Request) {
			principal := principalFromContext(r.Context())
			templates, err := deps.Planner.ListTemplates(r.Context(), principal.UserID)
			if err != nil {
				writePlannerError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, templates)
		})
		authenticated.Post("/api/v1/planner/templates", func(w http.ResponseWriter, r *http.Request) {
			var request planner.TemplateInput
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			template, err := deps.Planner.CreateTemplate(r.Context(), principal.UserID, request)
			if err != nil {
				writePlannerError(w, err)
				return
			}

			writeJSON(w, http.StatusCreated, template)
		})
		authenticated.Get("/api/v1/planner/templates/{templateID}", func(w http.ResponseWriter, r *http.Request) {
			templateID, err := parseUUIDParam(r, "templateID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			template, err := deps.Planner.GetTemplate(r.Context(), principal.UserID, templateID)
			if err != nil {
				writePlannerError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, template)
		})
		authenticated.Put("/api/v1/planner/templates/{templateID}", func(w http.ResponseWriter, r *http.Request) {
			templateID, err := parseUUIDParam(r, "templateID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request planner.TemplateInput
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			template, err := deps.Planner.UpdateTemplate(r.Context(), principal.UserID, templateID, request)
			if err != nil {
				writePlannerError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, template)
		})
		authenticated.Get("/api/v1/planner/weeks/{weekStart}", func(w http.ResponseWriter, r *http.Request) {
			principal := principalFromContext(r.Context())
			week, err := deps.Planner.GetWeek(r.Context(), principal.UserID, chi.URLParam(r, "weekStart"))
			if err != nil {
				writePlannerError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, week)
		})
		authenticated.Put("/api/v1/planner/weeks/{weekStart}", func(w http.ResponseWriter, r *http.Request) {
			var request planner.WeekInput
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			week, err := deps.Planner.PutWeek(r.Context(), principal.UserID, chi.URLParam(r, "weekStart"), request)
			if err != nil {
				writePlannerError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, week)
		})
		authenticated.Get("/api/v1/lobby/eligibility", func(w http.ResponseWriter, r *http.Request) {
			principal := principalFromContext(r.Context())
			lobbyEligibility, err := deps.Eligibility.GetLobbyEligibility(r.Context(), principal.UserID)
			if err != nil {
				switch {
				case errors.Is(err, eligibility.ErrNotFound):
					writeError(w, http.StatusNotFound, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			writeJSON(w, http.StatusOK, lobbyEligibility)
		})
		authenticated.Get("/api/v1/lobby/membership", func(w http.ResponseWriter, r *http.Request) {
			principal := principalFromContext(r.Context())
			lobbyMembership, err := deps.Membership.GetLobbyMembership(r.Context(), principal.UserID)
			if err != nil {
				switch {
				case errors.Is(err, membership.ErrNotFound):
					writeError(w, http.StatusNotFound, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			writeJSON(w, http.StatusOK, lobbyMembership)
		})
		authenticated.Get("/api/v1/lobby/match-preview", func(w http.ResponseWriter, r *http.Request) {
			matchPreview, err := deps.MatchPreview.GetLobbyMatchPreview(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}

			slog.Info(
				"lobby match preview read",
				"candidate_count", matchPreview.CandidateCount,
				"match_count", len(matchPreview.Matches),
				"unmatched_count", len(matchPreview.UnmatchedMemberIDs),
				"preview_version", matchPreview.PreviewVersion,
			)
			writeJSON(w, http.StatusOK, matchPreview)
		})
		authenticated.Get("/api/v1/competition/commands/readiness", func(w http.ResponseWriter, r *http.Request) {
			principal := principalFromContext(r.Context())
			readiness := deps.Competition.CompetitionReadiness(competitionActorFromPrincipal(principal, ""))

			writeJSON(w, http.StatusOK, readiness)
		})
		authenticated.Post("/api/v1/competition/commands", func(w http.ResponseWriter, r *http.Request) {
			var command competition.CompetitionCommand
			if err := decodeJSONBody(w, r, &command); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if command.IdempotencyKey == "" {
				command.IdempotencyKey = r.Header.Get("Idempotency-Key")
			}

			principal := principalFromContext(r.Context())
			if !command.DryRun {
				surface, err := trustedSurfaceVerifier.VerifyRequest(r)
				if err != nil {
					outcome := competition.CompetitionCommandOutcome{
						Name:    command.Name,
						Status:  competition.CommandStatusDenied,
						DryRun:  command.DryRun,
						Actor:   competitionCommandActorFromPrincipal(principal, ""),
						Message: "Competition command requires trusted-surface proof.",
						Error:   err.Error(),
					}
					writeJSON(w, http.StatusForbidden, outcome)
					return
				}
				principal = principal.WithTrustedSurface(surface)
			}
			command.Actor = competitionCommandActorFromPrincipal(principal, "")

			outcome, err := deps.Competition.ExecuteCommand(r.Context(), command)
			writeJSON(w, competitionCommandHTTPStatus(outcome, err), outcome)
		})
		authenticated.Get("/api/v1/competition/sessions", withCompetitionAccess(authz.CapabilityCompetitionRead, false, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, _ competition.StaffActor) {
			sessions, err := deps.Competition.ListSessions(r.Context())
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, sessions)
		}))
		authenticated.Get("/api/v1/competition/member-stats", func(w http.ResponseWriter, r *http.Request) {
			principal := principalFromContext(r.Context())
			memberStats, err := deps.Competition.ListMemberStats(r.Context(), principal.UserID)
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, memberStats)
		})
		authenticated.Get("/api/v1/competition/history", func(w http.ResponseWriter, r *http.Request) {
			if deps.CompetitionHistory == nil {
				writeError(w, http.StatusInternalServerError, errors.New("competition history dependency is unavailable"))
				return
			}

			principal := principalFromContext(r.Context())
			history, err := deps.CompetitionHistory.ListMemberHistory(r.Context(), principal.UserID)
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, history)
		})
		authenticated.Post("/api/v1/competition/sessions", withCompetitionAccess(authz.CapabilityCompetitionStructureManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			var request createCompetitionSessionRequest
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			session, err := deps.Competition.CreateSession(r.Context(), actor, competition.CreateSessionInput{
				DisplayName:         request.DisplayName,
				SportKey:            request.SportKey,
				FacilityKey:         request.FacilityKey,
				ZoneKey:             request.ZoneKey,
				ParticipantsPerSide: request.ParticipantsPerSide,
			})
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusCreated, session)
		}))
		authenticated.Get("/api/v1/competition/sessions/{sessionID}", withCompetitionAccess(authz.CapabilityCompetitionRead, false, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, _ competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			session, err := deps.Competition.GetSession(r.Context(), sessionID)
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, session)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/queue/open", withCompetitionAccess(authz.CapabilityCompetitionLiveManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			session, err := deps.Competition.OpenQueue(r.Context(), actor, sessionID)
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, session)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/queue/members", withCompetitionAccess(authz.CapabilityCompetitionLiveManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request addCompetitionQueueMemberRequest
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			session, err := deps.Competition.AddQueueMember(r.Context(), actor, sessionID, competition.QueueMemberInput{
				UserID: request.UserID,
				Tier:   request.Tier,
			})
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, session)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/queue/members/{userID}/remove", withCompetitionAccess(authz.CapabilityCompetitionLiveManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			userID, err := parseUUIDParam(r, "userID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			session, err := deps.Competition.RemoveQueueMember(r.Context(), actor, sessionID, userID)
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, session)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/assignment", withCompetitionAccess(authz.CapabilityCompetitionLiveManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request assignCompetitionSessionRequest
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			session, err := deps.Competition.AssignQueue(r.Context(), actor, sessionID, competition.AssignSessionInput{
				ExpectedQueueVersion: request.ExpectedQueueVersion,
			})
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, session)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/start", withCompetitionAccess(authz.CapabilityCompetitionLiveManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			session, err := deps.Competition.StartSession(r.Context(), actor, sessionID)
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, session)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/archive", withCompetitionAccess(authz.CapabilityCompetitionLiveManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			session, err := deps.Competition.ArchiveSession(r.Context(), actor, sessionID)
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, session)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/teams", withCompetitionAccess(authz.CapabilityCompetitionStructureManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request createCompetitionTeamRequest
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			team, err := deps.Competition.CreateTeam(r.Context(), actor, sessionID, competition.CreateTeamInput{
				SideIndex: request.SideIndex,
			})
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusCreated, team)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/teams/{teamID}/remove", withCompetitionAccess(authz.CapabilityCompetitionStructureManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			teamID, err := parseUUIDParam(r, "teamID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			if err := deps.Competition.RemoveTeam(r.Context(), actor, sessionID, teamID); err != nil {
				writeCompetitionError(w, err)
				return
			}

			w.WriteHeader(http.StatusNoContent)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/teams/{teamID}/members", withCompetitionAccess(authz.CapabilityCompetitionStructureManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			teamID, err := parseUUIDParam(r, "teamID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request addCompetitionRosterMemberRequest
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			team, err := deps.Competition.AddRosterMember(r.Context(), actor, sessionID, teamID, competition.AddRosterMemberInput{
				UserID:    request.UserID,
				SlotIndex: request.SlotIndex,
			})
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, team)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/teams/{teamID}/members/{userID}/remove", withCompetitionAccess(authz.CapabilityCompetitionStructureManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			teamID, err := parseUUIDParam(r, "teamID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			userID, err := parseUUIDParam(r, "userID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			if err := deps.Competition.RemoveRosterMember(r.Context(), actor, sessionID, teamID, userID); err != nil {
				writeCompetitionError(w, err)
				return
			}

			w.WriteHeader(http.StatusNoContent)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/matches", withCompetitionAccess(authz.CapabilityCompetitionStructureManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request createCompetitionMatchRequest
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			match, err := deps.Competition.CreateMatch(r.Context(), actor, sessionID, competition.CreateMatchInput{
				MatchIndex: request.MatchIndex,
				SideSlots:  request.SideSlots,
			})
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusCreated, match)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/matches/{matchID}/archive", withCompetitionAccess(authz.CapabilityCompetitionStructureManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			matchID, err := parseUUIDParam(r, "matchID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			match, err := deps.Competition.ArchiveMatch(r.Context(), actor, sessionID, matchID)
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, match)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/matches/{matchID}/result", withCompetitionAccess(authz.CapabilityCompetitionLiveManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			matchID, err := parseUUIDParam(r, "matchID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request recordCompetitionMatchResultRequest
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if request.ExpectedResultVersion == nil || *request.ExpectedResultVersion < 0 {
				writeError(w, http.StatusBadRequest, competition.ErrMatchResultVersion)
				return
			}

			session, err := deps.Competition.RecordMatchResult(r.Context(), actor, sessionID, matchID, competition.RecordMatchResultInput{
				ExpectedResultVersion: *request.ExpectedResultVersion,
				Sides:                 request.Sides,
			})
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, session)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/matches/{matchID}/result/finalize", withCompetitionAccess(authz.CapabilityCompetitionLiveManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			matchID, err := parseUUIDParam(r, "matchID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			expectedVersion, ok := decodeCompetitionResultExpectedVersion(w, r)
			if !ok {
				return
			}

			session, err := deps.Competition.FinalizeMatchResult(r.Context(), actor, sessionID, matchID, expectedVersion)
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, session)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/matches/{matchID}/result/dispute", withCompetitionAccess(authz.CapabilityCompetitionLiveManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			matchID, err := parseUUIDParam(r, "matchID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			expectedVersion, ok := decodeCompetitionResultExpectedVersion(w, r)
			if !ok {
				return
			}

			session, err := deps.Competition.DisputeMatchResult(r.Context(), actor, sessionID, matchID, expectedVersion)
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, session)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/matches/{matchID}/result/correct", withCompetitionAccess(authz.CapabilityCompetitionLiveManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			matchID, err := parseUUIDParam(r, "matchID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request recordCompetitionMatchResultRequest
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if request.ExpectedResultVersion == nil || *request.ExpectedResultVersion < 0 {
				writeError(w, http.StatusBadRequest, competition.ErrMatchResultVersion)
				return
			}

			session, err := deps.Competition.CorrectMatchResult(r.Context(), actor, sessionID, matchID, competition.RecordMatchResultInput{
				ExpectedResultVersion: *request.ExpectedResultVersion,
				Sides:                 request.Sides,
			})
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, session)
		}))
		authenticated.Post("/api/v1/competition/sessions/{sessionID}/matches/{matchID}/result/void", withCompetitionAccess(authz.CapabilityCompetitionLiveManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor competition.StaffActor) {
			sessionID, err := parseUUIDParam(r, "sessionID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			matchID, err := parseUUIDParam(r, "matchID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			expectedVersion, ok := decodeCompetitionResultExpectedVersion(w, r)
			if !ok {
				return
			}

			session, err := deps.Competition.VoidMatchResult(r.Context(), actor, sessionID, matchID, expectedVersion)
			if err != nil {
				writeCompetitionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, session)
		}))
		authenticated.Get("/api/v1/schedule/blocks", withScheduleAccess(authz.CapabilityScheduleRead, false, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, _ schedule.StaffActor) {
			facilityKey := strings.TrimSpace(r.URL.Query().Get("facility_key"))
			if facilityKey == "" {
				writeError(w, http.StatusBadRequest, errors.New("facility_key is required"))
				return
			}

			blocks, err := deps.Schedule.ListBlocks(r.Context(), facilityKey)
			if err != nil {
				writeScheduleError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, blocks)
		}))
		authenticated.Get("/api/v1/schedule/calendar", withScheduleAccess(authz.CapabilityScheduleRead, false, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, _ schedule.StaffActor) {
			facilityKey := strings.TrimSpace(r.URL.Query().Get("facility_key"))
			if facilityKey == "" {
				writeError(w, http.StatusBadRequest, errors.New("facility_key is required"))
				return
			}

			from, err := parseScheduleWindowBoundary(r.URL.Query().Get("from"), false)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			until, err := parseScheduleWindowBoundary(r.URL.Query().Get("until"), true)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			occurrences, err := deps.Schedule.GetCalendar(r.Context(), facilityKey, schedule.CalendarWindow{From: from, Until: until})
			if err != nil {
				writeScheduleError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, occurrences)
		}))
		authenticated.Get("/api/v1/booking/requests", withBookingAccess(authz.CapabilityBookingRead, false, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, _ booking.StaffActor) {
			if deps.Booking == nil {
				writeError(w, http.StatusServiceUnavailable, errors.New("booking request dependency is unavailable"))
				return
			}

			requests, err := deps.Booking.ListRequests(r.Context(), strings.TrimSpace(r.URL.Query().Get("facility_key")))
			if err != nil {
				writeBookingError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, requests)
		}))
		authenticated.Post("/api/v1/booking/requests", withBookingAccess(authz.CapabilityBookingManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor booking.StaffActor) {
			if deps.Booking == nil {
				writeError(w, http.StatusServiceUnavailable, errors.New("booking request dependency is unavailable"))
				return
			}

			var request booking.RequestInput
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			created, err := deps.Booking.CreateRequestWithIdempotency(r.Context(), actor, r.Header.Get("Idempotency-Key"), request)
			if err != nil {
				writeBookingError(w, err)
				return
			}

			writeJSON(w, http.StatusCreated, created)
		}))
		authenticated.Get("/api/v1/booking/requests/{requestID}", withBookingAccess(authz.CapabilityBookingRead, false, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, _ booking.StaffActor) {
			if deps.Booking == nil {
				writeError(w, http.StatusServiceUnavailable, errors.New("booking request dependency is unavailable"))
				return
			}

			requestID, err := parseUUIDParam(r, "requestID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			request, err := deps.Booking.GetRequest(r.Context(), requestID)
			if err != nil {
				writeBookingError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, request)
		}))
		authenticated.Post("/api/v1/booking/requests/{requestID}/edit", withBookingAccess(authz.CapabilityBookingManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor booking.StaffActor) {
			if deps.Booking == nil {
				writeError(w, http.StatusServiceUnavailable, errors.New("booking request dependency is unavailable"))
				return
			}

			requestID, err := parseUUIDParam(r, "requestID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var input booking.RequestEditInput
			if err := decodeJSONBody(w, r, &input); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if input.ExpectedVersion <= 0 {
				writeError(w, http.StatusBadRequest, booking.ErrExpectedVersionRequired)
				return
			}

			request, err := deps.Booking.UpdateRequest(r.Context(), actor, requestID, input)
			if err != nil {
				writeBookingError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, request)
		}))
		authenticated.Post("/api/v1/booking/requests/{requestID}/rebook", withBookingAccess(authz.CapabilityBookingManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor booking.StaffActor) {
			if deps.Booking == nil {
				writeError(w, http.StatusServiceUnavailable, errors.New("booking request dependency is unavailable"))
				return
			}

			requestID, err := parseUUIDParam(r, "requestID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var input booking.RequestEditInput
			if err := decodeJSONBody(w, r, &input); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if input.ExpectedVersion <= 0 {
				writeError(w, http.StatusBadRequest, booking.ErrExpectedVersionRequired)
				return
			}

			request, err := deps.Booking.RebookRequestWithIdempotency(r.Context(), actor, requestID, r.Header.Get("Idempotency-Key"), input)
			if err != nil {
				writeBookingError(w, err)
				return
			}

			writeJSON(w, http.StatusCreated, request)
		}))
		authenticated.Post("/api/v1/booking/requests/{requestID}/public-message", withBookingAccess(authz.CapabilityBookingManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor booking.StaffActor) {
			if deps.Booking == nil {
				writeError(w, http.StatusServiceUnavailable, errors.New("booking request dependency is unavailable"))
				return
			}

			requestID, err := parseUUIDParam(r, "requestID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var input booking.PublicMessageInput
			if err := decodeJSONBody(w, r, &input); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if input.ExpectedVersion <= 0 {
				writeError(w, http.StatusBadRequest, booking.ErrExpectedVersionRequired)
				return
			}

			request, err := deps.Booking.UpdatePublicMessage(r.Context(), actor, requestID, input)
			if err != nil {
				writeBookingError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, request)
		}))
		authenticated.Post("/api/v1/booking/requests/{requestID}/review", withBookingAccess(authz.CapabilityBookingManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor booking.StaffActor) {
			handleBookingTransition(w, r, deps.Booking, actor, func(ctx context.Context, manager BookingManager, requestID uuid.UUID, input booking.TransitionInput) (booking.Request, error) {
				return manager.StartReview(ctx, actor, requestID, input)
			})
		}))
		authenticated.Post("/api/v1/booking/requests/{requestID}/needs-changes", withBookingAccess(authz.CapabilityBookingManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor booking.StaffActor) {
			handleBookingTransition(w, r, deps.Booking, actor, func(ctx context.Context, manager BookingManager, requestID uuid.UUID, input booking.TransitionInput) (booking.Request, error) {
				return manager.NeedsChanges(ctx, actor, requestID, input)
			})
		}))
		authenticated.Post("/api/v1/booking/requests/{requestID}/reject", withBookingAccess(authz.CapabilityBookingManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor booking.StaffActor) {
			handleBookingTransition(w, r, deps.Booking, actor, func(ctx context.Context, manager BookingManager, requestID uuid.UUID, input booking.TransitionInput) (booking.Request, error) {
				return manager.Reject(ctx, actor, requestID, input)
			})
		}))
		authenticated.Post("/api/v1/booking/requests/{requestID}/cancel", withBookingAccess(authz.CapabilityBookingManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor booking.StaffActor) {
			handleBookingTransition(w, r, deps.Booking, actor, func(ctx context.Context, manager BookingManager, requestID uuid.UUID, input booking.TransitionInput) (booking.Request, error) {
				return manager.Cancel(ctx, actor, requestID, input)
			})
		}))
		authenticated.Post("/api/v1/booking/requests/{requestID}/approve", withBookingAccess(authz.CapabilityBookingManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor booking.StaffActor) {
			handleBookingTransition(w, r, deps.Booking, actor, func(ctx context.Context, manager BookingManager, requestID uuid.UUID, input booking.TransitionInput) (booking.Request, error) {
				return manager.Approve(ctx, actor, requestID, input)
			})
		}))
		authenticated.Get("/api/v1/ops/facilities/{facilityKey}/overview", withOpsAccess(authz.CapabilityOpsRead, func(w http.ResponseWriter, r *http.Request) {
			if deps.Ops == nil {
				writeError(w, http.StatusServiceUnavailable, errors.New("ops overview dependency is unavailable"))
				return
			}

			facilityKey := strings.TrimSpace(chi.URLParam(r, "facilityKey"))
			from, err := parseScheduleWindowBoundary(r.URL.Query().Get("from"), false)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			until, err := parseScheduleWindowBoundary(r.URL.Query().Get("until"), true)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			bucketMinutes, err := parseOptionalPositiveQueryInt(r.URL.Query().Get("bucket_minutes"), "bucket_minutes")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			overview, err := deps.Ops.GetFacilityOverview(r.Context(), ops.FacilityOverviewInput{
				FacilityKey:   facilityKey,
				From:          from,
				Until:         until,
				BucketMinutes: bucketMinutes,
			})
			if err != nil {
				writeOpsError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, overview)
		}))
		authenticated.Post("/api/v1/schedule/blocks", withScheduleAccess(authz.CapabilityScheduleManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor schedule.StaffActor) {
			var request schedule.BlockInput
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			block, err := deps.Schedule.CreateBlock(r.Context(), actor, request)
			if err != nil {
				writeScheduleError(w, err)
				return
			}

			writeJSON(w, http.StatusCreated, block)
		}))
		authenticated.Post("/api/v1/schedule/blocks/{blockID}/exceptions", withScheduleAccess(authz.CapabilityScheduleManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor schedule.StaffActor) {
			blockID, err := parseUUIDParam(r, "blockID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request struct {
				ExpectedVersion int    `json:"expected_version"`
				ExceptionDate   string `json:"exception_date"`
			}
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if request.ExpectedVersion <= 0 {
				writeError(w, http.StatusBadRequest, errors.New("expected_version must be positive"))
				return
			}

			block, err := deps.Schedule.AddException(r.Context(), actor, blockID, request.ExpectedVersion, schedule.BlockExceptionInput{
				ExceptionDate: request.ExceptionDate,
			})
			if err != nil {
				writeScheduleError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, block)
		}))
		authenticated.Post("/api/v1/schedule/blocks/{blockID}/cancel", withScheduleAccess(authz.CapabilityScheduleManage, true, trustedSurfaceVerifier, func(w http.ResponseWriter, r *http.Request, actor schedule.StaffActor) {
			blockID, err := parseUUIDParam(r, "blockID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request struct {
				ExpectedVersion int `json:"expected_version"`
			}
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if request.ExpectedVersion <= 0 {
				writeError(w, http.StatusBadRequest, errors.New("expected_version must be positive"))
				return
			}

			block, err := deps.Schedule.CancelBlock(r.Context(), actor, blockID, request.ExpectedVersion)
			if err != nil {
				writeScheduleError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, block)
		}))
		authenticated.Post("/api/v1/lobby/membership/join", func(w http.ResponseWriter, r *http.Request) {
			principal := principalFromContext(r.Context())
			lobbyMembership, err := deps.Membership.JoinLobbyMembership(r.Context(), principal.UserID)
			if err != nil {
				switch {
				case errors.Is(err, membership.ErrNotFound):
					writeError(w, http.StatusNotFound, err)
				case errors.Is(err, membership.ErrAlreadyJoined), errors.Is(err, membership.ErrIneligible):
					writeError(w, http.StatusConflict, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			logLobbyMembershipTransition("lobby membership joined", principal.UserID, lobbyMembership)
			writeJSON(w, http.StatusOK, lobbyMembership)
		})
		authenticated.Post("/api/v1/lobby/membership/leave", func(w http.ResponseWriter, r *http.Request) {
			principal := principalFromContext(r.Context())
			lobbyMembership, err := deps.Membership.LeaveLobbyMembership(r.Context(), principal.UserID)
			if err != nil {
				switch {
				case errors.Is(err, membership.ErrNotFound):
					writeError(w, http.StatusNotFound, err)
				case errors.Is(err, membership.ErrNotJoined):
					writeError(w, http.StatusConflict, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			logLobbyMembershipTransition("lobby membership left", principal.UserID, lobbyMembership)
			writeJSON(w, http.StatusOK, lobbyMembership)
		})
		authenticated.Get("/api/v1/recommendations/workout", func(w http.ResponseWriter, r *http.Request) {
			principal := principalFromContext(r.Context())
			workoutRecommendation, err := deps.Recommendations.GetWorkoutRecommendation(r.Context(), principal.UserID)
			if err != nil {
				switch {
				case errors.Is(err, recommendations.ErrInvalidFinishedWorkoutState):
					writeError(w, http.StatusInternalServerError, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			writeJSON(w, http.StatusOK, workoutRecommendation)
		})
		authenticated.Get("/api/v1/recommendations/coaching", func(w http.ResponseWriter, r *http.Request) {
			if deps.Coaching == nil {
				writeError(w, http.StatusInternalServerError, errors.New("coaching dependency is unavailable"))
				return
			}

			weekStart := strings.TrimSpace(r.URL.Query().Get("week_start"))
			if weekStart == "" {
				writeError(w, http.StatusBadRequest, planner.ErrWeekStartInvalid)
				return
			}

			principal := principalFromContext(r.Context())
			coachingRecommendation, err := deps.Coaching.GetCoachingRecommendation(r.Context(), principal.UserID, weekStart)
			if err != nil {
				writeCoachingError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, coachingRecommendation)
		})
		authenticated.Get("/api/v1/helpers/coaching", func(w http.ResponseWriter, r *http.Request) {
			if deps.Coaching == nil {
				writeError(w, http.StatusInternalServerError, errors.New("coaching dependency is unavailable"))
				return
			}

			weekStart := strings.TrimSpace(r.URL.Query().Get("week_start"))
			if weekStart == "" {
				writeError(w, http.StatusBadRequest, planner.ErrWeekStartInvalid)
				return
			}

			principal := principalFromContext(r.Context())
			helperRead, err := deps.Coaching.GetHelperRead(r.Context(), principal.UserID, weekStart)
			if err != nil {
				writeCoachingError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, helperRead)
		})
		authenticated.Get("/api/v1/helpers/coaching/why", func(w http.ResponseWriter, r *http.Request) {
			if deps.Coaching == nil {
				writeError(w, http.StatusInternalServerError, errors.New("coaching dependency is unavailable"))
				return
			}

			weekStart := strings.TrimSpace(r.URL.Query().Get("week_start"))
			if weekStart == "" {
				writeError(w, http.StatusBadRequest, planner.ErrWeekStartInvalid)
				return
			}
			topic := strings.TrimSpace(r.URL.Query().Get("topic"))
			if topic == "" {
				writeError(w, http.StatusBadRequest, helper.ErrUnsupportedWhyTopic)
				return
			}

			principal := principalFromContext(r.Context())
			why, err := deps.Coaching.AskWhy(r.Context(), principal.UserID, weekStart, topic)
			if err != nil {
				writeCoachingError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, why)
		})
		authenticated.Get("/api/v1/helpers/coaching/variation", func(w http.ResponseWriter, r *http.Request) {
			if deps.Coaching == nil {
				writeError(w, http.StatusInternalServerError, errors.New("coaching dependency is unavailable"))
				return
			}

			weekStart := strings.TrimSpace(r.URL.Query().Get("week_start"))
			if weekStart == "" {
				writeError(w, http.StatusBadRequest, planner.ErrWeekStartInvalid)
				return
			}
			variation := strings.TrimSpace(r.URL.Query().Get("variation"))
			if variation == "" {
				writeError(w, http.StatusBadRequest, helper.ErrUnsupportedVariation)
				return
			}

			principal := principalFromContext(r.Context())
			preview, err := deps.Coaching.PreviewVariation(r.Context(), principal.UserID, weekStart, variation)
			if err != nil {
				writeCoachingError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, preview)
		})
		authenticated.Get("/api/v1/recommendations/nutrition", func(w http.ResponseWriter, r *http.Request) {
			if deps.Nutrition == nil {
				writeError(w, http.StatusInternalServerError, errors.New("nutrition dependency is unavailable"))
				return
			}

			principal := principalFromContext(r.Context())
			recommendation, err := deps.Nutrition.GetRecommendation(r.Context(), principal.UserID)
			if err != nil {
				writeNutritionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, recommendation)
		})
		authenticated.Get("/api/v1/helpers/nutrition", func(w http.ResponseWriter, r *http.Request) {
			if deps.Nutrition == nil {
				writeError(w, http.StatusInternalServerError, errors.New("nutrition dependency is unavailable"))
				return
			}

			principal := principalFromContext(r.Context())
			helperRead, err := deps.Nutrition.GetHelperRead(r.Context(), principal.UserID)
			if err != nil {
				writeNutritionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, helperRead)
		})
		authenticated.Get("/api/v1/helpers/nutrition/why", func(w http.ResponseWriter, r *http.Request) {
			if deps.Nutrition == nil {
				writeError(w, http.StatusInternalServerError, errors.New("nutrition dependency is unavailable"))
				return
			}

			topic := strings.TrimSpace(r.URL.Query().Get("topic"))
			if topic == "" {
				writeError(w, http.StatusBadRequest, helper.ErrUnsupportedWhyTopic)
				return
			}

			principal := principalFromContext(r.Context())
			why, err := deps.Nutrition.AskWhy(r.Context(), principal.UserID, topic)
			if err != nil {
				writeNutritionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, why)
		})
		authenticated.Get("/api/v1/helpers/nutrition/variation", func(w http.ResponseWriter, r *http.Request) {
			if deps.Nutrition == nil {
				writeError(w, http.StatusInternalServerError, errors.New("nutrition dependency is unavailable"))
				return
			}

			variation := strings.TrimSpace(r.URL.Query().Get("variation"))
			if variation == "" {
				writeError(w, http.StatusBadRequest, helper.ErrUnsupportedVariation)
				return
			}

			principal := principalFromContext(r.Context())
			preview, err := deps.Nutrition.PreviewVariation(r.Context(), principal.UserID, variation)
			if err != nil {
				writeNutritionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, preview)
		})
		authenticated.Get("/api/v1/nutrition/meal-logs", func(w http.ResponseWriter, r *http.Request) {
			if deps.Nutrition == nil {
				writeError(w, http.StatusInternalServerError, errors.New("nutrition dependency is unavailable"))
				return
			}

			principal := principalFromContext(r.Context())
			mealLogs, err := deps.Nutrition.ListMealLogs(r.Context(), principal.UserID)
			if err != nil {
				writeNutritionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, mealLogs)
		})
		authenticated.Post("/api/v1/nutrition/meal-logs", func(w http.ResponseWriter, r *http.Request) {
			if deps.Nutrition == nil {
				writeError(w, http.StatusInternalServerError, errors.New("nutrition dependency is unavailable"))
				return
			}

			var request nutrition.MealLogInput
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			mealLog, err := deps.Nutrition.CreateMealLog(r.Context(), principal.UserID, request)
			if err != nil {
				writeNutritionError(w, err)
				return
			}

			writeJSON(w, http.StatusCreated, mealLog)
		})
		authenticated.Put("/api/v1/nutrition/meal-logs/{mealLogID}", func(w http.ResponseWriter, r *http.Request) {
			if deps.Nutrition == nil {
				writeError(w, http.StatusInternalServerError, errors.New("nutrition dependency is unavailable"))
				return
			}

			mealLogID, err := parseUUIDParam(r, "mealLogID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request nutrition.MealLogInput
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			mealLog, err := deps.Nutrition.UpdateMealLog(r.Context(), principal.UserID, mealLogID, request)
			if err != nil {
				writeNutritionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, mealLog)
		})
		authenticated.Get("/api/v1/nutrition/meal-templates", func(w http.ResponseWriter, r *http.Request) {
			if deps.Nutrition == nil {
				writeError(w, http.StatusInternalServerError, errors.New("nutrition dependency is unavailable"))
				return
			}

			principal := principalFromContext(r.Context())
			mealTemplates, err := deps.Nutrition.ListMealTemplates(r.Context(), principal.UserID)
			if err != nil {
				writeNutritionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, mealTemplates)
		})
		authenticated.Post("/api/v1/nutrition/meal-templates", func(w http.ResponseWriter, r *http.Request) {
			if deps.Nutrition == nil {
				writeError(w, http.StatusInternalServerError, errors.New("nutrition dependency is unavailable"))
				return
			}

			var request nutrition.MealTemplateInput
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			mealTemplate, err := deps.Nutrition.CreateMealTemplate(r.Context(), principal.UserID, request)
			if err != nil {
				writeNutritionError(w, err)
				return
			}

			writeJSON(w, http.StatusCreated, mealTemplate)
		})
		authenticated.Put("/api/v1/nutrition/meal-templates/{templateID}", func(w http.ResponseWriter, r *http.Request) {
			if deps.Nutrition == nil {
				writeError(w, http.StatusInternalServerError, errors.New("nutrition dependency is unavailable"))
				return
			}

			templateID, err := parseUUIDParam(r, "templateID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request nutrition.MealTemplateInput
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			mealTemplate, err := deps.Nutrition.UpdateMealTemplate(r.Context(), principal.UserID, templateID, request)
			if err != nil {
				writeNutritionError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, mealTemplate)
		})
		authenticated.Post("/api/v1/workouts", func(w http.ResponseWriter, r *http.Request) {
			var request createWorkoutRequest
			if err := decodeJSONBodyAllowEmpty(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			workout, err := deps.Workouts.CreateWorkout(r.Context(), principal.UserID, workouts.CreateInput{
				Notes: request.Notes,
			})
			if err != nil {
				switch {
				case errors.Is(err, workouts.ErrWorkoutAlreadyInProgress):
					writeError(w, http.StatusConflict, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			logWorkoutLifecycle("workout created", principal.UserID, workout)
			writeJSON(w, http.StatusCreated, workout)
		})
		authenticated.Get("/api/v1/workouts", func(w http.ResponseWriter, r *http.Request) {
			principal := principalFromContext(r.Context())
			memberWorkouts, err := deps.Workouts.ListWorkouts(r.Context(), principal.UserID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}

			writeJSON(w, http.StatusOK, memberWorkouts)
		})
		authenticated.Get("/api/v1/workouts/{workoutID}", func(w http.ResponseWriter, r *http.Request) {
			workoutID, err := parseUUIDParam(r, "workoutID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			workout, err := deps.Workouts.GetWorkout(r.Context(), principal.UserID, workoutID)
			if err != nil {
				switch {
				case errors.Is(err, workouts.ErrNotFound):
					writeError(w, http.StatusNotFound, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			writeJSON(w, http.StatusOK, workout)
		})
		authenticated.Put("/api/v1/workouts/{workoutID}", func(w http.ResponseWriter, r *http.Request) {
			workoutID, err := parseUUIDParam(r, "workoutID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request updateWorkoutRequest
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			workout, err := deps.Workouts.UpdateWorkout(r.Context(), principal.UserID, workoutID, workouts.UpdateInput{
				Notes:     request.Notes,
				Exercises: request.Exercises,
			})
			if err != nil {
				switch {
				case errors.Is(err, workouts.ErrNotFound):
					writeError(w, http.StatusNotFound, err)
				case errors.Is(err, workouts.ErrWorkoutFinished):
					writeError(w, http.StatusConflict, err)
				case errors.Is(err, workouts.ErrExercisePayloadRequired),
					errors.Is(err, workouts.ErrExerciseCountInvalid),
					errors.Is(err, workouts.ErrExerciseNameRequired),
					errors.Is(err, workouts.ErrExerciseSetsInvalid),
					errors.Is(err, workouts.ErrExerciseRepsInvalid),
					errors.Is(err, workouts.ErrExerciseWeightInvalid),
					errors.Is(err, workouts.ErrExerciseRPEInvalid):
					writeError(w, http.StatusBadRequest, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			logWorkoutLifecycle("workout updated", principal.UserID, workout)
			writeJSON(w, http.StatusOK, workout)
		})
		authenticated.Post("/api/v1/workouts/{workoutID}/finish", func(w http.ResponseWriter, r *http.Request) {
			workoutID, err := parseUUIDParam(r, "workoutID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			workout, err := deps.Workouts.FinishWorkout(r.Context(), principal.UserID, workoutID)
			if err != nil {
				switch {
				case errors.Is(err, workouts.ErrNotFound):
					writeError(w, http.StatusNotFound, err)
				case errors.Is(err, workouts.ErrWorkoutFinished):
					writeError(w, http.StatusConflict, err)
				case errors.Is(err, workouts.ErrCannotFinishEmptyWorkout):
					writeError(w, http.StatusBadRequest, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			logWorkoutLifecycle("workout finished", principal.UserID, workout)
			writeJSON(w, http.StatusOK, workout)
		})
		authenticated.Put("/api/v1/workouts/{workoutID}/effort-feedback", func(w http.ResponseWriter, r *http.Request) {
			if deps.Coaching == nil {
				writeError(w, http.StatusInternalServerError, errors.New("coaching dependency is unavailable"))
				return
			}

			workoutID, err := parseUUIDParam(r, "workoutID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request putEffortFeedbackRequest
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			feedback, err := deps.Coaching.PutEffortFeedback(r.Context(), principal.UserID, workoutID, coaching.EffortFeedbackInput{
				EffortLevel: request.EffortLevel,
			})
			if err != nil {
				writeCoachingError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, feedback)
		})
		authenticated.Put("/api/v1/workouts/{workoutID}/recovery-feedback", func(w http.ResponseWriter, r *http.Request) {
			if deps.Coaching == nil {
				writeError(w, http.StatusInternalServerError, errors.New("coaching dependency is unavailable"))
				return
			}

			workoutID, err := parseUUIDParam(r, "workoutID")
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			var request putRecoveryFeedbackRequest
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			feedback, err := deps.Coaching.PutRecoveryFeedback(r.Context(), principal.UserID, workoutID, coaching.RecoveryFeedbackInput{
				RecoveryLevel: request.RecoveryLevel,
			})
			if err != nil {
				writeCoachingError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, feedback)
		})
		authenticated.Patch("/api/v1/profile", func(w http.ResponseWriter, r *http.Request) {
			var request profile.UpdateInput
			if err := decodeJSONBody(w, r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			memberProfile, err := deps.Profile.UpdateProfile(r.Context(), principal.UserID, request)
			if err != nil {
				switch {
				case errors.Is(err, profile.ErrEmptyPatch),
					errors.Is(err, profile.ErrInvalidVisibilityMode),
					errors.Is(err, profile.ErrInvalidAvailabilityMode),
					errors.Is(err, profile.ErrInvalidGoalKey),
					errors.Is(err, profile.ErrInvalidDaysPerWeek),
					errors.Is(err, profile.ErrInvalidSessionMinutes),
					errors.Is(err, profile.ErrInvalidExperienceLevel),
					errors.Is(err, profile.ErrInvalidEquipmentKeys),
					errors.Is(err, profile.ErrInvalidDietaryRestrictions),
					errors.Is(err, profile.ErrInvalidCuisinePreferences),
					errors.Is(err, profile.ErrInvalidBudgetPreference),
					errors.Is(err, profile.ErrInvalidCookingCapability):
					writeError(w, http.StatusBadRequest, err)
				case errors.Is(err, profile.ErrNotFound):
					writeError(w, http.StatusNotFound, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			writeJSON(w, http.StatusOK, memberProfile)
		})
		authenticated.Post("/api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(deps.Auth.SessionCookieName())
			if err != nil {
				writeError(w, http.StatusUnauthorized, err)
				return
			}
			if err := deps.Auth.LogoutSession(r.Context(), cookie.Value); err != nil {
				switch {
				case errors.Is(err, auth.ErrInvalidSessionCookie), errors.Is(err, auth.ErrSessionNotFound), errors.Is(err, auth.ErrSessionExpired), errors.Is(err, auth.ErrSessionRevoked):
					writeError(w, http.StatusUnauthorized, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			http.SetCookie(w, deps.Auth.ExpiredSessionCookie())
			w.WriteHeader(http.StatusNoContent)
		})
	})

	return router
}

func sessionMiddleware(authenticator Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(authenticator.SessionCookieName())
			if err != nil {
				writeError(w, http.StatusUnauthorized, errors.New("missing session cookie"))
				return
			}

			principal, err := authenticator.AuthenticateSession(r.Context(), cookie.Value)
			if err != nil {
				switch {
				case errors.Is(err, auth.ErrInvalidSessionCookie), errors.Is(err, auth.ErrSessionNotFound), errors.Is(err, auth.ErrSessionExpired), errors.Is(err, auth.ErrSessionRevoked):
					writeError(w, http.StatusUnauthorized, err)
				default:
					writeError(w, http.StatusInternalServerError, err)
				}
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalContextKey, principal)))
		})
	}
}

func principalFromContext(ctx context.Context) auth.Principal {
	principal, ok := ctx.Value(principalContextKey).(auth.Principal)
	if !ok {
		return auth.Principal{}
	}

	return principal
}

func withCompetitionAccess(required authz.Capability, requireTrustedSurface bool, verifier *authz.TrustedSurfaceVerifier, next func(http.ResponseWriter, *http.Request, competition.StaffActor)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal := principalFromContext(r.Context())
		if !authz.HasCapability(principal.Capabilities, required) {
			writeError(w, http.StatusForbidden, authz.ErrCapabilityDenied)
			return
		}

		if requireTrustedSurface {
			surface, err := verifier.VerifyRequest(r)
			if err != nil {
				writeError(w, http.StatusForbidden, err)
				return
			}

			principal = principal.WithTrustedSurface(surface)
			r = r.WithContext(context.WithValue(r.Context(), principalContextKey, principal))
		}

		next(w, r, competitionActorFromPrincipal(principal, required))
	}
}

func withScheduleAccess(required authz.Capability, requireTrustedSurface bool, verifier *authz.TrustedSurfaceVerifier, next func(http.ResponseWriter, *http.Request, schedule.StaffActor)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal := principalFromContext(r.Context())
		if !authz.HasCapability(principal.Capabilities, required) {
			writeError(w, http.StatusForbidden, authz.ErrCapabilityDenied)
			return
		}
		w.Header().Set("X-Apollo-Schedule-Can-Manage", strconv.FormatBool(authz.HasCapability(principal.Capabilities, authz.CapabilityScheduleManage)))

		if requireTrustedSurface {
			surface, err := verifier.VerifyRequest(r)
			if err != nil {
				writeError(w, http.StatusForbidden, err)
				return
			}
			principal = principal.WithTrustedSurface(surface)
		}

		next(w, r, scheduleActorFromPrincipal(principal, required))
	}
}

func withBookingAccess(required authz.Capability, requireTrustedSurface bool, verifier *authz.TrustedSurfaceVerifier, next func(http.ResponseWriter, *http.Request, booking.StaffActor)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal := principalFromContext(r.Context())
		if !authz.HasCapability(principal.Capabilities, required) {
			writeError(w, http.StatusForbidden, authz.ErrCapabilityDenied)
			return
		}
		w.Header().Set("X-Apollo-Booking-Can-Manage", strconv.FormatBool(authz.HasCapability(principal.Capabilities, authz.CapabilityBookingManage)))

		if requireTrustedSurface {
			surface, err := verifier.VerifyRequest(r)
			if err != nil {
				writeError(w, http.StatusForbidden, err)
				return
			}
			principal = principal.WithTrustedSurface(surface)
		}

		next(w, r, bookingActorFromPrincipal(principal, required))
	}
}

func withOpsAccess(required authz.Capability, next func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal := principalFromContext(r.Context())
		if !authz.HasCapability(principal.Capabilities, required) {
			writeError(w, http.StatusForbidden, authz.ErrCapabilityDenied)
			return
		}

		next(w, r)
	}
}

func competitionActorFromPrincipal(principal auth.Principal, capability authz.Capability) competition.StaffActor {
	actor := competition.StaffActor{
		UserID:     principal.UserID,
		Role:       principal.Role,
		SessionID:  principal.SessionID,
		Capability: capability,
	}
	if principal.TrustedSurface != nil {
		actor.TrustedSurfaceKey = principal.TrustedSurface.Key
		actor.TrustedSurfaceLabel = principal.TrustedSurface.Label
	}

	return actor
}

func competitionCommandActorFromPrincipal(principal auth.Principal, capability authz.Capability) competition.CompetitionCommandActor {
	actor := competition.CompetitionCommandActor{
		UserID:     principal.UserID,
		Role:       principal.Role,
		SessionID:  principal.SessionID,
		Capability: capability,
	}
	if principal.TrustedSurface != nil {
		actor.TrustedSurfaceKey = principal.TrustedSurface.Key
		actor.TrustedSurfaceLabel = principal.TrustedSurface.Label
	}

	return actor
}

func scheduleActorFromPrincipal(principal auth.Principal, capability authz.Capability) schedule.StaffActor {
	actor := schedule.StaffActor{
		UserID:     principal.UserID,
		Role:       principal.Role,
		SessionID:  principal.SessionID,
		Capability: capability,
	}
	if principal.TrustedSurface != nil {
		actor.TrustedSurfaceKey = principal.TrustedSurface.Key
		actor.TrustedSurfaceLabel = principal.TrustedSurface.Label
	}

	return actor
}

func bookingActorFromPrincipal(principal auth.Principal, capability authz.Capability) booking.StaffActor {
	actor := booking.StaffActor{
		UserID:     principal.UserID,
		Role:       principal.Role,
		SessionID:  principal.SessionID,
		Capability: capability,
	}
	if principal.TrustedSurface != nil {
		actor.TrustedSurfaceKey = principal.TrustedSurface.Key
		actor.TrustedSurfaceLabel = principal.TrustedSurface.Label
	}

	return actor
}

func logWorkoutLifecycle(message string, userID uuid.UUID, workout workouts.Workout) {
	attrs := []any{
		"user_id", userID,
		"workout_id", workout.ID,
		"status", workout.Status,
		"exercise_count", len(workout.Exercises),
	}
	if workout.FinishedAt != nil {
		attrs = append(attrs, "finished_at", workout.FinishedAt.UTC())
	}

	slog.Info(message, attrs...)
}

func logLobbyMembershipTransition(message string, userID uuid.UUID, lobbyMembership membership.LobbyMembership) {
	attrs := []any{
		"user_id", userID,
		"status", lobbyMembership.Status,
	}
	if lobbyMembership.JoinedAt != nil {
		attrs = append(attrs, "joined_at", lobbyMembership.JoinedAt.UTC())
	}
	if lobbyMembership.LeftAt != nil {
		attrs = append(attrs, "left_at", lobbyMembership.LeftAt.UTC())
	}

	slog.Info(message, attrs...)
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, target any) error {
	return decodeJSONBodyWithLimit(w, r, target, maxJSONBodyBytes)
}

func decodeJSONBodyWithLimit(w http.ResponseWriter, r *http.Request, target any, limit int64) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return normalizeDecodeError(err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}

func decodeJSONBodyAllowEmpty(w http.ResponseWriter, r *http.Request, target any) error {
	if r.Body == nil {
		return nil
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return normalizeDecodeError(err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}

func decodeCompetitionResultExpectedVersion(w http.ResponseWriter, r *http.Request) (int, bool) {
	var request competitionMatchResultTransitionRequest
	if err := decodeJSONBody(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return 0, false
	}
	if request.ExpectedResultVersion == nil || *request.ExpectedResultVersion < 0 {
		writeError(w, http.StatusBadRequest, competition.ErrMatchResultVersion)
		return 0, false
	}
	return *request.ExpectedResultVersion, true
}

func normalizeDecodeError(err error) error {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return errors.New("request body is too large")
	}

	return err
}

func parseUUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	value := chi.URLParam(r, name)
	if value == "" {
		return uuid.Nil, errors.New("missing " + name)
	}

	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, errors.New("invalid " + name)
	}

	return id, nil
}

type bookingTransitionFunc func(context.Context, BookingManager, uuid.UUID, booking.TransitionInput) (booking.Request, error)

func handleBookingTransition(w http.ResponseWriter, r *http.Request, manager BookingManager, _ booking.StaffActor, transition bookingTransitionFunc) {
	if manager == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("booking request dependency is unavailable"))
		return
	}

	requestID, err := parseUUIDParam(r, "requestID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var input booking.TransitionInput
	if err := decodeJSONBody(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if input.ExpectedVersion <= 0 {
		writeError(w, http.StatusBadRequest, booking.ErrExpectedVersionRequired)
		return
	}

	request, err := transition(r.Context(), manager, requestID, input)
	if err != nil {
		writeBookingError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, request)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, errorResponse{Error: err.Error()})
}

func competitionCommandHTTPStatus(outcome competition.CompetitionCommandOutcome, err error) int {
	if err == nil {
		return http.StatusOK
	}

	switch {
	case errors.Is(err, authz.ErrCapabilityDenied),
		errors.Is(err, authz.ErrTrustedSurfaceMissing),
		errors.Is(err, authz.ErrTrustedSurfaceKey),
		errors.Is(err, authz.ErrTrustedSurfaceInvalid),
		errors.Is(err, competition.ErrCommandActorRequired),
		errors.Is(err, competition.ErrCommandActorSession),
		errors.Is(err, competition.ErrCommandTrustedSurface):
		return http.StatusForbidden
	case errors.Is(err, competition.ErrSessionNotFound),
		errors.Is(err, competition.ErrTeamNotFound),
		errors.Is(err, competition.ErrMatchNotFound),
		errors.Is(err, competition.ErrQueueIntentNotFound),
		errors.Is(err, competition.ErrUserNotFound),
		errors.Is(err, competition.ErrSportNotFound):
		return http.StatusNotFound
	case errors.Is(err, competition.ErrCommandNameRequired),
		errors.Is(err, competition.ErrCommandUnsupported),
		errors.Is(err, competition.ErrCommandSessionIDRequired),
		errors.Is(err, competition.ErrCommandTeamIDRequired),
		errors.Is(err, competition.ErrCommandMatchIDRequired),
		errors.Is(err, competition.ErrCommandUserIDRequired),
		errors.Is(err, competition.ErrCommandExpectedVersion),
		errors.Is(err, competition.ErrCommandCreateSessionInput),
		errors.Is(err, competition.ErrCommandCreateTeamInput),
		errors.Is(err, competition.ErrCommandRosterInput),
		errors.Is(err, competition.ErrCommandQueueMemberInput),
		errors.Is(err, competition.ErrCommandCreateMatchInput),
		errors.Is(err, competition.ErrCommandMatchResultInput),
		errors.Is(err, competition.ErrSessionNameRequired),
		errors.Is(err, competition.ErrParticipantsPerSide),
		errors.Is(err, competition.ErrFacilityUnsupported),
		errors.Is(err, competition.ErrZoneUnsupported),
		errors.Is(err, competition.ErrTeamSideIndexInvalid),
		errors.Is(err, competition.ErrRosterSlotIndexInvalid),
		errors.Is(err, competition.ErrRosterSlotOutOfRange),
		errors.Is(err, competition.ErrQueueIntentTier),
		errors.Is(err, competition.ErrQueueVersionRequired),
		errors.Is(err, competition.ErrMatchIndexInvalid),
		errors.Is(err, competition.ErrMatchSideCountMismatch),
		errors.Is(err, competition.ErrMatchSideIndexInvalid),
		errors.Is(err, competition.ErrMatchResultVersion),
		errors.Is(err, competition.ErrMatchResultSideCount),
		errors.Is(err, competition.ErrMatchResultSideIndex),
		errors.Is(err, competition.ErrMatchResultOutcome),
		errors.Is(err, competition.ErrMatchResultShape):
		return http.StatusBadRequest
	case errors.Is(err, competition.ErrCommandApplyUnsupported),
		errors.Is(err, competition.ErrSessionArchived),
		errors.Is(err, competition.ErrQueueClosed),
		errors.Is(err, competition.ErrQueueMemberAlreadyJoined),
		errors.Is(err, competition.ErrQueueMemberNotFound),
		errors.Is(err, competition.ErrQueueMemberNotJoined),
		errors.Is(err, competition.ErrQueueMemberIneligible),
		errors.Is(err, competition.ErrQueueCapacityReached),
		errors.Is(err, competition.ErrQueueStateStale),
		errors.Is(err, competition.ErrQueueNotReady),
		errors.Is(err, competition.ErrQueueNotEmpty),
		errors.Is(err, competition.ErrExecutionAlreadySeeded),
		errors.Is(err, competition.ErrInvalidSessionTransition),
		errors.Is(err, competition.ErrDuplicateSession),
		errors.Is(err, competition.ErrDuplicateTeam),
		errors.Is(err, competition.ErrRosterConflict),
		errors.Is(err, competition.ErrDuplicateRosterSlot),
		errors.Is(err, competition.ErrTeamReferencedByMatch),
		errors.Is(err, competition.ErrTeamSizeMismatch),
		errors.Is(err, competition.ErrDuplicateMatch),
		errors.Is(err, competition.ErrDuplicateMatchSideIndex),
		errors.Is(err, competition.ErrDuplicateMatchTeam),
		errors.Is(err, competition.ErrSessionHasDraftMatches),
		errors.Is(err, competition.ErrMatchArchived),
		errors.Is(err, competition.ErrMatchNotInProgress),
		errors.Is(err, competition.ErrMatchResultRecorded),
		errors.Is(err, competition.ErrMatchResultNotFound),
		errors.Is(err, competition.ErrMatchResultStateStale),
		errors.Is(err, competition.ErrMatchResultNotRecorded),
		errors.Is(err, competition.ErrMatchResultNotCanonical),
		errors.Is(err, competition.ErrMatchResultNotFinal),
		errors.Is(err, competition.ErrMatchResultVoided),
		errors.Is(err, competition.ErrMatchResultTeamMismatch):
		return http.StatusConflict
	default:
		if outcome.Status == competition.CommandStatusDenied {
			return http.StatusForbidden
		}
		return http.StatusInternalServerError
	}
}

func writeCompetitionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, competition.ErrSessionNotFound),
		errors.Is(err, competition.ErrTeamNotFound),
		errors.Is(err, competition.ErrMatchNotFound),
		errors.Is(err, competition.ErrMatchResultNotFound),
		errors.Is(err, competition.ErrQueueIntentNotFound),
		errors.Is(err, competition.ErrRosterMemberNotFound),
		errors.Is(err, competition.ErrUserNotFound),
		errors.Is(err, competition.ErrSportNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, authz.ErrCapabilityDenied),
		errors.Is(err, authz.ErrTrustedSurfaceMissing),
		errors.Is(err, authz.ErrTrustedSurfaceKey),
		errors.Is(err, authz.ErrTrustedSurfaceInvalid):
		writeError(w, http.StatusForbidden, err)
	case errors.Is(err, competition.ErrSessionNameRequired),
		errors.Is(err, competition.ErrParticipantsPerSide),
		errors.Is(err, competition.ErrFacilityUnsupported),
		errors.Is(err, competition.ErrZoneUnsupported),
		errors.Is(err, competition.ErrTeamSideIndexInvalid),
		errors.Is(err, competition.ErrRosterSlotIndexInvalid),
		errors.Is(err, competition.ErrRosterSlotOutOfRange),
		errors.Is(err, competition.ErrQueueIntentTier),
		errors.Is(err, competition.ErrQueueVersionRequired),
		errors.Is(err, competition.ErrMatchIndexInvalid),
		errors.Is(err, competition.ErrMatchSideCountMismatch),
		errors.Is(err, competition.ErrMatchSideIndexInvalid),
		errors.Is(err, competition.ErrMatchResultVersion),
		errors.Is(err, competition.ErrMatchResultSideCount),
		errors.Is(err, competition.ErrMatchResultSideIndex),
		errors.Is(err, competition.ErrMatchResultOutcome),
		errors.Is(err, competition.ErrMatchResultShape):
		writeError(w, http.StatusBadRequest, err)
	case errors.Is(err, competition.ErrSessionArchived),
		errors.Is(err, competition.ErrQueueClosed),
		errors.Is(err, competition.ErrQueueMemberAlreadyJoined),
		errors.Is(err, competition.ErrQueueMemberNotFound),
		errors.Is(err, competition.ErrQueueMemberNotJoined),
		errors.Is(err, competition.ErrQueueMemberIneligible),
		errors.Is(err, competition.ErrQueueCapacityReached),
		errors.Is(err, competition.ErrQueueStateStale),
		errors.Is(err, competition.ErrQueueNotReady),
		errors.Is(err, competition.ErrQueueNotEmpty),
		errors.Is(err, competition.ErrExecutionAlreadySeeded),
		errors.Is(err, competition.ErrInvalidSessionTransition),
		errors.Is(err, competition.ErrDuplicateSession),
		errors.Is(err, competition.ErrDuplicateTeam),
		errors.Is(err, competition.ErrRosterConflict),
		errors.Is(err, competition.ErrDuplicateRosterSlot),
		errors.Is(err, competition.ErrTeamReferencedByMatch),
		errors.Is(err, competition.ErrTeamSizeMismatch),
		errors.Is(err, competition.ErrDuplicateMatch),
		errors.Is(err, competition.ErrDuplicateMatchSideIndex),
		errors.Is(err, competition.ErrDuplicateMatchTeam),
		errors.Is(err, competition.ErrSessionHasDraftMatches),
		errors.Is(err, competition.ErrMatchArchived),
		errors.Is(err, competition.ErrMatchNotInProgress),
		errors.Is(err, competition.ErrMatchResultRecorded),
		errors.Is(err, competition.ErrMatchResultStateStale),
		errors.Is(err, competition.ErrMatchResultNotRecorded),
		errors.Is(err, competition.ErrMatchResultNotCanonical),
		errors.Is(err, competition.ErrMatchResultNotFinal),
		errors.Is(err, competition.ErrMatchResultVoided),
		errors.Is(err, competition.ErrMatchResultTeamMismatch):
		writeError(w, http.StatusConflict, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func writeScheduleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, schedule.ErrResourceNotFound), errors.Is(err, schedule.ErrBlockNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, schedule.ErrBlockVersionStale):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, schedule.ErrBlockConflictRejected),
		errors.Is(err, schedule.ErrBlockResourceNotClaimable),
		errors.Is(err, schedule.ErrBlockClaimableScopeEmpty),
		errors.Is(err, schedule.ErrBlockOperatingHoursOverlap),
		errors.Is(err, schedule.ErrBlockCancelled),
		errors.Is(err, schedule.ErrBlockBookingLinked),
		errors.Is(err, schedule.ErrBlockReservationMismatch):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, schedule.ErrResourceKeyRequired),
		errors.Is(err, schedule.ErrResourceTypeRequired),
		errors.Is(err, schedule.ErrResourceDisplayNameRequired),
		errors.Is(err, schedule.ErrResourceFacilityRequired),
		errors.Is(err, schedule.ErrResourceFacilityInvalid),
		errors.Is(err, schedule.ErrResourceZoneInvalid),
		errors.Is(err, schedule.ErrResourceEdgeInvalid),
		errors.Is(err, schedule.ErrResourceEdgeSelfReference),
		errors.Is(err, schedule.ErrResourceEdgeCycle),
		errors.Is(err, schedule.ErrBlockScopeInvalid),
		errors.Is(err, schedule.ErrBlockShapeInvalid),
		errors.Is(err, schedule.ErrBlockKindInvalid),
		errors.Is(err, schedule.ErrBlockEffectInvalid),
		errors.Is(err, schedule.ErrBlockVisibilityInvalid),
		errors.Is(err, schedule.ErrBlockTimezoneRequired),
		errors.Is(err, schedule.ErrBlockRecurrenceInvalid),
		errors.Is(err, schedule.ErrBlockDateWindowInvalid),
		errors.Is(err, schedule.ErrBlockDateWindowTooLarge),
		errors.Is(err, schedule.ErrExceptionDateRequired),
		errors.Is(err, schedule.ErrExceptionNotAllowed),
		errors.Is(err, schedule.ErrExceptionWindowInvalid),
		errors.Is(err, schedule.ErrActorAttributionRequired),
		errors.Is(err, schedule.ErrActorTrustedSurfaceMissing):
		writeError(w, http.StatusBadRequest, err)
	case errors.Is(err, authz.ErrCapabilityDenied),
		errors.Is(err, authz.ErrTrustedSurfaceMissing),
		errors.Is(err, authz.ErrTrustedSurfaceKey),
		errors.Is(err, authz.ErrTrustedSurfaceInvalid):
		writeError(w, http.StatusForbidden, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func writeBookingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, booking.ErrRequestNotFound),
		errors.Is(err, booking.ErrPublicReceiptNotFound),
		errors.Is(err, schedule.ErrResourceNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, booking.ErrRequestVersionStale),
		errors.Is(err, booking.ErrRequestTransitionInvalid),
		errors.Is(err, booking.ErrLinkedScheduleBlockDrift),
		errors.Is(err, booking.ErrIdempotencyConflict),
		errors.Is(err, schedule.ErrBlockConflictRejected),
		errors.Is(err, schedule.ErrBlockResourceNotClaimable),
		errors.Is(err, schedule.ErrBlockClaimableScopeEmpty):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, authz.ErrCapabilityDenied),
		errors.Is(err, authz.ErrTrustedSurfaceMissing),
		errors.Is(err, authz.ErrTrustedSurfaceKey),
		errors.Is(err, authz.ErrTrustedSurfaceInvalid):
		writeError(w, http.StatusForbidden, err)
	case errors.Is(err, booking.ErrFacilityRequired),
		errors.Is(err, booking.ErrWindowInvalid),
		errors.Is(err, booking.ErrContactNameRequired),
		errors.Is(err, booking.ErrContactChannelRequired),
		errors.Is(err, booking.ErrContactEmailInvalid),
		errors.Is(err, booking.ErrAttendeeCountInvalid),
		errors.Is(err, booking.ErrIdempotencyKeyRequired),
		errors.Is(err, booking.ErrPublicMessageTooLong),
		errors.Is(err, booking.ErrExpectedVersionRequired),
		errors.Is(err, booking.ErrRequestActorRequired),
		errors.Is(err, booking.ErrRequestTrustedSurface),
		errors.Is(err, schedule.ErrResourceFacilityRequired),
		errors.Is(err, schedule.ErrResourceFacilityInvalid),
		errors.Is(err, schedule.ErrResourceZoneInvalid),
		errors.Is(err, schedule.ErrBlockScopeInvalid),
		errors.Is(err, schedule.ErrBlockShapeInvalid),
		errors.Is(err, schedule.ErrBlockKindInvalid),
		errors.Is(err, schedule.ErrBlockEffectInvalid),
		errors.Is(err, schedule.ErrBlockVisibilityInvalid),
		errors.Is(err, schedule.ErrBlockDateWindowInvalid),
		errors.Is(err, schedule.ErrBlockDateWindowTooLarge):
		writeError(w, http.StatusBadRequest, err)
	case errors.Is(err, booking.ErrScheduleServiceUnavailable):
		writeError(w, http.StatusServiceUnavailable, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func writePublicBookingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, booking.ErrPublicOptionNotFound),
		errors.Is(err, booking.ErrPublicReceiptNotFound),
		errors.Is(err, schedule.ErrResourceNotFound),
		errors.Is(err, schedule.ErrBlockResourceNotClaimable):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, booking.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, booking.ErrPublicOptionRequired),
		errors.Is(err, booking.ErrPublicReceiptRequired),
		errors.Is(err, booking.ErrIdempotencyKeyRequired),
		errors.Is(err, booking.ErrFacilityRequired),
		errors.Is(err, booking.ErrWindowInvalid),
		errors.Is(err, booking.ErrPublicWindowPast),
		errors.Is(err, booking.ErrPublicWindowTooFar),
		errors.Is(err, booking.ErrPublicDurationInvalid),
		errors.Is(err, booking.ErrPublicAvailabilityTooLarge),
		errors.Is(err, booking.ErrContactNameRequired),
		errors.Is(err, booking.ErrContactChannelRequired),
		errors.Is(err, booking.ErrContactEmailInvalid),
		errors.Is(err, booking.ErrContactPhoneInvalid),
		errors.Is(err, booking.ErrPublicFieldTooLong),
		errors.Is(err, booking.ErrAttendeeCountInvalid),
		errors.Is(err, schedule.ErrResourceFacilityRequired),
		errors.Is(err, schedule.ErrResourceFacilityInvalid),
		errors.Is(err, schedule.ErrResourceZoneInvalid),
		errors.Is(err, schedule.ErrBlockScopeInvalid),
		errors.Is(err, schedule.ErrBlockShapeInvalid),
		errors.Is(err, schedule.ErrBlockKindInvalid),
		errors.Is(err, schedule.ErrBlockEffectInvalid),
		errors.Is(err, schedule.ErrBlockVisibilityInvalid),
		errors.Is(err, schedule.ErrBlockDateWindowInvalid),
		errors.Is(err, schedule.ErrBlockDateWindowTooLarge):
		writeError(w, http.StatusBadRequest, err)
	case errors.Is(err, booking.ErrScheduleServiceUnavailable):
		writeError(w, http.StatusServiceUnavailable, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func writeOpsError(w http.ResponseWriter, err error) {
	var upstreamStatus *athena.UpstreamStatusError
	switch {
	case errors.Is(err, ops.ErrFacilityRequired),
		errors.Is(err, ops.ErrWindowRequired),
		errors.Is(err, ops.ErrWindowInvalid),
		errors.Is(err, ops.ErrWindowTooLarge),
		errors.Is(err, ops.ErrBucketInvalid),
		errors.Is(err, athena.ErrAnalyticsFacilityMissing),
		errors.Is(err, athena.ErrAnalyticsWindowInvalid),
		errors.Is(err, athena.ErrAnalyticsBucketInvalid),
		errors.Is(err, athena.ErrAnalyticsLimitInvalid),
		errors.Is(err, schedule.ErrResourceFacilityRequired),
		errors.Is(err, schedule.ErrResourceFacilityInvalid),
		errors.Is(err, schedule.ErrBlockDateWindowInvalid),
		errors.Is(err, schedule.ErrBlockDateWindowTooLarge):
		writeError(w, http.StatusBadRequest, err)
	case errors.Is(err, schedule.ErrResourceNotFound), errors.Is(err, schedule.ErrBlockNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, authz.ErrCapabilityDenied):
		writeError(w, http.StatusForbidden, err)
	case errors.Is(err, athena.ErrRequestTimeout):
		writeError(w, http.StatusGatewayTimeout, err)
	case errors.Is(err, athena.ErrRequestFailed),
		errors.Is(err, athena.ErrMalformedResponse),
		errors.Is(err, athena.ErrAnalyticsMalformed),
		errors.As(err, &upstreamStatus):
		writeError(w, http.StatusBadGateway, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func parseScheduleWindowBoundary(raw string, _ bool) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, errors.New("from and until are required")
	}

	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}

	return time.Time{}, errors.New("from and until must be RFC3339")
}

func parseOptionalPositiveQueryInt(raw string, field string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.New(field + " must be an integer")
	}
	if parsed <= 0 {
		return 0, errors.New(field + " must be greater than zero")
	}

	return parsed, nil
}

func writePlannerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, planner.ErrTemplateNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, planner.ErrDuplicateTemplateName):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, planner.ErrTemplateNameRequired),
		errors.Is(err, planner.ErrTemplateItemsRequired),
		errors.Is(err, planner.ErrExerciseKeyRequired),
		errors.Is(err, planner.ErrExerciseNotFound),
		errors.Is(err, planner.ErrEquipmentNotFound),
		errors.Is(err, planner.ErrEquipmentNotAllowed),
		errors.Is(err, planner.ErrSetsInvalid),
		errors.Is(err, planner.ErrRepsInvalid),
		errors.Is(err, planner.ErrWeightInvalid),
		errors.Is(err, planner.ErrRPEInvalid),
		errors.Is(err, planner.ErrWeekStartInvalid),
		errors.Is(err, planner.ErrSessionDayIndexInvalid),
		errors.Is(err, planner.ErrSessionItemsRequired),
		errors.Is(err, planner.ErrSessionShapeInvalid):
		writeError(w, http.StatusBadRequest, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func writeCoachingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, planner.ErrWeekStartInvalid),
		errors.Is(err, helper.ErrUnsupportedWhyTopic),
		errors.Is(err, helper.ErrUnsupportedVariation),
		errors.Is(err, coaching.ErrInvalidEffortLevel),
		errors.Is(err, coaching.ErrInvalidRecoveryLevel):
		writeError(w, http.StatusBadRequest, err)
	case errors.Is(err, coaching.ErrWorkoutNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, coaching.ErrWorkoutNotFinished):
		writeError(w, http.StatusConflict, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func writeNutritionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, nutrition.ErrMealLogNotFound), errors.Is(err, nutrition.ErrMealTemplateNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, nutrition.ErrDuplicateMealTemplateName):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, nutrition.ErrMealNameRequired),
		errors.Is(err, helper.ErrUnsupportedWhyTopic),
		errors.Is(err, helper.ErrUnsupportedVariation),
		errors.Is(err, nutrition.ErrMealTemplateNameRequired),
		errors.Is(err, nutrition.ErrMealTypeInvalid),
		errors.Is(err, nutrition.ErrCaloriesInvalid),
		errors.Is(err, nutrition.ErrProteinInvalid),
		errors.Is(err, nutrition.ErrCarbsInvalid),
		errors.Is(err, nutrition.ErrFatInvalid),
		errors.Is(err, nutrition.ErrMealTemplateNutritionRequired),
		errors.Is(err, nutrition.ErrMealLogNutritionRequired),
		errors.Is(err, nutrition.ErrLoggedAtRequired):
		writeError(w, http.StatusBadRequest, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}
