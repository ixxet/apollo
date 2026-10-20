package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/schedule"
	"github.com/ixxet/apollo/internal/testutil"
)

func TestBookingClosedTestFrontendsRealPostgres(t *testing.T) {
	if os.Getenv("APOLLO_BOOKING_FRONTEND_REHEARSAL") != "1" {
		t.Skip("set APOLLO_BOOKING_FRONTEND_REHEARSAL=1 with built sibling Hestia/Themis and existing Playwright browsers")
	}
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)
	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "booking-rehearsal-manager", "manager@example.invalid")
	setUserRole(t, env, manager.ID, authz.RoleManager)
	optionID := insertPublicBookingHTTPResource(t, env, "rehearsal-court", "Rehearsal Court", true, true)
	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Hour)
	end := start.Add(time.Hour)
	createAvailabilityScheduleBlock(t, env, managerCookie, publicAvailabilityOneOffBlockJSON("rehearsal-court", schedule.KindOperatingHours, schedule.EffectInformational, schedule.VisibilityPublicLabeled, start.Format(time.RFC3339), end.Format(time.RFC3339)))
	response := env.doJSONRequest(t, http.MethodPost, "/api/v1/auth/verification/start", `{"student_id":"booking-rehearsal-manager","email":"manager@example.invalid"}`)
	if response.Code != http.StatusAccepted {
		t.Fatal("could not start synthetic staff verification")
	}
	token := env.sender.lastToken(t)
	var mu sync.Mutex
	seen := map[string]bool{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.handler.ServeHTTP(w, r)
		var state string
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/public/booking/requests":
			state = "requested"
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/approve"):
			state = "approved"
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cancel"):
			state = "cancelled"
		default:
			return
		}
		var requests, reservations int
		if err := env.db.DB.QueryRow(r.Context(), `SELECT count(*) FROM apollo.booking_requests WHERE status=$1`, state).Scan(&requests); err != nil {
			t.Error(err)
			return
		}
		blockState := "scheduled"
		if state == "cancelled" {
			blockState = "cancelled"
		}
		if err := env.db.DB.QueryRow(r.Context(), `SELECT count(*) FROM apollo.schedule_blocks WHERE kind='reservation' AND status=$1`, blockState).Scan(&reservations); err != nil {
			t.Error(err)
			return
		}
		wantReservations := 1
		if state == "requested" {
			wantReservations = 0
		}
		if requests != 1 || reservations != wantReservations {
			t.Errorf("%s: requests=%d reservations=%d, want 1/%d", state, requests, reservations, wantReservations)
		}
		mu.Lock()
		seen[state] = true
		mu.Unlock()
	}))
	defer upstream.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	hestia := filepath.Clean(testutil.RepoFilePath("..", "hestia"))
	themis := filepath.Clean(testutil.RepoFilePath("..", "themis"))
	startFrontend := func(directory, host string) string {
		t.Helper()
		if _, err := os.Stat(filepath.Join(directory, "build", "index.js")); err != nil {
			t.Fatalf("build %s before running the rehearsal", filepath.Base(directory))
		}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		_ = listener.Close()
		origin := fmt.Sprintf("http://%s:%d", host, port)
		child := exec.CommandContext(ctx, "node", "build")
		child.Dir = directory
		child.Env = append(os.Environ(), "HOST=127.0.0.1", fmt.Sprintf("PORT=%d", port), "ORIGIN="+origin, "APOLLO_ORIGIN="+upstream.URL, "THEMIS_DEFAULT_FACILITY_KEY=ashtonbee", "THEMIS_APOLLO_TRUSTED_SURFACE="+env.trustedSurfaceKey, "THEMIS_APOLLO_TRUSTED_SURFACE_TOKEN="+env.trustedSurfaceToken)
		child.Stdout = io.Discard
		child.Stderr = io.Discard
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		deferCleanup := func() { _ = child.Process.Kill(); _ = child.Wait() }
		t.Cleanup(deferCleanup)
		client := &http.Client{Timeout: time.Second}
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
			result, err := client.Get(origin + "/login")
			if err == nil {
				_ = result.Body.Close()
				if result.StatusCode < 500 {
					return origin
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("%s did not become ready", filepath.Base(directory))
		return ""
	}
	publicOrigin := startFrontend(hestia, "127.0.0.1")
	staffOrigin := startFrontend(themis, "localhost")
	toronto, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	browser := exec.CommandContext(ctx, "node", "--input-type=module", "-e", bookingFrontendBrowserRehearsal)
	browser.Dir = themis
	browser.Env = append(os.Environ(), "BOOKING_PUBLIC_ORIGIN="+publicOrigin, "BOOKING_STAFF_ORIGIN="+staffOrigin, "BOOKING_STAFF_TOKEN="+token, "BOOKING_OPTION="+optionID.String(), "BOOKING_DAY="+start.In(toronto).Format("2006-01-02"))
	output, err := browser.CombinedOutput()
	if err != nil {
		t.Fatalf("real frontend rehearsal failed: %v\n%s", err, strings.ReplaceAll(string(output), token, "[redacted]"))
	}
	for _, state := range []string{"requested", "approved", "cancelled"} {
		mu.Lock()
		present := seen[state]
		mu.Unlock()
		if !present {
			t.Errorf("missing real HTTP/SQL checkpoint %s", state)
		}
	}
	var forbiddenWrites int
	if err := env.db.DB.QueryRow(ctx, `SELECT (SELECT count(*) FROM apollo.visits)+(SELECT count(*) FROM apollo.visit_tap_links)`).Scan(&forbiddenWrites); err != nil {
		t.Fatal(err)
	}
	if forbiddenWrites != 0 {
		t.Fatalf("unrelated visit/tap writes=%d", forbiddenWrites)
	}
	t.Log("PASS: real Hestia request/receipt/status -> Themis verified login/approval/message/cancellation -> APOLLO PostgreSQL; pending=1/reservations=0, approved=1/reservations=1, cancelled=1/reservations cancelled=1; no ATHENA dependency")
}

const bookingFrontendBrowserRehearsal = `
import assert from 'node:assert/strict';
import { chromium, expect } from '@playwright/test';
const browser=await chromium.launch({headless:true});
try {
 const customerContext=await browser.newContext({timezoneId:'America/Toronto'});
 const staffContext=await browser.newContext({timezoneId:'America/Toronto'});
 const customer=await customerContext.newPage(); const staff=await staffContext.newPage();
 await customer.goto(process.env.BOOKING_PUBLIC_ORIGIN+'/intake',{waitUntil:'networkidle'});
 await customer.getByRole('button',{name:/Rehearsal Court/}).click();
 await customer.locator('input[name="availability_date"]').fill(process.env.BOOKING_DAY);
 await customer.getByRole('button',{name:'Check availability'}).click();
 await customer.waitForURL(/from=/); await customer.waitForLoadState('networkidle');
 await customer.getByRole('button',{name:'Use this requestable time'}).click();
 await customer.locator('input[name="contact_name"]').fill('Booking Rehearsal');
 await customer.locator('input[name="contact_email"]').fill('customer@example.invalid');
 await customer.getByRole('button',{name:'Send request'}).click();
 const receipt=await customer.locator('.receipt-box strong').innerText();
 assert.match(receipt,/^BR-/);
 await customer.getByRole('link',{name:'Check status',exact:true}).click();
 await expect(customer.getByRole('heading',{name:'Received',exact:true})).toBeVisible();
 const statusURL=customer.url();
 await staff.goto(process.env.BOOKING_STAFF_ORIGIN+'/login',{waitUntil:'networkidle'});
 await staff.getByLabel('Sign-in code',{exact:true}).fill(process.env.BOOKING_STAFF_TOKEN);
 await staff.getByRole('button',{name:'Verify and continue'}).click();
 await staff.waitForURL(/ops\/facilities/);
 const cookie=(await staffContext.cookies()).find(c=>c.name==='apollo_session');
 assert.ok(cookie?.httpOnly); assert.equal(cookie.sameSite,'Strict');assert.ok(cookie.secure);
 await staff.getByRole('link',{name:'Requests',exact:true}).click();
 await staff.getByRole('link').filter({hasText:'Booking Rehearsal'}).click();
 await staff.waitForLoadState('networkidle');
 await staff.getByLabel('Public message',{exact:true}).fill('Your requested time is approved.');
 await staff.getByRole('button',{name:'Save public message'}).click();
 await expect(staff.getByText('Your requested time is approved.').first()).toBeVisible();
 await staff.getByRole('button',{name:'Approve request',exact:true}).click();
 await expect(staff.locator('.metric-card').filter({has:staff.getByText('Status',{exact:true})}).getByText('Approved',{exact:true})).toBeVisible();
 await customer.goto(statusURL,{waitUntil:'networkidle'});
 await expect(customer.getByRole('heading',{name:'Approved',exact:true})).toBeVisible();
 await expect(customer.getByText('Your requested time is approved.')).toBeVisible();
 assert.ok(!(await customer.locator('body').innerText()).includes('staff-secret'));
 assert.equal((await customer.request.get(process.env.BOOKING_PUBLIC_ORIGIN+'/api/v1/booking/requests')).status(),404);
 await staff.getByRole('button',{name:'Cancel approved booking',exact:true}).click();
 await expect(staff.locator('.metric-card').filter({has:staff.getByText('Status',{exact:true})}).getByText('Cancelled',{exact:true})).toBeVisible();
 await customer.goto(statusURL,{waitUntil:'networkidle'});
 await expect(customer.getByRole('heading',{name:'Cancelled',exact:true})).toBeVisible();
} finally {await browser.close();}
`
