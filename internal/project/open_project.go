package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/inoxlang/inox/internal/buntdb"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/project/access"
	"github.com/inoxlang/inox/internal/project/cloudflareprovider"
)

type OpenProjectParams struct {
	Id                core.ProjectID
	DevSideConfig     DevSideProjectConfig `json:"config"`
	TempTokens        *TempProjectTokens   `json:"tempTokens,omitempty"`
	MaxFilesystemSize core.ByteCount       `json:"-"`
	ExposeWebServers  bool
}

func (r *Registry) OpenProject(ctx *core.Context, params OpenProjectParams) (*Project, error) {
	r.openProjectsLock.Lock()
	defer r.openProjectsLock.Unlock()

	openProjects := r.openProjects

	if project, ok := openProjects[params.Id]; ok {
		return project, nil
	}

	// Get project data from the database.

	var serializedProjectData string
	var found bool

	err := r.metadata.View(func(tx *buntdb.Tx) error {
		projectKey := getProjectKvKey(params.Id)
		data, err := tx.Get(string(projectKey))
		if errors.Is(err, buntdb.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		serializedProjectData = data
		found = true
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error while reading KV: %w", err)
	}

	if !found {
		return nil, ErrProjectNotFound
	}

	config := ProjectConfiguration{
		ExposeWebServers: params.ExposeWebServers,
	}

	var projectData projectData
	err = json.Unmarshal([]byte(serializedProjectData), &projectData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal project's data: %w", err)
	}

	if projectData.Applications == nil {
		projectData.Applications = map[node.ApplicationName]*applicationData{}
	}

	if projectData.Secrets == nil {
		projectData.Secrets = map[core.SecretName]localSecret{}
	}

	// Open the project's filesystem

	projectDir := r.filesystem.Join(r.projectsDir, string(params.Id))
	projectFS, err := fs_ns.OpenMetaFilesystem(r.openProjectsContext, r.filesystem, fs_ns.MetaFilesystemParams{
		Dir:            projectDir,
		MaxUsableSpace: params.MaxFilesystemSize,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to open filesystem of project %s: %w", params.Id, err)
	}

	closeProjectFSBecauseOfError := true
	defer func() {
		if closeProjectFSBecauseOfError {
			projectFS.Close(ctx)
		}
	}()

	// Create and initialize a *Project.

	project := &Project{
		id:             params.Id,
		liveFilesystem: projectFS,
		tempTokens:     params.TempTokens,
		data:           projectData,
		persistFn:      r.persistProjectData,

		storeSecretsInProjectData: true,

		config: config,
	}

	if params.DevSideConfig.Cloudflare != nil {
		cf, err := cloudflareprovider.New(project.id, params.DevSideConfig.Cloudflare)
		if err != nil {
			return nil, fmt.Errorf("failed to create clouflare helper: %w", err)
		}
		project.cloudflare = cf
	}

	project.members = make([]*access.Member, 0, len(project.data.Members))
	for _, data := range project.data.Members {
		member, err := access.MemberFromData(data)
		if err != nil {
			return nil, fmt.Errorf("failed to create project member from data: %w", err)
		}

		if slices.ContainsFunc(project.members, func(m *access.Member) bool {
			return m.Name() == member.Name()
		}) {
			return nil, fmt.Errorf("invalid project member data: at least two members have the same name: %s", member.Name())
		}

		project.members = append(project.members, member)
	}

	project.Share(nil)
	r.openProjects[project.id] = project

	projectDevDatabasesDir, err := r.getCreateDevDatabasesDir(project.id)
	if err != nil {
		return nil, err
	}

	project.devDatabasesDirOnOsFs.Store(projectDevDatabasesDir)

	closeProjectFSBecauseOfError = false
	return project, nil
}
