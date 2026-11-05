package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCatalog(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ai.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAICatalog(t *testing.T) {
	path := writeCatalog(t, `{
		"providers": {
			"claude": {"runner": "claude", "bin": "claude"},
			"opencode": {"runner": "opencode", "bin": "opencode", "agent": "build"}
		},
		"models": [
			{"id": "claude", "label": "Claude", "provider": "claude"},
			{"id": "deepseek", "label": "DeepSeek", "provider": "opencode", "model": "p/m"}
		]
	}`)
	c, err := LoadAICatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Providers()) != 2 || len(c.List()) != 2 {
		t.Fatalf("catalog = %+v", c)
	}
	m, ok := c.Get("deepseek")
	if !ok || m.Model != "p/m" || m.Provider != "opencode" {
		t.Fatalf("model = %+v, %v", m, ok)
	}
}

func TestLoadAICatalogRejectsBadModels(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"unknown provider", `{"providers":{"c":{"runner":"claude","bin":"claude"}},"models":[{"id":"x","label":"X","provider":"nope"}]}`, "unknown provider"},
		{"duplicate id", `{"providers":{"c":{"runner":"claude","bin":"claude"}},"models":[{"id":"x","label":"X","provider":"c"},{"id":"x","label":"Y","provider":"c"}]}`, "repeated"},
		{"opencode without model", `{"providers":{"o":{"runner":"opencode","bin":"opencode"}},"models":[{"id":"x","label":"X","provider":"o"}]}`, "needs model"},
		{"empty models", `{"providers":{"c":{"runner":"claude","bin":"claude"}},"models":[]}`, "models is empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadAICatalog(writeCatalog(t, tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}
