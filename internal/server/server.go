package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/eligibility"
	"github.com/ixxet/apollo/internal/profile"
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

type EligibilityReader interface {
	GetLobbyEligibility(ctx context.Context, userID uuid.UUID) (eligibility.LobbyEligibility, error)
}

type WorkoutManager interface {
	CreateWorkout(ctx context.Context, userID uuid.UUID, input workouts.CreateInput) (workouts.Workout, error)
	ListWorkouts(ctx context.Context, userID uuid.UUID) ([]workouts.Workout, error)
	GetWorkout(ctx context.Context, userID uuid.UUID, workoutID uuid.UUID) (workouts.Workout, error)
	UpdateWorkout(ctx context.Context, userID uuid.UUID, workoutID uuid.UUID, input workouts.UpdateInput) (workouts.Workout, error)
	FinishWorkout(ctx context.Context, userID uuid.UUID, workoutID uuid.UUID) (workouts.Workout, error)
}

type Dependencies struct {
	ConsumerEnabled bool
	Auth            Authenticator
	Profile         Profiler
	Eligibility     EligibilityReader
	Workouts        WorkoutManager
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

type createWorkoutRequest struct {
	Notes *string `json:"notes"`
}

type updateWorkoutRequest struct {
	Notes     *string                   `json:"notes"`
	Exercises *[]workouts.ExerciseInput `json:"exercises"`
}

type contextKey string

const principalContextKey contextKey = "session_principal"

func NewHandler(deps Dependencies) http.Handler {
	router := chi.NewRouter()
	router.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{
			Service:         "apollo",
			Status:          "ok",
			ConsumerEnabled: deps.ConsumerEnabled,
		})
	})

	router.Post("/api/v1/auth/verification/start", func(w http.ResponseWriter, r *http.Request) {
		var request startVerificationRequest
		if err := decodeJSONBody(r, &request); err != nil {
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
			if err := decodeJSONBody(r, &request); err != nil {
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
		authenticated.Post("/api/v1/workouts", func(w http.ResponseWriter, r *http.Request) {
			var request createWorkoutRequest
			if err := decodeJSONBodyAllowEmpty(r, &request); err != nil {
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
			if err := decodeJSONBody(r, &request); err != nil {
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

			writeJSON(w, http.StatusOK, workout)
		})
		authenticated.Patch("/api/v1/profile", func(w http.ResponseWriter, r *http.Request) {
			var request profile.UpdateInput
			if err := decodeJSONBody(r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			principal := principalFromContext(r.Context())
			memberProfile, err := deps.Profile.UpdateProfile(r.Context(), principal.UserID, request)
			if err != nil {
				switch {
				case errors.Is(err, profile.ErrEmptyPatch), errors.Is(err, profile.ErrInvalidVisibilityMode), errors.Is(err, profile.ErrInvalidAvailabilityMode):
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

func decodeJSONBody(r *http.Request, target any) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}

func decodeJSONBodyAllowEmpty(r *http.Request, target any) error {
	if r.Body == nil {
		return nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return nil
	}

	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, errorResponse{Error: err.Error()})
}
