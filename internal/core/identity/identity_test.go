package identity

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDecideAccess(t *testing.T) {
	withEmail := Claims{Subject: "ana-1", Email: "ana@example.com"}
	withoutEmail := Claims{Subject: "ana-1"}
	cases := []struct {
		name string
		in   AccessInput
		want AccessOutcome
	}{
		{"existing person enters", AccessInput{Existing: true, Claims: withoutEmail, RequireInvitation: true}, Enter},
		{"new person gets an organization", AccessInput{Claims: withEmail}, CreateWithOrganization},
		{"new person without email", AccessInput{Claims: withoutEmail}, DenyEmailRequired},
		{"new person without invitation", AccessInput{Claims: withEmail, RequireInvitation: true}, DenyInvitationRequired},
		{"missing email wins over missing invitation", AccessInput{Claims: withoutEmail, RequireInvitation: true}, DenyEmailRequired},
	}
	for _, tc := range cases {
		if got := DecideAccess(tc.in); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestDisplayName(t *testing.T) {
	cases := map[string]Claims{
		"Ana Pérez": {Name: " Ana Pérez ", PreferredUsername: "ana", Email: "ana@example.com"},
		"ana":       {PreferredUsername: "ana", Email: "ana@example.com"},
		"ana.perez": {Email: "ana.perez@example.com"},
		"ana-1":     {Subject: "ana-1"},
	}
	for want, claims := range cases {
		if got := DisplayName(claims); got != want {
			t.Errorf("DisplayName(%+v) = %q, want %q", claims, got, want)
		}
	}
}

func TestActiveOrganization(t *testing.T) {
	first := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	second := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	foreign := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	memberships := []Membership{
		{OrganizationID: second, JoinedAt: day.Add(24 * time.Hour)},
		{OrganizationID: first, JoinedAt: day},
	}

	if got, _ := ActiveOrganization(memberships, nil); got != first {
		t.Errorf("without a remembered one: %s, want the first joined", got)
	}
	if got, _ := ActiveOrganization(memberships, &second); got != second {
		t.Errorf("remembered own organization: %s", got)
	}
	if got, _ := ActiveOrganization(memberships, &foreign); got != first {
		t.Errorf("remembered foreign organization: %s, want the first joined", got)
	}
	if _, ok := ActiveOrganization(nil, &first); ok {
		t.Error("no memberships means no active organization")
	}
}
