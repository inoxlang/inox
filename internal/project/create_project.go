package project

import (
	"fmt"
	"regexp"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/project/access"
	"github.com/inoxlang/inox/internal/project/scaffolding"
)

type CreateProjectParams struct {
	Name       string `json:"name"`
	Template   string `json:"template,omitempty"`
	AddTutFile bool   `json:"addTutFile,omitempty"`
}

// CreateProject creates a project and returns the project ID and the ID the owner member.
func (r *Registry) CreateProject(ctx *core.Context, params CreateProjectParams) (core.ProjectID, access.MemberID, error) {
	if matched, err := regexp.MatchString(PROJECT_NAME_REGEX, params.Name); !matched || err != nil {
		return "", "", ErrInvalidProjectName
	}
	id := core.RandomProjectID(params.Name)

	//Create the directory for storing projects if necessary.
	err := r.filesystem.MkdirAll(r.projectsDir, fs_ns.DEFAULT_DIR_FMODE)
	if err != nil {
		return "", "", fmt.Errorf("failed to create directory to store projects: %w", err)
	}

	//Initialize project data.

	ownerMember := access.MemberData{
		Name: OWNER_MEMBER_NAME,
		ID:   access.RandomMemberID(),
	}

	projectData := projectData{
		CreationParams: params,
		Members:        []access.MemberData{ownerMember},
	}

	// persist data

	r.persistProjectData(ctx, id, projectData)

	//Create the project's directory.
	projectFsDir := r.projectFsDir(id)
	err = r.filesystem.MkdirAll(projectFsDir, fs_ns.DEFAULT_DIR_FMODE)

	if err != nil {
		return "", "", fmt.Errorf("failed to create directory for project %s: %w", id, err)
	}

	//Create initial files.
	projectFS, err := fs_ns.OpenMetaFilesystem(ctx, r.filesystem, fs_ns.MetaFilesystemParams{
		Dir: projectFsDir,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to open the project filesystem to write initial files %s: %w", id, err)
	}

	defer projectFS.Close(ctx)

	if params.Template != "" {
		if err := scaffolding.WriteTemplate(params.Template, projectFS); err != nil {
			return "", "", fmt.Errorf("failed to write template %q to the project filesystem: %w", params.Template, err)
		}
	}

	if params.AddTutFile {
		util.WriteFile(projectFS, DEFAULT_TUT_FILENAME, []byte(nil), fs_ns.DEFAULT_DIR_FMODE)
	}

	return id, ownerMember.ID, nil
}

func (r *Registry) projectFsDir(id core.ProjectID) string {
	return r.filesystem.Join(r.projectsDir, string(id), FS_OS_DIR)
}
