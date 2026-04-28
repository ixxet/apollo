package authz

import (
	"net/http/httptest"
	"testing"
)

func TestCapabilitiesForRoleRemainDeterministic(t *testing.T) {
	capabilities := CapabilitiesForRole(RoleManager)
	expected := []Capability{
		CapabilityCompetitionLiveManage,
		CapabilityCompetitionRead,
		CapabilityCompetitionSafetyReview,
		CapabilityCompetitionStructureManage,
	}
	if len(capabilities) != len(expected) {
		t.Fatalf("len(capabilities) = %d, want %d", len(capabilities), len(expected))
	}
	for index, capability := range expected {
		if capabilities[index] != capability {
			t.Fatalf("capabilities[%d] = %q, want %q", index, capabilities[index], capability)
		}
	}
}

func TestScheduleCapabilitiesForRoleRemainDeterministic(t *testing.T) {
	capabilities := ScheduleCapabilitiesForRole(RoleManager)
	expected := []Capability{
		CapabilityScheduleManage,
		CapabilityScheduleRead,
	}
	if len(capabilities) != len(expected) {
		t.Fatalf("len(capabilities) = %d, want %d", len(capabilities), len(expected))
	}
	for index, capability := range expected {
		if capabilities[index] != capability {
			t.Fatalf("capabilities[%d] = %q, want %q", index, capabilities[index], capability)
		}
	}
}

func TestOpsCapabilitiesForRoleRemainDeterministicAndMemberDenied(t *testing.T) {
	for _, role := range []Role{RoleSupervisor, RoleManager, RoleOwner} {
		capabilities := OpsCapabilitiesForRole(role)
		expected := []Capability{CapabilityOpsRead}
		if len(capabilities) != len(expected) {
			t.Fatalf("%s len(capabilities) = %d, want %d", role, len(capabilities), len(expected))
		}
		for index, capability := range expected {
			if capabilities[index] != capability {
				t.Fatalf("%s capabilities[%d] = %q, want %q", role, index, capabilities[index], capability)
			}
		}
	}

	if capabilities := OpsCapabilitiesForRole(RoleMember); len(capabilities) != 0 {
		t.Fatalf("member ops capabilities = %v, want none", capabilities)
	}
}

func TestBookingCapabilitiesForRoleRemainDeterministicAndMemberDenied(t *testing.T) {
	expectedByRole := map[Role][]Capability{
		RoleSupervisor: {CapabilityBookingRead},
		RoleManager:    {CapabilityBookingManage, CapabilityBookingRead},
		RoleOwner:      {CapabilityBookingManage, CapabilityBookingRead},
	}
	for role, expected := range expectedByRole {
		capabilities := BookingCapabilitiesForRole(role)
		if len(capabilities) != len(expected) {
			t.Fatalf("%s len(capabilities) = %d, want %d", role, len(capabilities), len(expected))
		}
		for index, capability := range expected {
			if capabilities[index] != capability {
				t.Fatalf("%s capabilities[%d] = %q, want %q", role, index, capabilities[index], capability)
			}
		}
	}

	if capabilities := BookingCapabilitiesForRole(RoleMember); len(capabilities) != 0 {
		t.Fatalf("member booking capabilities = %v, want none", capabilities)
	}
}

func TestTrustedSurfaceVerifierRejectsMissingAndInvalidTokens(t *testing.T) {
	verifier := NewTrustedSurfaceVerifier("staff-console=staff-secret")

	missingTokenRequest := httptest.NewRequest("POST", "/api/v1/competition/sessions", nil)
	missingTokenRequest.Header.Set(TrustedSurfaceHeader, "staff-console")
	if _, err := verifier.VerifyRequest(missingTokenRequest); err != ErrTrustedSurfaceMissing {
		t.Fatalf("VerifyRequest(missing token) error = %v, want %v", err, ErrTrustedSurfaceMissing)
	}

	invalidTokenRequest := httptest.NewRequest("POST", "/api/v1/competition/sessions", nil)
	invalidTokenRequest.Header.Set(TrustedSurfaceHeader, "staff-console")
	invalidTokenRequest.Header.Set(TrustedSurfaceTokenHeader, "wrong-secret")
	if _, err := verifier.VerifyRequest(invalidTokenRequest); err != ErrTrustedSurfaceInvalid {
		t.Fatalf("VerifyRequest(invalid token) error = %v, want %v", err, ErrTrustedSurfaceInvalid)
	}

	validRequest := httptest.NewRequest("POST", "/api/v1/competition/sessions", nil)
	validRequest.Header.Set(TrustedSurfaceHeader, "staff-console")
	validRequest.Header.Set(TrustedSurfaceTokenHeader, "staff-secret")
	surface, err := verifier.VerifyRequest(validRequest)
	if err != nil {
		t.Fatalf("VerifyRequest(valid) error = %v", err)
	}
	if surface.Key != "staff-console" {
		t.Fatalf("surface.Key = %q, want %q", surface.Key, "staff-console")
	}
}
