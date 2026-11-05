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
	ID                int64
	TaskID            int64
	Role              Role
	Provider          string
	ExternalSessionID string
	CreatedAt         time.Time
}
