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
	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

var ErrInvalidInput = errors.New("invalid input")

type ProjectUsecase struct {
	projects irepository.ProjectRepository
	git      irepository.Git
	settings Settings
}

func NewProjectUsecase(projects irepository.ProjectRepository, git irepository.Git, settings Settings) *ProjectUsecase {
	return &ProjectUsecase{projects: projects, git: git, settings: settings}
}

func (u *ProjectUsecase) List(ctx context.Context) ([]domain.Project, error) {
	return u.projects.List(ctx)
}

func (u *ProjectUsecase) Get(ctx context.Context, id int64) (domain.Project, error) {
	return u.projects.Get(ctx, id)
}

type CandidateProblem string

const (
	ProblemNotGit    CandidateProblem = "not_git"
	ProblemNotGitHub CandidateProblem = "not_github"
)

// ProjectCandidate is a directory under the projects directory that can be registered as a project.
type ProjectCandidate struct {
	Dir        string `json:"dir"`
	Registered bool   `json:"registered"`
	// Problem says why it cannot be registered.
	Problem CandidateProblem `json:"problem,omitempty"`
}

// Candidates lists every directory under the projects directory, in name order.
func (u *ProjectUsecase) Candidates(ctx context.Context) ([]ProjectCandidate, error) {
	entries, err := os.ReadDir(u.settings.ProjectsDir)
	if err != nil {
		return nil, err
	}
	projects, err := u.projects.List(ctx)
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
			if c.Problem, err = u.problem(ctx, filepath.Join(u.settings.ProjectsDir, e.Name())); err != nil {
				return nil, err
			}
		}
		out = append(out, c)
	}
	return out, nil
}

func (u *ProjectUsecase) problem(ctx context.Context, path string) (CandidateProblem, error) {
	if _, err := os.Stat(filepath.Join(path, ".git")); errors.Is(err, os.ErrNotExist) {
		return ProblemNotGit, nil
	} else if err != nil {
		return "", err
	}
	_, _, ok, err := u.git.GitHubOrigin(ctx, path)
	if err != nil {
		return "", err
	}
	if !ok {
		return ProblemNotGitHub, nil
	}
	return "", nil
}

// Dir is the project's checkout on disk.
func (u *ProjectUsecase) Dir(p domain.Project) string {
	return filepath.Join(u.settings.ProjectsDir, p.Dir)
}

// Ensure returns the project for a directory under the projects directory,
// registering it from its git remote the first time it is used.
func (u *ProjectUsecase) Ensure(ctx context.Context, dir string, now time.Time) (domain.Project, error) {
	if dir == "" || strings.ContainsAny(dir, `/\`) || dir == "." || dir == ".." {
		return domain.Project{}, fmt.Errorf("%w: project must be a directory name under %s", ErrInvalidInput, u.settings.ProjectsDir)
	}
	p, err := u.projects.GetByDir(ctx, dir)
	if err == nil || !errors.Is(err, irepository.ErrNotFound) {
		return p, err
	}
	path := filepath.Join(u.settings.ProjectsDir, dir)
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return domain.Project{}, fmt.Errorf("%w: %s is not a directory", ErrInvalidInput, path)
	}
	remote, err := u.git.Remote(ctx, path)
	if err != nil {
		return domain.Project{}, fmt.Errorf("%w: read git remote of %s: %v", ErrInvalidInput, path, err)
	}
	p = domain.Project{Dir: dir, Owner: remote.Owner, Name: remote.Name, DefaultBranch: remote.DefaultBranch, CreatedAt: now}
	if err := u.projects.Create(ctx, &p); err != nil {
		return domain.Project{}, err
	}
	return p, nil
}
