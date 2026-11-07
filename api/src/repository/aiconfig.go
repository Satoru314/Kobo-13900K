package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

// AIProvider is one runner kobo can start, from the providers section of ai.json.
type AIProvider struct {
	Name   string
	Runner string
	Bin    string
	Agent  string
}

// AICatalog is ai.json loaded and checked. It implements usecase.Catalog.
type AICatalog struct {
	providers []AIProvider
	models    []usecase.AIModel
	byID      map[string]usecase.AIModel
}

type aiFile struct {
	Providers map[string]struct {
		Runner string `json:"runner"`
		Bin    string `json:"bin"`
		Agent  string `json:"agent"`
	} `json:"providers"`
	Models []struct {
		ID       string              `json:"id"`
		Label    string              `json:"label"`
		Provider string              `json:"provider"`
		Model    string              `json:"model"`
		OffPeak  map[string][]string `json:"off_peak"`
	} `json:"models"`
}

// LoadAICatalog reads and validates the AI catalog. Adding a model means adding
// an entry here; adding a kind of CLI means adding a runner in cmd/kobo.
func LoadAICatalog(path string) (*AICatalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read AI catalog: %w", err)
	}
	var f aiFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("read AI catalog %s: %w", path, err)
	}
	if len(f.Providers) == 0 {
		return nil, fmt.Errorf("AI catalog %s: providers is empty", path)
	}
	c := &AICatalog{byID: map[string]usecase.AIModel{}}
	for name, p := range f.Providers {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(p.Runner) == "" || strings.TrimSpace(p.Bin) == "" {
			return nil, fmt.Errorf("AI catalog %s: provider %q needs runner and bin", path, name)
		}
		c.providers = append(c.providers, AIProvider{Name: name, Runner: p.Runner, Bin: p.Bin, Agent: p.Agent})
	}
	sort.Slice(c.providers, func(i, j int) bool { return c.providers[i].Name < c.providers[j].Name })
	for _, m := range f.Models {
		if m.ID == "" || m.Label == "" || m.Provider == "" {
			return nil, fmt.Errorf("AI catalog %s: every model needs id, label and provider", path)
		}
		if _, ok := f.Providers[m.Provider]; !ok {
			return nil, fmt.Errorf("AI catalog %s: model %q uses unknown provider %q", path, m.ID, m.Provider)
		}
		if _, ok := c.byID[m.ID]; ok {
			return nil, fmt.Errorf("AI catalog %s: model id %q is repeated", path, m.ID)
		}
		p := f.Providers[m.Provider]
		if p.Runner == "opencode" && strings.TrimSpace(m.Model) == "" {
			return nil, fmt.Errorf("AI catalog %s: opencode model %q needs model", path, m.ID)
		}
		offPeak, err := usecase.ParseOffPeakSchedule(m.OffPeak)
		if err != nil {
			return nil, fmt.Errorf("AI catalog %s: model %q: %w", path, m.ID, err)
		}
		model := usecase.AIModel{ID: m.ID, Label: m.Label, Provider: m.Provider, Model: m.Model, OffPeak: offPeak}
		c.models = append(c.models, model)
		c.byID[m.ID] = model
	}
	if len(c.models) == 0 {
		return nil, fmt.Errorf("AI catalog %s: models is empty", path)
	}
	return c, nil
}

func (c *AICatalog) Get(id string) (usecase.AIModel, bool) {
	m, ok := c.byID[id]
	return m, ok
}

func (c *AICatalog) List() []usecase.AIModel { return c.models }

func (c *AICatalog) Providers() []AIProvider { return c.providers }
