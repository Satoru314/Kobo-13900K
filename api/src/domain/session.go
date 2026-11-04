package domain

import "time"

type Role string

const (
	RoleDev    Role = "dev"
	RoleReview Role = "review"
)

// Session is one AI conversation. A task has at most one per role, resumed on later runs.
type Session struct {
	ID                int64     `json:"id"`
	TaskID            int64     `json:"task_id"`
	Role              Role      `json:"role"`
	Provider          string    `json:"provider"`
	ExternalSessionID string    `json:"external_session_id"`
	CreatedAt         time.Time `json:"created_at"`
}
