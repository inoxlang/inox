package project

import (
	"bufio"
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
	"github.com/oklog/ulid/v2"
)

const (
	PROJECTS_KV_PREFIX = "/projects"
)

var (
	ErrProjectNotFound = errors.New("project not found")

	_ core.Value               = (*Project)(nil)
	_ core.PotentiallySharable = (*Project)(nil)
)

type Project struct {
	id                ProjectID
	projectFilesystem afs.Filesystem
	lock              core.SmartLock
	devSideConfig     DevSideProjectConfig
	tempTokens        *TempProjectTokens
	secretsBucket     *s3_ns.Bucket
}

func (p *Project) Id() ProjectID {
	return p.id
}

func (p *Project) DevSideConfig() DevSideProjectConfig {
	return p.devSideConfig
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
	Id            ProjectID
	DevSideConfig DevSideProjectConfig `json:"config"`
	TempTokens    *TempProjectTokens   `json:"tempTokens,omitempty"`
}

type DevSideProjectConfig struct {
	Cloudflare *DevSideCloudflareConfig `json:"cloudflare,omitempty"`
}

type DevSideCloudflareConfig struct {
	AdditionalTokensApiToken string `json:"additional-tokens-api-token"`
	AccountID                string `json:"account-id"`
}

// OpenProject
func (r *Registry) OpenProject(ctx *core.Context, params OpenProjectParams) (*Project, error) {
	_, found, err := r.kv.Get(ctx, params.Id.KvKey(), r)

	if err != nil {
		return nil, fmt.Errorf("error while reading KV: %w", err)
	}

	if !found {
		return nil, ErrProjectNotFound
	}

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
		devSideConfig:     params.DevSideConfig,
		tempTokens:        params.TempTokens,
	}

	return project, nil
}

func (p *Project) Filesystem() afs.Filesystem {
	return p.projectFilesystem
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

func (p *Project) IsSharable(originState *core.GlobalState) (bool, string) {
	return true, ""
}

func (p *Project) Share(originState *core.GlobalState) {
	p.lock.Share(originState, func() {

	})
}

func (p *Project) IsShared() bool {
	return p.lock.IsValueShared()
}

func (p *Project) ForceLock() {
	p.lock.ForceLock()
}
func (p *Project) ForceUnlock() {
	p.lock.ForceUnlock()
}
