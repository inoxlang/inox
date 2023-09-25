package project

import (
	"fmt"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/filekv"
)

const (
	KV_FILENAME = "projects.kv"
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
		return nil, fmt.Errorf("failed to open database of projects")
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
