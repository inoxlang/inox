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
	projectsDir string
	filesystem  afs.Filesystem
	kv          *filekv.SingleFileKV
}

func OpenRegistry(projectsDir string, fls afs.Filesystem) (*Registry, error) {
	kvPath := fls.Join(projectsDir, KV_FILENAME)

	kv, err := filekv.OpenSingleFileKV(filekv.KvStoreConfig{
		Path:       core.PathFrom(kvPath),
		Filesystem: fls,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to open database of projects")
	}

	return &Registry{
		projectsDir: projectsDir,
		filesystem:  fls,
		kv:          kv,
	}, nil
}

func (r *Registry) Close(ctx *core.Context) {
	r.kv.Close(ctx)
}
