package authz

import (
	"errors"
	"slices"
)

type Role string

const (
	RoleMember     Role = "member"
	RoleSupervisor Role = "supervisor"
	RoleManager    Role = "manager"
	RoleOwner      Role = "owner"
)

type Capability string

const (
	CapabilityCompetitionRead            Capability = "competition_read"
	CapabilityCompetitionLiveManage      Capability = "competition_live_manage"
	CapabilityCompetitionStructureManage Capability = "competition_structure_manage"
)

var (
	ErrRoleInvalid           = errors.New("role is invalid")
	ErrCapabilityDenied      = errors.New("required capability is missing")
	ErrTrustedSurfaceMissing = errors.New("trusted surface token is required")
	ErrTrustedSurfaceKey     = errors.New("trusted surface key is required")
	ErrTrustedSurfaceInvalid = errors.New("trusted surface token is invalid")
)

var capabilitiesByRole = map[Role][]Capability{
	RoleMember:     {},
	RoleSupervisor: {CapabilityCompetitionRead, CapabilityCompetitionLiveManage},
	RoleManager:    {CapabilityCompetitionRead, CapabilityCompetitionLiveManage, CapabilityCompetitionStructureManage},
	RoleOwner:      {CapabilityCompetitionRead, CapabilityCompetitionLiveManage, CapabilityCompetitionStructureManage},
}

func NormalizeRole(value string) (Role, error) {
	role := Role(value)
	if _, ok := capabilitiesByRole[role]; !ok {
		return "", ErrRoleInvalid
	}

	return role, nil
}

func CapabilitiesForRole(role Role) []Capability {
	capabilities, ok := capabilitiesByRole[role]
	if !ok {
		return nil
	}

	cloned := append([]Capability(nil), capabilities...)
	slices.Sort(cloned)
	return cloned
}

func HasCapability(capabilities []Capability, required Capability) bool {
	return slices.Contains(capabilities, required)
}
