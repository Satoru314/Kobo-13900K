package domain

import "time"

type EventType string

const (
	EventTaskCreated    EventType = "task_created"
	EventStatusChanged  EventType = "status_changed" // payload: from, to
	EventSessionStarted EventType = "session_started"
	EventSessionResumed EventType = "session_resumed"
	EventAIInput        EventType = "ai_input"  // payload: prompt
	EventAIResult       EventType = "ai_result" // payload: what the AI reported
	EventPRCreated      EventType = "pr_created"
	EventCIResult       EventType = "ci_result"
	EventReviewResult   EventType = "review_result"
	EventSystemError    EventType = "system_error" // payload: message
	EventBlocked        EventType = "blocked"      // payload: reason
	EventRetried        EventType = "retried"
)

// Event is one milestone in a task's run. SessionID is set when it happened inside an AI session.
type Event struct {
	ID        int64          `json:"id"`
	TaskID    int64          `json:"task_id"`
	SessionID *int64         `json:"session_id"`
	Type      EventType      `json:"type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}
