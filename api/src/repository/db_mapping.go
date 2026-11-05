package repository

import (
	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/repository/model"
)

// Conversions between rows (model) and domain types.

func projectToModel(p domain.Project) model.Project {
	return model.Project{ID: p.ID, Dir: p.Dir, Owner: p.Owner, Name: p.Name, DefaultBranch: p.DefaultBranch, CreatedAt: p.CreatedAt}
}

func projectFromModel(m model.Project) domain.Project {
	return domain.Project{ID: m.ID, Dir: m.Dir, Owner: m.Owner, Name: m.Name, DefaultBranch: m.DefaultBranch, CreatedAt: m.CreatedAt}
}

func taskToModel(t domain.Task) model.Task {
	return model.Task{ID: t.ID, ProjectID: t.ProjectID, Title: t.Title, Description: t.Description,
		Status: string(t.Status), Mode: string(t.Mode), EnqueuedAt: t.EnqueuedAt, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

func taskFromModel(m model.Task) domain.Task {
	return domain.Task{ID: m.ID, ProjectID: m.ProjectID, Title: m.Title, Description: m.Description,
		Status: domain.Status(m.Status), Mode: domain.Mode(m.Mode), EnqueuedAt: m.EnqueuedAt, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}

func pullRequestToModel(p domain.PullRequest) model.PullRequest {
	return model.PullRequest{ID: p.ID, TaskID: p.TaskID, Number: p.Number, URL: p.URL, Branch: p.Branch,
		State: string(p.State), CreatedAt: p.CreatedAt, MergedAt: p.MergedAt}
}

func pullRequestFromModel(m model.PullRequest) domain.PullRequest {
	return domain.PullRequest{ID: m.ID, TaskID: m.TaskID, Number: m.Number, URL: m.URL, Branch: m.Branch,
		State: domain.PRState(m.State), CreatedAt: m.CreatedAt, MergedAt: m.MergedAt}
}

func sessionToModel(s domain.Session) model.Session {
	return model.Session{ID: s.ID, TaskID: s.TaskID, Role: string(s.Role), Provider: s.Provider,
		AI: s.AI, Model: s.Model, ExternalSessionID: s.ExternalSessionID, CreatedAt: s.CreatedAt}
}

func sessionFromModel(m model.Session) domain.Session {
	return domain.Session{ID: m.ID, TaskID: m.TaskID, Role: domain.Role(m.Role), Provider: m.Provider,
		AI: m.AI, Model: m.Model, ExternalSessionID: m.ExternalSessionID, CreatedAt: m.CreatedAt}
}

func aiSettingToModel(s domain.AISetting) model.AISetting {
	return model.AISetting{ID: 1, DevAI: s.DevAI, ReviewAI: s.ReviewAI, SeparateReview: s.SeparateReview, UpdatedAt: s.UpdatedAt}
}

func aiSettingFromModel(m model.AISetting) domain.AISetting {
	return domain.AISetting{DevAI: m.DevAI, ReviewAI: m.ReviewAI, SeparateReview: m.SeparateReview, UpdatedAt: m.UpdatedAt}
}

func reviewRoundToModel(r domain.ReviewRound) model.ReviewRound {
	return model.ReviewRound{ID: r.ID, TaskID: r.TaskID, RoundNo: r.RoundNo, Result: string(r.Result),
		Comments: r.Comments, CreatedAt: r.CreatedAt}
}

func reviewRoundFromModel(m model.ReviewRound) domain.ReviewRound {
	return domain.ReviewRound{ID: m.ID, TaskID: m.TaskID, RoundNo: m.RoundNo, Result: domain.ReviewResult(m.Result),
		Comments: m.Comments, CreatedAt: m.CreatedAt}
}

func eventToModel(e domain.Event) model.Event {
	return model.Event{ID: e.ID, TaskID: e.TaskID, SessionID: e.SessionID, Type: string(e.Type), Payload: e.Payload, CreatedAt: e.CreatedAt}
}

func eventFromModel(m model.Event) domain.Event {
	return domain.Event{ID: m.ID, TaskID: m.TaskID, SessionID: m.SessionID, Type: domain.EventType(m.Type), Payload: m.Payload, CreatedAt: m.CreatedAt}
}

// mapAll converts a list of rows.
func mapAll[M, D any](rows []M, f func(M) D) []D {
	out := make([]D, len(rows))
	for i, r := range rows {
		out[i] = f(r)
	}
	return out
}
