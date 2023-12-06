package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/project/cloudflareprovider"
)

const (
	KV_FILENAME = "projects.kv"
)

var (
	ErrProjectNotFound                = errors.New("project not found")
	ErrProjectPersistenceNotAvailable = errors.New("project persistence is not available")
)

type Registry struct {
	projectsDir         string
	filesystem          afs.Filesystem
	metadata            *filekv.SingleFileKV
	openProjects        map[core.ProjectID]*Project
	openProjectsContext *core.Context

	//TODO: close idle projects (no FS access AND no provider-related accesses AND no production program running)
}

func OpenRegistry(projectsDir string, fls afs.Filesystem, openProjectsContext *core.Context) (*Registry, error) {
	kvPath := fls.Join(projectsDir, KV_FILENAME)

	kv, err := filekv.OpenSingleFileKV(filekv.KvStoreConfig{
		Path:       core.PathFrom(kvPath),
		Filesystem: fls,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to open database of projects: %w", err)
	}

	return &Registry{
		projectsDir:         projectsDir,
		filesystem:          fls,
		metadata:            kv,
		openProjects:        map[core.ProjectID]*Project{},
		openProjectsContext: openProjectsContext,
	}, nil
}

func (r *Registry) Close(ctx *core.Context) {
	r.metadata.Close(ctx)
}

type CreateProjectParams struct {
	Name        string `json:"name"`
	AddMainFile bool   `json:"addMainFile,omitempty"`
	AddTutFile  bool   `json:"addTutFile,omitempty"`
}

// CreateProject
func (r *Registry) CreateProject(ctx *core.Context, params CreateProjectParams) (core.ProjectID, error) {
	if matched, err := regexp.MatchString(PROJECT_NAME_REGEX, params.Name); !matched || err != nil {
		return "", ErrInvalidProjectName
	}
	id := core.RandomProjectID(params.Name)

	// create the directory for storing projects if necessary
	err := r.filesystem.MkdirAll(r.projectsDir, fs_ns.DEFAULT_DIR_FMODE)
	if err != nil {
		return "", fmt.Errorf("failed to create directory to store projects: %w", err)
	}

	// persist data

	r.persistProjectData(ctx, id, projectData{CreationParams: params})

	// create the project's directory
	projectDir := r.filesystem.Join(r.projectsDir, string(id))
	err = r.filesystem.MkdirAll(projectDir, fs_ns.DEFAULT_DIR_FMODE)

	if err != nil {
		return "", fmt.Errorf("failed to create directory for project %s: %w", id, err)
	}

	// create initial files
	projectFS, err := fs_ns.OpenMetaFilesystem(ctx, r.filesystem, fs_ns.MetaFilesystemParams{
		Dir: projectDir,
	})
	if err != nil {
		return "", fmt.Errorf("failed to open the project filesystem to write initial files %s: %w", id, err)
	}

	defer projectFS.Close(ctx)

	if params.AddMainFile {
		util.WriteFile(projectFS, DEFAULT_MAIN_FILENAME, []byte("manifest {\n\n}"), fs_ns.DEFAULT_FILE_FMODE)
	}

	if params.AddTutFile {
		util.WriteFile(projectFS, DEFAULT_TUT_FILENAME, []byte(nil), fs_ns.DEFAULT_DIR_FMODE)
	}

	return id, nil
}

type OpenProjectParams struct {
	Id            core.ProjectID
	DevSideConfig DevSideProjectConfig `json:"config"`
	TempTokens    *TempProjectTokens   `json:"tempTokens,omitempty"`
}

func (r *Registry) OpenProject(ctx *core.Context, params OpenProjectParams) (*Project, error) {
	if project, ok := r.openProjects[params.Id]; ok {
		return project, nil
	}

	serializedProjectData, found, err := r.metadata.GetSerialized(ctx, getProjectKvKey(params.Id), r)

	if err != nil {
		return nil, fmt.Errorf("error while reading KV: %w", err)
	}

	if !found {
		return nil, ErrProjectNotFound
	}

	// get project data from the database

	var projectData projectData
	err = json.Unmarshal([]byte(serializedProjectData), &projectData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal project's data: %w", err)
	}

	if projectData.Applications == nil {
		projectData.Applications = map[node.ApplicationName]*applicationData{}
	}

	if projectData.Secrets == nil {
		projectData.Secrets = map[core.SecretName]core.ProjectSecret{}
	}

	// open the project's filesystem

	projectDir := r.filesystem.Join(r.projectsDir, string(params.Id))
	projectFS, err := fs_ns.OpenMetaFilesystem(r.openProjectsContext, r.filesystem, fs_ns.MetaFilesystemParams{
		Dir: projectDir,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to open filesystem of project %s: %w", params.Id, err)
	}

	project := &Project{
		id:             params.Id,
		liveFilesystem: projectFS,
		tempTokens:     params.TempTokens,
		data:           projectData,
		persistFn:      r.persistProjectData,

		storeSecretsInProjectData: true,
	}

	if params.DevSideConfig.Cloudflare != nil {
		cf, err := cloudflareprovider.New(project.id, params.DevSideConfig.Cloudflare)
		if err != nil {
			return nil, fmt.Errorf("failed to create clouflare helper: %w", err)
		}
		project.cloudflare = cf
	}

	project.Share(nil)
	r.openProjects[project.id] = project

	return project, nil
}

func (r *Registry) persistProjectData(ctx *core.Context, id core.ProjectID, data projectData) error {
	serialized, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal project data: %w", err)
	}

	r.metadata.SetSerialized(ctx, getProjectKvKey(id), string(serialized), r)
	return nil
}
