package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/inoxlang/inox/internal/buntdb"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
)

const (
	KV_FILENAME = "projects.kv"

	DEV_OS_DIR           = "dev" //dir present in the same level as projects and in each project
	DEV_DATABASES_OS_DIR = "databases"
	DEV_SERVERS_OS_DIR   = "servers"

	OWNER_MEMBER_NAME = "owner"
)

var (
	ErrProjectNotFound                = errors.New("project not found")
	ErrProjectPersistenceNotAvailable = errors.New("project persistence is not available")
)

type Registry struct {
	projectsDir string //projects directory on the OS filesystem
	filesystem  *fs_ns.OsFilesystem
	metadata    *buntdb.DB

	openProjects        map[core.ProjectID]*Project
	openProjectsLock    sync.Mutex
	openProjectsContext *core.Context

	//TODO: close idle projects (no FS access AND no provider-related accesses AND no production program running)
}

// OpenRegistry opens a project registry located on the OS filesystem.
func OpenRegistry(projectsDir string, openProjectsContext *core.Context) (*Registry, error) {
	kvPath := filepath.Join(projectsDir, KV_FILENAME)

	kv, err := buntdb.OpenBuntDBNoPermCheck(kvPath, fs_ns.GetOsFilesystem())

	if err != nil {
		return nil, fmt.Errorf("failed to open database of projects: %w", err)
	}

	return &Registry{
		projectsDir:         projectsDir,
		metadata:            kv,
		filesystem:          fs_ns.GetOsFilesystem(),
		openProjects:        map[core.ProjectID]*Project{},
		openProjectsContext: openProjectsContext,
	}, nil
}

func (r *Registry) Close(ctx *core.Context) {
	r.metadata.Close()
}

func (r *Registry) DevDir() (string, error) {
	devDir := filepath.Join(r.projectsDir, DEV_OS_DIR)

	err := r.filesystem.MkdirAll(devDir, fs_ns.DEFAULT_DIR_FMODE)
	if err != nil {
		return "", err
	}

	return devDir, nil
}

func (r *Registry) DevServersDir() (string, error) {
	devDir, err := r.DevDir()

	if err != nil {
		return "", err
	}

	return filepath.Join(devDir, DEV_SERVERS_OS_DIR), nil
}

func (r *Registry) getCreateDevDatabasesDir(id core.ProjectID) (projectDevDatabasesDir string, err error) {
	//create the dev dir that will store the dev databases

	devDir, err := r.DevDir()
	if err != nil {
		return "", err
	}

	//create the <dev dir>/<project id>/databases dir
	projectDevDatabasesDir = filepath.Join(devDir, string(id), DEV_DATABASES_OS_DIR)
	err = r.filesystem.MkdirAll(projectDevDatabasesDir, fs_ns.DEFAULT_DIR_FMODE)
	if err != nil {
		projectDevDatabasesDir = ""
		return
	}

	return projectDevDatabasesDir, nil
}

func (r *Registry) persistProjectData(ctx *core.Context, id core.ProjectID, data projectData) error {
	serialized, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal project data: %w", err)
	}

	return r.metadata.Update(func(tx *buntdb.Tx) error {
		key := getProjectKvKey(id)
		_, _, err := tx.Set(string(key), string(serialized), nil)
		return err
	})
}
