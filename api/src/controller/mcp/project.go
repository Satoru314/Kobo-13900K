package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/iusecase"
	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type ProjectTools struct {
	projects iusecase.ProjectUsecase
}

func NewProjectTools(projects iusecase.ProjectUsecase) *ProjectTools {
	return &ProjectTools{projects: projects}
}

type NoInput struct{}

type ProjectList struct {
	Projects []domain.Project `json:"projects"`
}

func (t *ProjectTools) List(ctx context.Context, _ *mcp.CallToolRequest, _ NoInput) (*mcp.CallToolResult, ProjectList, error) {
	projects, err := t.projects.List(ctx)
	if projects == nil {
		projects = []domain.Project{}
	}
	return nil, ProjectList{Projects: projects}, err
}
