package usecase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

func (u *Usecase) ListProjects(ctx context.Context) ([]domain.Project, error) {
	projects, err := u.db.ListProjects(ctx)
	if projects == nil {
		projects = []domain.Project{}
	}
	return projects, err
}

// ListProjectCandidates lists every directory under the projects directory, in name order.
func (u *Usecase) ListProjectCandidates(ctx context.Context) ([]ProjectCandidate, error) {
	entries, err := os.ReadDir(u.settings.ProjectsDir)
	if err != nil {
		return nil, err
	}
	projects, err := u.db.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	registered := map[string]bool{}
	for _, p := range projects {
		registered[p.Dir] = true
	}
	out := []ProjectCandidate{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		c := ProjectCandidate{Dir: e.Name(), Registered: registered[e.Name()]}
		if !c.Registered {
			if c.Problem, err = u.candidateProblem(ctx, filepath.Join(u.settings.ProjectsDir, e.Name())); err != nil {
				return nil, err
			}
		}
		out = append(out, c)
	}
	return out, nil
}

func (u *Usecase) candidateProblem(ctx context.Context, path string) (CandidateProblem, error) {
	if _, err := os.Stat(filepath.Join(path, ".git")); errors.Is(err, os.ErrNotExist) {
		return ProblemNotGit, nil
	} else if err != nil {
		return "", err
	}
	_, _, ok, err := u.repo.GitHubOrigin(ctx, path)
	if err != nil {
		return "", err
	}
	if !ok {
		return ProblemNotGitHub, nil
	}
	return "", nil
}

// AddProject returns the project for a directory under the projects directory,
// registering it from its git remote the first time.
func (u *Usecase) AddProject(ctx context.Context, dir string, now time.Time) (domain.Project, error) {
	if dir == "" || strings.ContainsAny(dir, `/\`) || dir == "." || dir == ".." {
		return domain.Project{}, fmt.Errorf("%w: project must be a directory name under %s", ErrInvalidInput, u.settings.ProjectsDir)
	}
	p, err := u.db.GetProjectByDir(ctx, dir)
	if err == nil || !errors.Is(err, domain.ErrNotFound) {
		return p, err
	}
	path := filepath.Join(u.settings.ProjectsDir, dir)
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return domain.Project{}, fmt.Errorf("%w: %s is not a directory", ErrInvalidInput, path)
	}
	remote, err := u.repo.Remote(ctx, path)
	if err != nil {
		return domain.Project{}, fmt.Errorf("%w: read git remote of %s: %v", ErrInvalidInput, path, err)
	}
	p = domain.Project{Dir: dir, Owner: remote.Owner, Name: remote.Name, DefaultBranch: remote.DefaultBranch, CreatedAt: now}
	if err := u.db.CreateProject(ctx, &p); err != nil {
		return domain.Project{}, err
	}
	return p, nil
}

// projectDir is the project's checkout on disk.
func (u *Usecase) projectDir(p domain.Project) string {
	return filepath.Join(u.settings.ProjectsDir, p.Dir)
}
