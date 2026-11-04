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
