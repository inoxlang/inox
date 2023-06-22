package project

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/oklog/ulid/v2"
)

const (
	PROJECTS_KV_PREFIX = "/projects"
)

var (
	_ core.Value = (*Project)(nil)
)

type Project struct {
	id                ProjectID
	projectFilesystem afs.Filesystem

	core.NoReprMixin
	core.NotClonableMixin
}

type ProjectID string

func RandomProjectID(projectName string) ProjectID {
	return ProjectID(projectName + "-" + ulid.Make().String())
}

func (id ProjectID) KvKey() core.Path {
	return core.PathFrom(PROJECTS_KV_PREFIX + "/" + string(id))
}

type CreateProjectParams struct {
	Name string
}

// CreateProject
func (r *Registry) CreateProject(ctx *core.Context, params CreateProjectParams) (ProjectID, error) {
	id := RandomProjectID(params.Name)

	err := r.filesystem.MkdirAll(r.projectsDir, fs_ns.DEFAULT_DIR_FMODE)
	if err != nil {
		return "", fmt.Errorf("failed to create directory to store projects: %w", err)
	}

	r.kv.Insert(ctx, id.KvKey(), core.Nil, r)

	projectDir := r.filesystem.Join(r.projectsDir, string(id))

	err = r.filesystem.MkdirAll(projectDir, fs_ns.DEFAULT_DIR_FMODE)
	if err != nil {
		return "", fmt.Errorf("failed to create directory for project %s: %w", id, err)
	}

	return id, nil
}

type OpenProjectParams struct {
	Id ProjectID
}

// OpenProject
func (r *Registry) OpenProject(ctx *core.Context, params OpenProjectParams) (*Project, error) {
	projectDir := r.filesystem.Join(r.projectsDir, string(params.Id))

	projectFS, err := fs_ns.OpenMetaFilesystem(ctx, r.filesystem, fs_ns.MetaFilesystemOptions{
		Dir: projectDir,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to open filesystem of project %s", params.Id)
	}

	project := &Project{
		id:                params.Id,
		projectFilesystem: projectFS,
	}

	return project, nil
}

func (p *Project) IsMutable() bool {
	return true
}

func (p *Project) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherProject, ok := other.(*Project)
	if !ok {
		return false
	}

	return p == otherProject
}

func (p *Project) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	core.PrintType(w, p)
}

func (p *Project) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY, nil
}
