// Package identity holds the pure decisions about who enters and in which
// organization they work.
package identity

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Claims struct {
	Subject           string
	Email             string
	Name              string
	PreferredUsername string
}

type AccessInput struct {
	Existing          bool
	Claims            Claims
	RequireInvitation bool
	HasInvitation     bool
}

type AccessOutcome int

const (
	Enter AccessOutcome = iota
	CreateWithOrganization
	DenyEmailRequired
	DenyInvitationRequired
)

func (o AccessOutcome) String() string {
	switch o {
	case Enter:
		return "enter"
	case CreateWithOrganization:
		return "create_with_organization"
	case DenyEmailRequired:
		return "email_required"
	case DenyInvitationRequired:
		return "invitation_required"
	}
	return "unknown"
}

// DecideAccess applies F1: people who already exist always enter; newcomers
// need an email and, if the instance demands it, an invitation.
func DecideAccess(in AccessInput) AccessOutcome {
	switch {
	case in.Existing:
		return Enter
	case strings.TrimSpace(in.Claims.Email) == "":
		return DenyEmailRequired
	case in.RequireInvitation && !in.HasInvitation:
		return DenyInvitationRequired
	default:
		return CreateWithOrganization
	}
}

func DisplayName(c Claims) string {
	for _, candidate := range []string{c.Name, c.PreferredUsername} {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	if local, _, found := strings.Cut(strings.TrimSpace(c.Email), "@"); found && local != "" {
		return local
	}
	return c.Subject
}

type Membership struct {
	OrganizationID uuid.UUID
	JoinedAt       time.Time
}

// ActiveOrganization returns the remembered organization while the person is
// still a member of it and, otherwise, the first one they joined.
func ActiveOrganization(memberships []Membership, remembered *uuid.UUID) (uuid.UUID, bool) {
	if len(memberships) == 0 {
		return uuid.Nil, false
	}
	if remembered != nil {
		for _, m := range memberships {
			if m.OrganizationID == *remembered {
				return m.OrganizationID, true
			}
		}
	}
	ordered := append([]Membership(nil), memberships...)
	sort.Slice(ordered, func(i, j int) bool {
		if !ordered[i].JoinedAt.Equal(ordered[j].JoinedAt) {
			return ordered[i].JoinedAt.Before(ordered[j].JoinedAt)
		}
		return ordered[i].OrganizationID.String() < ordered[j].OrganizationID.String()
	})
	return ordered[0].OrganizationID, true
}
