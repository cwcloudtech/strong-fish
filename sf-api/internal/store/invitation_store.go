package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/utils"
)

type InvitationStore struct {
	pool *pgxpool.Pool
}

func NewInvitationStore(pool *pgxpool.Pool) *InvitationStore {
	return &InvitationStore{pool: pool}
}

// invitationData is the JSONB payload of the club_invitations table.
type invitationData struct {
	Role    string `json:"role"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

const invitationSelect = `
	SELECT i.id, i.club_id, i.inviter_id, i.email, i.data, i.created_at, i.updated_at,
	       coalesce(c.data->>'name', ''),
	       trim(coalesce(u.data->>'name', '') || ' ' || coalesce(u.data->>'surname', '')),
	       (SELECT count(*) FROM club_members WHERE club_id = i.club_id)
	FROM club_invitations i
	JOIN clubs c ON c.id = i.club_id
	JOIN users u ON u.id = i.inviter_id`

func scanInvitation(row pgx.Row) (models.Invitation, error) {
	var invitation models.Invitation
	var raw []byte
	if err := row.Scan(&invitation.ID, &invitation.ClubID, &invitation.InviterID, &invitation.Email,
		&raw, &invitation.CreatedAt, &invitation.UpdatedAt,
		&invitation.ClubName, &invitation.InviterName, &invitation.MemberCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Invitation{}, ErrNotFound
		}
		return models.Invitation{}, err
	}

	var d invitationData
	if err := json.Unmarshal(raw, &d); err != nil {
		return models.Invitation{}, err
	}
	invitation.Role = models.Role(d.Role)
	if !models.IsValidRole(invitation.Role) {
		invitation.Role = models.RoleMember
	}
	invitation.Status = d.Status
	invitation.Message = d.Message
	return invitation, nil
}

func scanInvitations(rows pgx.Rows) ([]models.Invitation, error) {
	defer rows.Close()
	invitations := []models.Invitation{}
	for rows.Next() {
		invitation, err := scanInvitation(rows)
		if err != nil {
			return nil, err
		}
		invitations = append(invitations, invitation)
	}
	return invitations, rows.Err()
}

// Invite creates or refreshes an invitation.
//
// Re-inviting the same address is an update rather than a second row: the
// partial unique index only covers pending invitations, so a fresh invitation
// can still follow one that was declined, while nobody ever has to decline the
// same club twice.
func (s *InvitationStore) Invite(ctx context.Context, clubID, inviterID, email string, role models.Role, message string) (models.Invitation, error) {
	data, err := json.Marshal(invitationData{
		Role: string(role), Status: models.InvitationStatusPending, Message: message,
	})
	if err != nil {
		return models.Invitation{}, err
	}

	var id string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO club_invitations (club_id, inviter_id, email, data)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (club_id, lower(email)) WHERE data->>'status' = 'pending'
		DO UPDATE SET data = club_invitations.data || $4::jsonb, inviter_id = $2, updated_at = now()
		RETURNING id
	`, clubID, inviterID, strings.ToLower(strings.TrimSpace(email)), data).Scan(&id)
	if err != nil {
		return models.Invitation{}, err
	}
	return s.FindByID(ctx, id)
}

func (s *InvitationStore) FindByID(ctx context.Context, id string) (models.Invitation, error) {
	return scanInvitation(s.pool.QueryRow(ctx, invitationSelect+` WHERE i.id = $1`, id))
}

// ListForClub returns a club's invitations, most recent first, for the coach
// who sent them.
func (s *InvitationStore) ListForClub(ctx context.Context, clubID string) ([]models.Invitation, error) {
	rows, err := s.pool.Query(ctx, invitationSelect+`
		WHERE i.club_id = $1 ORDER BY i.created_at DESC`, clubID)
	if err != nil {
		return nil, err
	}
	return scanInvitations(rows)
}

// ListPendingForEmail returns the invitations waiting for one address.
//
// Matching on the address rather than on a user id is what lets somebody
// register after being invited and still find the invitation - it was addressed
// to them before they existed here.
func (s *InvitationStore) ListPendingForEmail(ctx context.Context, email string) ([]models.Invitation, error) {
	rows, err := s.pool.Query(ctx, invitationSelect+`
		WHERE lower(i.email) = $1 AND i.data->>'status' = $2
		ORDER BY i.created_at DESC`, strings.ToLower(strings.TrimSpace(email)), models.InvitationStatusPending)
	if err != nil {
		return nil, err
	}
	return scanInvitations(rows)
}

// CountPendingForEmail backs the badge on the invitations nav entry.
func (s *InvitationStore) CountPendingForEmail(ctx context.Context, email string) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM club_invitations
		WHERE lower(email) = $1 AND data->>'status' = $2
	`, strings.ToLower(strings.TrimSpace(email)), models.InvitationStatusPending).Scan(&count)
	return count, err
}

// SetStatus records a decision. It is scoped by the current status so a
// double-click, or a stale tab, can't accept an invitation that was already
// withdrawn.
func (s *InvitationStore) SetStatus(ctx context.Context, id, status string) (models.Invitation, error) {
	patch, err := json.Marshal(map[string]any{"status": status})
	if err != nil {
		return models.Invitation{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE club_invitations SET data = data || $2::jsonb, updated_at = now()
		WHERE id = $1 AND data->>'status' = $3
	`, id, patch, models.InvitationStatusPending)
	if err != nil {
		return models.Invitation{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.Invitation{}, ErrNotFound
	}
	return s.FindByID(ctx, id)
}

// Delete withdraws an invitation, for the coach who sent it.
func (s *InvitationStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM club_invitations WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Accept turns an invitation into a membership in one transaction: an accepted
// invitation whose membership failed to write would leave somebody who thinks
// they joined a club they are not in.
func (s *InvitationStore) Accept(ctx context.Context, invitation models.Invitation, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	patch, err := json.Marshal(map[string]any{"status": models.InvitationStatusAccepted})
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE club_invitations SET data = data || $2::jsonb, updated_at = now()
		WHERE id = $1 AND data->>'status' = $3
	`, invitation.ID, patch, models.InvitationStatusPending)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	// Already a member - invited twice, or joined by another route in the
	// meantime. The invitation is still marked accepted; the membership just
	// keeps whatever role it already had.
	if _, err := tx.Exec(ctx, `
		INSERT INTO club_members (club_id, user_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (club_id, user_id) DO NOTHING
	`, invitation.ClubID, userID, string(invitation.Role)); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// --- coach requests ---

// RequestCoach records an account's claim to be a coach, made at signup.
func (s *UserStore) RequestCoach(ctx context.Context, id string, requestedAt time.Time) (models.User, error) {
	return s.merge(ctx, id, map[string]any{
		"coachRequest": models.CoachRequest{Status: models.CoachRequestPending, RequestedAt: &requestedAt},
	})
}

// DecideCoachRequest records a superadmin's decision. Approving is what
// actually grants the role - asking never did - and rejecting keeps the motive,
// which is written to be shown to the applicant.
func (s *UserStore) DecideCoachRequest(ctx context.Context, id, status, motive, decidedBy string, decidedAt time.Time) (models.User, error) {
	user, err := s.FindByID(ctx, id)
	if err != nil {
		return models.User{}, err
	}

	patch := map[string]any{
		"coachRequest": models.CoachRequest{
			Status: status, Motive: motive,
			RequestedAt: user.CoachRequest.RequestedAt,
			DecidedAt:   &decidedAt, DecidedBy: decidedBy,
		},
	}
	if status == models.CoachRequestApproved {
		// Only a confirmed athlete is promoted: an account still waiting on its
		// activation link keeps waiting, and a banned one stays banned.
		if user.Role == models.GlobalRoleConfirmed {
			patch["role"] = string(models.GlobalRoleCoach)
		}
	}
	return s.merge(ctx, id, patch)
}

// ListCoachApplicants returns the accounts waiting on a decision, oldest first -
// a queue, so nobody is left at the bottom of it.
func (s *UserStore) ListCoachApplicants(ctx context.Context) ([]models.User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+userColumns+` FROM users
		WHERE data->'coachRequest'->>'status' = $1
		ORDER BY created_at
	`, models.CoachRequestPending)
	if err != nil {
		return nil, err
	}
	return scanUsers(rows)
}

// CountCoachApplicants backs the superadmin's pending-work badge.
func (s *UserStore) CountCoachApplicants(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM users WHERE data->'coachRequest'->>'status' = $1
	`, models.CoachRequestPending).Scan(&count)
	return count, err
}

// ListSuperadminEmails is who a coach request is announced to.
func (s *UserStore) ListSuperadminEmails(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT email FROM users WHERE data->>'role' = $1
	`, string(models.GlobalRoleSuperadmin))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	emails := []string{}
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		if utils.IsNotBlank(email) {
			emails = append(emails, email)
		}
	}
	return emails, rows.Err()
}
