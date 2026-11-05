package e2e

import (
	"testing"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/schema"
	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

// The settings screen picks one AI normally, or a separate one for review. The
// AI chosen when a session starts stays with that session even after a change.
func TestAISettingSeparatesReviewAndKeepsSessionModel(t *testing.T) {
	h := newHarness(t)

	var view schema.AIView
	if code := h.rest("GET", "/ai", nil, &view); code != 200 {
		t.Fatalf("GET /ai = %d", code)
	}
	if len(view.Models) != 2 || view.Setting.DevAI != "claude" || view.Setting.ReviewAI != "claude" || view.Setting.SeparateReview {
		t.Fatalf("default AI view = %+v", view)
	}

	h.setAI("deepseek-v4.1-flash", "claude", true)
	task := h.createTask("split AI", "")
	h.runner.script(devDone(1), requestChanges("fix it"), devDone(1), approve())

	dev := h.runNext()
	if dev.AI != "deepseek-v4.1-flash" || dev.Model != "opencode-go/deepseek-v4.1-flash" {
		t.Fatalf("dev run = %+v", dev)
	}
	h.wantStatus(task.ID, domain.StatusReview)

	// The review gets its own model and starts its own session.
	review := h.runNext()
	if review.AI != "claude" || review.Model != "" || review.Role != domain.RoleReview {
		t.Fatalf("review run = %+v", review)
	}
	h.wantStatus(task.ID, domain.StatusFix)

	// Changing development to Claude must not change the existing dev session.
	h.setAI("claude", "claude", true)
	fix := h.runNext()
	if fix.AI != "deepseek-v4.1-flash" || fix.Model != "opencode-go/deepseek-v4.1-flash" || !fix.Resume {
		t.Fatalf("fix run = %+v", fix)
	}
	h.wantStatus(task.ID, domain.StatusReview)

	// The review session keeps the model it started with as well.
	review = h.runNext()
	if review.AI != "claude" || !review.Resume {
		t.Fatalf("second review = %+v", review)
	}
	h.wantStatus(task.ID, domain.StatusDone)

	d := h.detail(task.ID)
	if len(d.Sessions) != 2 {
		t.Fatalf("sessions = %+v", d.Sessions)
	}
	for _, s := range d.Sessions {
		switch s.Role {
		case "dev":
			if s.AI != "deepseek-v4.1-flash" || s.AILabel != "DeepSeek V4.1 Flash" || s.Model != "opencode-go/deepseek-v4.1-flash" {
				t.Errorf("dev session = %+v", s)
			}
		case "review":
			if s.AI != "claude" || s.AILabel != "Claude" || s.Model != "" {
				t.Errorf("review session = %+v", s)
			}
		}
	}

	if code := h.rest("PUT", "/ai/setting", map[string]any{"dev_ai": "missing", "review_ai": "claude"}, nil); code != 400 {
		t.Fatalf("unknown model = %d, want 400", code)
	}
}

// Without a separate review model, review follows development.
func TestAISettingUsesDevForReviewByDefault(t *testing.T) {
	h := newHarness(t)
	h.setAI("deepseek-v4.1-flash", "claude", false)
	task := h.createTask("one AI", "")
	h.runner.script(devDone(2), approve())

	dev := h.runNext()
	if dev.AI != "deepseek-v4.1-flash" {
		t.Fatalf("dev run = %+v", dev)
	}
	h.wantStatus(task.ID, domain.StatusReview)
	review := h.runNext()
	if review.AI != "deepseek-v4.1-flash" || review.Role != domain.RoleReview {
		t.Fatalf("review run = %+v", review)
	}
	h.wantStatus(task.ID, domain.StatusDone)
}
