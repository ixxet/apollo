package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/ares"
	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/coaching"
	"github.com/ixxet/apollo/internal/competition"
	"github.com/ixxet/apollo/internal/eligibility"
	"github.com/ixxet/apollo/internal/exercises"
	"github.com/ixxet/apollo/internal/membership"
	"github.com/ixxet/apollo/internal/nutrition"
	"github.com/ixxet/apollo/internal/planner"
	"github.com/ixxet/apollo/internal/presence"
	"github.com/ixxet/apollo/internal/profile"
	"github.com/ixxet/apollo/internal/recommendations"
	"github.com/ixxet/apollo/internal/store"
	"github.com/ixxet/apollo/internal/testutil"
	"github.com/ixxet/apollo/internal/visits"
	"github.com/ixxet/apollo/internal/workouts"
)

type integrationEmailSender struct {
	deliveries []auth.VerificationDelivery
}

func (s *integrationEmailSender) SendVerification(_ context.Context, delivery auth.VerificationDelivery) error {
	s.deliveries = append(s.deliveries, delivery)
	return nil
}

func (s *integrationEmailSender) lastToken(t *testing.T) string {
	t.Helper()

	if len(s.deliveries) == 0 {
		t.Fatal("no verification deliveries captured")
	}

	return s.deliveries[len(s.deliveries)-1].Token
}

type authProfileServerEnv struct {
	db                  *testutil.PostgresEnv
	handler             http.Handler
	sender              *integrationEmailSender
	cookies             *auth.SessionCookieManager
	queries             *store.Queries
	trustedSurfaceKey   string
	trustedSurfaceToken string
}

func TestRegistrationVerificationAndProfileRoundTrip(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	startResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/auth/verification/start", `{"student_id":"student-001","email":"member@example.com"}`)
	if startResponse.Code != http.StatusAccepted {
		t.Fatalf("startResponse.Code = %d, want %d", startResponse.Code, http.StatusAccepted)
	}

	verifyResponse := env.doJSONRequest(t, http.MethodGet, "/api/v1/auth/verify?token="+env.sender.lastToken(t), "")
	if verifyResponse.Code != http.StatusOK {
		t.Fatalf("verifyResponse.Code = %d, want %d", verifyResponse.Code, http.StatusOK)
	}

	cookie := sessionCookieFromResponse(t, verifyResponse)
	if !cookie.HttpOnly {
		t.Fatal("cookie.HttpOnly = false, want true")
	}
	if !cookie.Secure {
		t.Fatal("cookie.Secure = false, want true")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie.SameSite = %v, want %v", cookie.SameSite, http.SameSiteStrictMode)
	}

	profileResponse := env.doRequest(t, http.MethodGet, "/api/v1/profile", nil, cookie)
	if profileResponse.Code != http.StatusOK {
		t.Fatalf("profileResponse.Code = %d, want %d", profileResponse.Code, http.StatusOK)
	}

	memberProfile := decodeProfileResponse(t, profileResponse)
	if memberProfile.VisibilityMode != profile.VisibilityModeGhost {
		t.Fatalf("VisibilityMode = %q, want %q", memberProfile.VisibilityMode, profile.VisibilityModeGhost)
	}
	if memberProfile.AvailabilityMode != profile.AvailabilityModeUnavailable {
		t.Fatalf("AvailabilityMode = %q, want %q", memberProfile.AvailabilityMode, profile.AvailabilityModeUnavailable)
	}
	if !memberProfile.EmailVerified {
		t.Fatal("EmailVerified = false, want true")
	}

	patchVisibilityResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"discoverable"}`, cookie)
	if patchVisibilityResponse.Code != http.StatusOK {
		t.Fatalf("patchVisibilityResponse.Code = %d, want %d", patchVisibilityResponse.Code, http.StatusOK)
	}
	patchedVisibilityProfile := decodeProfileResponse(t, patchVisibilityResponse)
	if patchedVisibilityProfile.VisibilityMode != profile.VisibilityModeDiscoverable {
		t.Fatalf("VisibilityMode = %q, want %q", patchedVisibilityProfile.VisibilityMode, profile.VisibilityModeDiscoverable)
	}
	if patchedVisibilityProfile.AvailabilityMode != profile.AvailabilityModeUnavailable {
		t.Fatalf("AvailabilityMode = %q, want %q", patchedVisibilityProfile.AvailabilityMode, profile.AvailabilityModeUnavailable)
	}

	patchAvailabilityResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"availability_mode":"available_now"}`, cookie)
	if patchAvailabilityResponse.Code != http.StatusOK {
		t.Fatalf("patchAvailabilityResponse.Code = %d, want %d", patchAvailabilityResponse.Code, http.StatusOK)
	}
	patchedAvailabilityProfile := decodeProfileResponse(t, patchAvailabilityResponse)
	if patchedAvailabilityProfile.VisibilityMode != profile.VisibilityModeDiscoverable {
		t.Fatalf("VisibilityMode = %q, want %q", patchedAvailabilityProfile.VisibilityMode, profile.VisibilityModeDiscoverable)
	}
	if patchedAvailabilityProfile.AvailabilityMode != profile.AvailabilityModeAvailableNow {
		t.Fatalf("AvailabilityMode = %q, want %q", patchedAvailabilityProfile.AvailabilityMode, profile.AvailabilityModeAvailableNow)
	}

	secondProfileResponse := env.doRequest(t, http.MethodGet, "/api/v1/profile", nil, cookie)
	if secondProfileResponse.Code != http.StatusOK {
		t.Fatalf("secondProfileResponse.Code = %d, want %d", secondProfileResponse.Code, http.StatusOK)
	}
	secondProfile := decodeProfileResponse(t, secondProfileResponse)
	if secondProfile.VisibilityMode != profile.VisibilityModeDiscoverable || secondProfile.AvailabilityMode != profile.AvailabilityModeAvailableNow {
		t.Fatalf("profile round-trip mismatch: %#v", secondProfile)
	}
}

func TestAuthAndProfileEndpointsRejectTokenAndSessionEdgeCases(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	missingCookieResponse := env.doRequest(t, http.MethodGet, "/api/v1/profile", nil)
	if missingCookieResponse.Code != http.StatusUnauthorized {
		t.Fatalf("missingCookieResponse.Code = %d, want %d", missingCookieResponse.Code, http.StatusUnauthorized)
	}

	invalidEmailResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/auth/verification/start", `{"student_id":"student-001","email":"not-an-email"}`)
	if invalidEmailResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidEmailResponse.Code = %d, want %d", invalidEmailResponse.Code, http.StatusBadRequest)
	}

	firstStart := env.doJSONRequest(t, http.MethodPost, "/api/v1/auth/verification/start", `{"student_id":"student-001","email":"member@example.com"}`)
	if firstStart.Code != http.StatusAccepted {
		t.Fatalf("firstStart.Code = %d, want %d", firstStart.Code, http.StatusAccepted)
	}
	duplicateStudentResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/auth/verification/start", `{"student_id":"student-001","email":"other@example.com"}`)
	if duplicateStudentResponse.Code != http.StatusConflict {
		t.Fatalf("duplicateStudentResponse.Code = %d, want %d", duplicateStudentResponse.Code, http.StatusConflict)
	}
	duplicateEmailResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/auth/verification/start", `{"student_id":"student-002","email":"member@example.com"}`)
	if duplicateEmailResponse.Code != http.StatusConflict {
		t.Fatalf("duplicateEmailResponse.Code = %d, want %d", duplicateEmailResponse.Code, http.StatusConflict)
	}

	expiredStart := env.doJSONRequest(t, http.MethodPost, "/api/v1/auth/verification/start", `{"student_id":"student-003","email":"expired@example.com"}`)
	if expiredStart.Code != http.StatusAccepted {
		t.Fatalf("expiredStart.Code = %d, want %d", expiredStart.Code, http.StatusAccepted)
	}
	expiredToken := env.sender.lastToken(t)
	if _, err := env.db.DB.Exec(context.Background(), "UPDATE apollo.email_verification_tokens SET expires_at = $2 WHERE token_hash = $1", tokenHash(expiredToken), time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(expire token) error = %v", err)
	}
	expiredVerifyResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/auth/verify", `{"token":"`+expiredToken+`"}`)
	if expiredVerifyResponse.Code != http.StatusGone {
		t.Fatalf("expiredVerifyResponse.Code = %d, want %d", expiredVerifyResponse.Code, http.StatusGone)
	}

	invalidTokenResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/auth/verify", `{"token":"missing-token"}`)
	if invalidTokenResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidTokenResponse.Code = %d, want %d", invalidTokenResponse.Code, http.StatusBadRequest)
	}

	verifyResponse := env.doJSONRequest(t, http.MethodGet, "/api/v1/auth/verify?token="+env.sender.deliveries[0].Token, "")
	if verifyResponse.Code != http.StatusOK {
		t.Fatalf("verifyResponse.Code = %d, want %d", verifyResponse.Code, http.StatusOK)
	}
	reuseVerifyResponse := env.doJSONRequest(t, http.MethodGet, "/api/v1/auth/verify?token="+env.sender.deliveries[0].Token, "")
	if reuseVerifyResponse.Code != http.StatusConflict {
		t.Fatalf("reuseVerifyResponse.Code = %d, want %d", reuseVerifyResponse.Code, http.StatusConflict)
	}

	validCookie := sessionCookieFromResponse(t, verifyResponse)
	tamperedCookie := *validCookie
	tamperedCookie.Value = tamperSignedCookieValue(t, validCookie.Value)
	tamperedCookieResponse := env.doRequest(t, http.MethodGet, "/api/v1/profile", nil, &tamperedCookie)
	if tamperedCookieResponse.Code != http.StatusUnauthorized {
		t.Fatalf("tamperedCookieResponse.Code = %d, want %d", tamperedCookieResponse.Code, http.StatusUnauthorized)
	}

	expiredUser := createVerifiedUser(t, env, "student-expired", "session-expired@example.com")
	expiredSession, err := env.queries.CreateSession(context.Background(), store.CreateSessionParams{
		UserID:    expiredUser.ID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateSession(expired) error = %v", err)
	}
	expiredCookie := env.cookies.SessionCookie(expiredSession.ID, expiredSession.ExpiresAt.Time)
	expiredCookieResponse := env.doRequest(t, http.MethodGet, "/api/v1/profile", nil, expiredCookie)
	if expiredCookieResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expiredCookieResponse.Code = %d, want %d", expiredCookieResponse.Code, http.StatusUnauthorized)
	}

	logoutResponse := env.doRequest(t, http.MethodPost, "/api/v1/auth/logout", nil, validCookie)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logoutResponse.Code = %d, want %d", logoutResponse.Code, http.StatusNoContent)
	}
	repeatedLogoutResponse := env.doRequest(t, http.MethodPost, "/api/v1/auth/logout", nil, validCookie)
	if repeatedLogoutResponse.Code != http.StatusUnauthorized {
		t.Fatalf("repeatedLogoutResponse.Code = %d, want %d", repeatedLogoutResponse.Code, http.StatusUnauthorized)
	}
	expiredLogoutResponse := env.doRequest(t, http.MethodPost, "/api/v1/auth/logout", nil, expiredCookie)
	if expiredLogoutResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expiredLogoutResponse.Code = %d, want %d", expiredLogoutResponse.Code, http.StatusUnauthorized)
	}
	postLogoutResponse := env.doRequest(t, http.MethodGet, "/api/v1/profile", nil, validCookie)
	if postLogoutResponse.Code != http.StatusUnauthorized {
		t.Fatalf("postLogoutResponse.Code = %d, want %d", postLogoutResponse.Code, http.StatusUnauthorized)
	}
}

func TestProfilePatchRejectsInvalidBodiesAndDoesNotTouchVisitsWorkoutsOrClaimedTags(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	verifiedCookie, user := createVerifiedSessionViaHTTP(t, env, "student-100", "profile@example.com")

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label) VALUES ($1, $2, $3)", user.ID, "tag-profile-001", "locker tag"); err != nil {
		t.Fatalf("Exec(insert claimed tag) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.visits (user_id, facility_key, source_event_id, arrived_at) VALUES ($1, $2, $3, $4)", user.ID, "ashtonbee", "visit-001", time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(insert visit) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.workouts (user_id, started_at, status, finished_at) VALUES ($1, $2, $3, $4)", user.ID, time.Date(2026, 4, 2, 12, 5, 0, 0, time.UTC), "finished", time.Date(2026, 4, 2, 13, 5, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(insert workout) error = %v", err)
	}

	beforeVisits := countRows(t, env, "apollo.visits", user.ID)
	beforeWorkouts := countRows(t, env, "apollo.workouts", user.ID)
	beforeClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID)
	tagHashBefore := lookupTagHash(t, env, user.ID)

	invalidPatchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"hidden"}`, verifiedCookie)
	if invalidPatchResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidPatchResponse.Code = %d, want %d", invalidPatchResponse.Code, http.StatusBadRequest)
	}
	emptyPatchResponse := env.doRequest(t, http.MethodPatch, "/api/v1/profile", bytes.NewBufferString(""), verifiedCookie)
	if emptyPatchResponse.Code != http.StatusBadRequest {
		t.Fatalf("emptyPatchResponse.Code = %d, want %d", emptyPatchResponse.Code, http.StatusBadRequest)
	}
	malformedPatchResponse := env.doRequest(t, http.MethodPatch, "/api/v1/profile", bytes.NewBufferString("{"), verifiedCookie)
	if malformedPatchResponse.Code != http.StatusBadRequest {
		t.Fatalf("malformedPatchResponse.Code = %d, want %d", malformedPatchResponse.Code, http.StatusBadRequest)
	}

	validPatchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"discoverable","availability_mode":"with_team"}`, verifiedCookie)
	if validPatchResponse.Code != http.StatusOK {
		t.Fatalf("validPatchResponse.Code = %d, want %d", validPatchResponse.Code, http.StatusOK)
	}

	if afterVisits := countRows(t, env, "apollo.visits", user.ID); afterVisits != beforeVisits {
		t.Fatalf("visit count changed from %d to %d after profile patch", beforeVisits, afterVisits)
	}
	if afterWorkouts := countRows(t, env, "apollo.workouts", user.ID); afterWorkouts != beforeWorkouts {
		t.Fatalf("workout count changed from %d to %d after profile patch", beforeWorkouts, afterWorkouts)
	}
	if afterClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID); afterClaimedTags != beforeClaimedTags {
		t.Fatalf("claimed tag count changed from %d to %d after profile patch", beforeClaimedTags, afterClaimedTags)
	}
	if tagHashAfter := lookupTagHash(t, env, user.ID); tagHashAfter != tagHashBefore {
		t.Fatalf("tag hash changed from %q to %q after profile patch", tagHashBefore, tagHashAfter)
	}
}

func newAuthProfileServerEnv(t *testing.T) *authProfileServerEnv {
	t.Helper()

	ctx := context.Background()
	db, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	if err := testutil.ApplyApolloSchema(ctx, db.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}
	t.Setenv(authz.TrustedSurfaceConfigEnv, "staff-console=staff-secret")

	cookies, err := auth.NewSessionCookieManager("apollo_session", "0123456789abcdef0123456789abcdef", true)
	if err != nil {
		t.Fatalf("NewSessionCookieManager() error = %v", err)
	}
	sender := &integrationEmailSender{}
	authService := auth.NewService(auth.NewRepository(db.DB), cookies, sender, 15*time.Minute, 7*24*time.Hour)
	visitRepository := visits.NewRepository(db.DB)
	visitService := visits.NewService(visitRepository)
	presenceService := presence.NewService(presence.NewRepository(db.DB), visitService)
	exerciseRepository := exercises.NewRepository(db.DB)
	exerciseService := exercises.NewService(exerciseRepository)
	plannerRepository := planner.NewRepository(db.DB)
	plannerService := planner.NewService(plannerRepository, exerciseService)
	profileRepository := profile.NewRepository(db.DB)
	profileService := profile.NewService(profileRepository, exerciseService)
	eligibilityService := eligibility.NewService(profileRepository)
	membershipService := membership.NewService(membership.NewRepository(db.DB), eligibilityService)
	matchPreviewService := ares.NewService(ares.NewRepository(db.DB))
	competitionService := competition.NewService(competition.NewRepository(db.DB))
	recommendationService := recommendations.NewService(recommendations.NewRepository(db.DB))
	coachingService := coaching.NewService(coaching.NewRepository(db.DB), plannerService, profileService)
	nutritionService := nutrition.NewService(nutrition.NewRepository(db.DB), profileService)
	workoutService := workouts.NewService(workouts.NewRepository(db.DB))

	return &authProfileServerEnv{
		db: db,
		handler: NewHandler(Dependencies{
			Auth:            authService,
			Competition:     competitionService,
			Profile:         profileService,
			Presence:        presenceService,
			Exercises:       exerciseService,
			Planner:         plannerService,
			Eligibility:     eligibilityService,
			Membership:      membershipService,
			MatchPreview:    matchPreviewService,
			Recommendations: recommendationService,
			Coaching:        coachingService,
			Nutrition:       nutritionService,
			Workouts:        workoutService,
		}),
		sender:              sender,
		cookies:             cookies,
		queries:             store.New(db.DB),
		trustedSurfaceKey:   "staff-console",
		trustedSurfaceToken: "staff-secret",
	}
}

func closeServerEnv(t *testing.T, env *authProfileServerEnv) {
	t.Helper()

	if err := env.db.Close(); err != nil {
		t.Fatalf("env.db.Close() error = %v", err)
	}
}

func (e *authProfileServerEnv) doJSONRequest(t *testing.T, method string, path string, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	return e.doRequest(t, method, path, bytes.NewBufferString(body), cookies...)
}

func (e *authProfileServerEnv) doRequest(t *testing.T, method string, path string, body *bytes.Buffer, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	return e.doRequestInternal(t, method, path, body, true, nil, cookies...)
}

func (e *authProfileServerEnv) doRequestWithoutTrustedSurface(t *testing.T, method string, path string, body *bytes.Buffer, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	return e.doRequestInternal(t, method, path, body, false, nil, cookies...)
}

func (e *authProfileServerEnv) doRequestWithHeaders(t *testing.T, method string, path string, body *bytes.Buffer, headers map[string]string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	return e.doRequestInternal(t, method, path, body, false, headers, cookies...)
}

func (e *authProfileServerEnv) doRequestInternal(t *testing.T, method string, path string, body *bytes.Buffer, autoTrustedSurface bool, headers map[string]string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Buffer
	if body == nil {
		reader = bytes.NewBuffer(nil)
	} else {
		reader = body
	}

	request := httptest.NewRequest(method, path, reader)
	if method == http.MethodPost || method == http.MethodPatch || method == http.MethodPut {
		request.Header.Set("Content-Type", "application/json")
	}
	if autoTrustedSurface && method == http.MethodPost && strings.HasPrefix(path, "/api/v1/competition/") {
		request.Header.Set(authz.TrustedSurfaceHeader, e.trustedSurfaceKey)
		request.Header.Set(authz.TrustedSurfaceTokenHeader, e.trustedSurfaceToken)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}

	recorder := httptest.NewRecorder()
	e.handler.ServeHTTP(recorder, request)
	return recorder
}

func sessionCookieFromResponse(t *testing.T, recorder *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	result := recorder.Result()
	defer result.Body.Close()
	cookies := result.Cookies()
	if len(cookies) == 0 {
		t.Fatal("response did not include a session cookie")
	}
	return cookies[0]
}

func decodeProfileResponse(t *testing.T, recorder *httptest.ResponseRecorder) profile.MemberProfile {
	t.Helper()

	var memberProfile profile.MemberProfile
	if err := json.Unmarshal(recorder.Body.Bytes(), &memberProfile); err != nil {
		t.Fatalf("json.Unmarshal(profile) error = %v", err)
	}
	return memberProfile
}

func createVerifiedSessionViaHTTP(t *testing.T, env *authProfileServerEnv, studentID string, email string) (*http.Cookie, store.ApolloUser) {
	t.Helper()

	startResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/auth/verification/start", `{"student_id":"`+studentID+`","email":"`+email+`"}`)
	if startResponse.Code != http.StatusAccepted {
		t.Fatalf("startResponse.Code = %d, want %d", startResponse.Code, http.StatusAccepted)
	}

	verifyResponse := env.doJSONRequest(t, http.MethodGet, "/api/v1/auth/verify?token="+env.sender.lastToken(t), "")
	if verifyResponse.Code != http.StatusOK {
		t.Fatalf("verifyResponse.Code = %d, want %d", verifyResponse.Code, http.StatusOK)
	}

	user, err := env.queries.GetUserByStudentID(context.Background(), studentID)
	if err != nil {
		t.Fatalf("GetUserByStudentID() error = %v", err)
	}

	return sessionCookieFromResponse(t, verifyResponse), store.ApolloUserFromGetUserByStudentIDRow(user)
}

func createVerifiedUser(t *testing.T, env *authProfileServerEnv, studentID string, email string) store.ApolloUser {
	t.Helper()

	user, err := env.queries.CreateUser(context.Background(), store.CreateUserParams{
		StudentID:   studentID,
		DisplayName: studentID,
		Email:       email,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "UPDATE apollo.users SET email_verified_at = $2 WHERE id = $1", user.ID, time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(mark email verified) error = %v", err)
	}
	verifiedUser, err := env.queries.GetUserByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	return store.ApolloUserFromGetUserByIDRow(verifiedUser)
}

func setUserRole(t *testing.T, env *authProfileServerEnv, userID uuid.UUID, role authz.Role) store.ApolloUser {
	t.Helper()

	row, err := env.queries.SetUserRole(context.Background(), store.SetUserRoleParams{
		ID:   userID,
		Role: string(role),
	})
	if err != nil {
		t.Fatalf("SetUserRole() error = %v", err)
	}

	return store.ApolloUser{
		ID:              row.ID,
		StudentID:       row.StudentID,
		DisplayName:     row.DisplayName,
		Email:           row.Email,
		Role:            row.Role,
		Preferences:     row.Preferences,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		EmailVerifiedAt: row.EmailVerifiedAt,
	}
}

func countRows(t *testing.T, env *authProfileServerEnv, table string, userID uuid.UUID) int {
	t.Helper()

	var count int
	query := "SELECT count(*) FROM " + table + " WHERE user_id = $1"
	if err := env.db.DB.QueryRow(context.Background(), query, userID).Scan(&count); err != nil {
		t.Fatalf("QueryRow(%s) error = %v", table, err)
	}
	return count
}

func lookupTagHash(t *testing.T, env *authProfileServerEnv, userID uuid.UUID) string {
	t.Helper()

	var tagHash string
	if err := env.db.DB.QueryRow(context.Background(), "SELECT tag_hash FROM apollo.claimed_tags WHERE user_id = $1 LIMIT 1", userID).Scan(&tagHash); err != nil {
		t.Fatalf("QueryRow(tag_hash) error = %v", err)
	}
	return tagHash
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func tamperSignedCookieValue(t *testing.T, value string) string {
	t.Helper()

	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		t.Fatalf("cookie format = %q, want two parts", value)
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	if len(signature) == 0 {
		t.Fatal("signature length = 0, want non-empty signature")
	}

	signature[0] ^= 0xFF

	return parts[0] + "." + base64.RawURLEncoding.EncodeToString(signature)
}
