package domain

import (
	"slices"
	"time"
)

type EventType string

const (
	EventTaskCreated    EventType = "task_created"
	EventStatusChanged  EventType = "status_changed" // payload: from, to, reason
	EventSessionStarted EventType = "session_started"
	EventSessionResumed EventType = "session_resumed"
	EventAIInput        EventType = "ai_input"  // payload: prompt
	EventAIResult       EventType = "ai_result" // payload: what the AI reported
	EventPRCreated      EventType = "pr_created"
	EventCIResult       EventType = "ci_result" // payload: state, summary
	EventReviewResult   EventType = "review_result"
	EventSystemError    EventType = "system_error" // payload: message
	EventBlocked        EventType = "blocked"      // payload: reason
	EventRetried        EventType = "retried"
)

var eventTypes = []EventType{EventTaskCreated, EventStatusChanged, EventSessionStarted, EventSessionResumed,
	EventAIInput, EventAIResult, EventPRCreated, EventCIResult, EventReviewResult, EventSystemError,
	EventBlocked, EventRetried}

func (t EventType) Valid() bool { return slices.Contains(eventTypes, t) }

// Event is one milestone in a task's run. SessionID is set when it happened inside an AI session.
type Event struct {
	ID        int64
	TaskID    int64
	SessionID *int64
	Type      EventType
	Payload   map[string]any
	CreatedAt time.Time
}
