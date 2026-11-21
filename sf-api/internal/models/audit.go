package models

import "time"

// AuditEntry is one row of the administrative audit trail as the superadmin
// console reads it. UserEmail is resolved at read time - the live account's
// address, falling back to the snapshot copied into the row when the event was
// recorded, so an event by a since-deleted account still names who did it.
// Detail is the event's own payload (an AI call's provider/model/tokens, a
// ban's reason, a role change's before/after).
type AuditEntry struct {
	Id          string         `json:"id"`
	UserId      string         `json:"userId,omitempty"`
	UserEmail   string         `json:"userEmail,omitempty"`
	Action      string         `json:"action"`
	SubjectType string         `json:"subjectType,omitempty"`
	SubjectId   string         `json:"subjectId,omitempty"`
	Detail      map[string]any `json:"detail,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
}

// Audit actions: the verb of an event, dotted namespace.verb. These are the
// stable identifiers written into audit_log.action and offered as the UI's
// action filter - the screen translates them, so they are not display text.
const (
	AuditActionSignedIn    = "auth.signedIn"
	AuditActionRegistered  = "account.registered"
	AuditActionAICall      = "ai.call"
	AuditActionRoleChanged = "user.roleChanged"
	AuditActionUserDeleted = "user.deleted"
	AuditActionIPBanned    = "ip.banned"
	AuditActionIPUnbanned  = "ip.unbanned"
)

// Audit subject types: what an event was about, so the trail can be filtered to
// one kind of thing. subject_id then carries which one (a user id, an AI
// connection id, an address).
const (
	AuditSubjectUser         = "user"
	AuditSubjectAIConnection = "aiConnection"
	AuditSubjectIP           = "ip"
)

// AuditActions and AuditSubjectTypes are the console's filter dropdowns, in the
// order they are shown. Kept here, beside the constants, so the one list cannot
// drift from the values actually written.
var (
	AuditActions = []string{
		AuditActionSignedIn,
		AuditActionRegistered,
		AuditActionAICall,
		AuditActionRoleChanged,
		AuditActionUserDeleted,
		AuditActionIPBanned,
		AuditActionIPUnbanned,
	}
	AuditSubjectTypes = []string{
		AuditSubjectUser,
		AuditSubjectAIConnection,
		AuditSubjectIP,
	}
)
