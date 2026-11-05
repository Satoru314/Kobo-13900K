package main

import (
	"strings"
	"testing"
)

func TestLoadConfigShotsRepo(t *testing.T) {
	t.Setenv("KOBO_DATABASE_URL", "postgres://example")
	t.Setenv("KOBO_AI_CONFIG", "ai.json")
	for _, c := range []struct{ value, wantErr string }{
		{"", "KOBO_SHOTS_REPO is required"},
		{"kobo-shots", "not owner/name"},
		{"a/b/c", "not owner/name"},
		{"Satoru314/kobo-shots", ""},
	} {
		t.Setenv("KOBO_SHOTS_REPO", c.value)
		cfg, err := LoadConfig()
		if c.wantErr == "" {
			if err != nil || cfg.Settings.ShotsRepo != c.value {
				t.Errorf("%q: got %q, %v", c.value, cfg.Settings.ShotsRepo, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("%q: err = %v, want %q", c.value, err, c.wantErr)
		}
	}
}
