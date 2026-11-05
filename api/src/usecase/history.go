package usecase

import "github.com/Satoru314/Kobo-13900K/api/src/domain"

// history answers questions from a task's events, oldest first.
type history []domain.Event

// statusBeforeError is the waiting status of the stage the task last failed in.
func (h history) statusBeforeError() (domain.Status, bool) {
	for i := len(h) - 1; i >= 0; i-- {
		e := h[i]
		if e.Type == domain.EventStatusChanged && e.Payload["to"] == string(domain.StatusError) {
			from, _ := e.Payload["from"].(string)
			return domain.Status(from).WaitingStatus(), from != ""
		}
	}
	return "", false
}

// lastCIState is the state of the latest CI result, empty when CI never reported.
func (h history) lastCIState() string {
	for i := len(h) - 1; i >= 0; i-- {
		if h[i].Type == domain.EventCIResult {
			state, _ := h[i].Payload["state"].(string)
			return state
		}
	}
	return ""
}

// failuresInARow counts system errors since the task last made progress.
func (h history) failuresInARow() int {
	n := 0
	for i := len(h) - 1; i >= 0; i-- {
		switch h[i].Type {
		case domain.EventSystemError:
			n++
		case domain.EventTaskCreated, domain.EventAIResult, domain.EventCIResult,
			domain.EventReviewResult, domain.EventRetried, domain.EventBlocked:
			return n
		}
	}
	return n
}

// lastError is the message of the latest system error or blocked report.
func (h history) lastError() string {
	for i := len(h) - 1; i >= 0; i-- {
		switch h[i].Type {
		case domain.EventSystemError:
			msg, _ := h[i].Payload["message"].(string)
			return msg
		case domain.EventBlocked:
			reason, _ := h[i].Payload["reason"].(string)
			return "blocked: " + reason
		}
	}
	return ""
}
