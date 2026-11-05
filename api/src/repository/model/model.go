// Package model is the rows of kobo's tables as GORM reads and writes them.
// Table names are the singular type names (task, pull_request, ...); the SQL
// migrations, not these types, define the tables.
package model

import "time"

type Project struct {
	ID            int64     `gorm:"column:id;primaryKey"`
	Dir           string    `gorm:"column:dir"`
	Owner         string    `gorm:"column:owner"`
	Name          string    `gorm:"column:name"`
	DefaultBranch string    `gorm:"column:default_branch"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime:false"`
}

type Task struct {
	ID          int64      `gorm:"column:id;primaryKey"`
	ProjectID   int64      `gorm:"column:project_id"`
	Title       string     `gorm:"column:title"`
	Description string     `gorm:"column:description"`
	Status      string     `gorm:"column:status"`
	Mode        string     `gorm:"column:mode"`
	EnqueuedAt  *time.Time `gorm:"column:enqueued_at"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime:false"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime:false"`
}

type PullRequest struct {
	ID        int64      `gorm:"column:id;primaryKey"`
	TaskID    int64      `gorm:"column:task_id"`
	Number    int        `gorm:"column:number"`
	URL       string     `gorm:"column:url"`
	Branch    string     `gorm:"column:branch"`
	State     string     `gorm:"column:state"`
	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime:false"`
	MergedAt  *time.Time `gorm:"column:merged_at"`
}

type Session struct {
	ID                int64     `gorm:"column:id;primaryKey"`
	TaskID            int64     `gorm:"column:task_id"`
	Role              string    `gorm:"column:role"`
	Provider          string    `gorm:"column:provider"`
	ExternalSessionID string    `gorm:"column:external_session_id"`
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime:false"`
}

type ReviewRound struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	TaskID    int64     `gorm:"column:task_id"`
	RoundNo   int       `gorm:"column:round_no"`
	Result    string    `gorm:"column:result"`
	Comments  string    `gorm:"column:comments"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime:false"`
}

type Event struct {
	ID        int64          `gorm:"column:id;primaryKey"`
	TaskID    int64          `gorm:"column:task_id"`
	SessionID *int64         `gorm:"column:session_id"`
	Type      string         `gorm:"column:type"`
	Payload   map[string]any `gorm:"column:payload;serializer:json"` // jsonb
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime:false"`
}

type QueueState struct {
	ID     int  `gorm:"column:id;primaryKey"`
	Paused bool `gorm:"column:paused"`
}
