package domain

import "time"

type Role string

const (
	RoleDev    Role = "dev"
	RoleReview Role = "review"
)

func (r Role) Valid() bool { return r == RoleDev || r == RoleReview }

// Session is one AI conversation. A task has at most one per role, resumed on later runs.
type Session struct {
	ID       int64
	TaskID   int64
	Role     Role
	Provider string
	// AI is the catalog id chosen for this session. Model is the model string
	// handed to the provider. Both are kept so resuming never changes the AI.
	AI                string
	Model             string
	ExternalSessionID string
	CreatedAt         time.Time
}
