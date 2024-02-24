package project

import (
	"errors"
	"fmt"
	"sync"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/project/access"
	"github.com/inoxlang/inox/internal/project/cloudflareprovider"
	"github.com/inoxlang/inox/internal/secrets"
)

const (
	PROJECTS_KV_PREFIX                           = "/projects"
	DEV_DATABASES_FOLDER_NAME_IN_PROCESS_TEMPDIR = "dev-databases"

	DEFAULT_MAIN_FILENAME = "main" + inoxconsts.INOXLANG_FILE_EXTENSION
	DEFAULT_TUT_FILENAME  = "learn.tut" + inoxconsts.INOXLANG_FILE_EXTENSION

	CREATION_PARAMS_KEY               = "creation-params"
	CREATION_PARAMS_NAME_KEY          = "name"
	CREATION_PARAMS_ADD_TUT_FILE_KEY  = "add-tut-file"
	CREATION_PARAMS_ADD_MAIN_FILE_KEY = "add-main-file"

	APPS_KEY = "applications"

	PROJECT_NAME_REGEX = "^[a-zA-Z][a-zA-Z0-9_-]*$"
)

var (
	ErrInvalidProjectName   = errors.New("invalid project name")
	ErrNoCloudflareProvider = errors.New("cloudflare provider not present")

	_ core.Value               = (*Project)(nil)
	_ core.PotentiallySharable = (*Project)(nil)
	_ core.Project             = (*Project)(nil)
)

type Project struct {
	data projectData

	id      core.ProjectID
	lock    core.SmartLock
	config  ProjectConfiguration
	members []*access.Member

	//filesystems and images

	//TODO: add base filesystem (VCS ?)
	osFilesystem        *fs_ns.OsFilesystem
	stagingFilesystem   core.SnapshotableFilesystem
	maxFilesystemSize   core.ByteCount
	developerCopies     map[access.MemberID]*developerCopy
	developerCopiesLock sync.Mutex

	dirOnOsFs    string
	fsDirOnOsFs  string
	devDirOnOsFs string

	//tokens and secrets

	tempTokens                *TempProjectTokens
	storeSecretsInProjectData bool
	secretStorage             secrets.SecretStorage //can be nil, always nil if storeSecretsInProjectData is true

	//providers

	cloudflare *cloudflareprovider.Cloudflare //can be nil

	persistFn func(ctx *core.Context, id core.ProjectID, data projectData) error //optional
}

func (p *Project) Id() core.ProjectID {
	return p.id
}

func (p *Project) CreationParams() CreateProjectParams {
	return p.data.CreationParams
}

func (p *Project) HasProviders() bool {
	return p.cloudflare != nil
}

func getProjectKvKey(id core.ProjectID) string {
	return PROJECTS_KV_PREFIX + "/" + string(id)
}

type DevSideProjectConfig struct {
	Cloudflare *cloudflareprovider.DevSideConfig `json:"cloudflare,omitempty"`
}

// NewDummyProject creates a project without any providers or tokens,
// the returned project should only be used in test.
func NewDummyProject(name string, fls core.SnapshotableFilesystem) *Project {
	return &Project{
		id:                        core.RandomProjectID(name),
		stagingFilesystem:         fls,
		storeSecretsInProjectData: true,
	}
}

// NewDummyProjectWithConfig creates a project without any providers or tokens,
// the returned project should only be used in test.
func NewDummyProjectWithConfig(name string, fls core.SnapshotableFilesystem, config ProjectConfiguration) *Project {
	return &Project{
		id:                        core.RandomProjectID(name),
		stagingFilesystem:         fls,
		storeSecretsInProjectData: true,
		config:                    config,
	}
}

func (p *Project) persistNoLock(ctx *core.Context) error {
	if p.persistFn == nil {
		return ErrProjectPersistenceNotAvailable
	}
	return p.persistFn(ctx, p.id, p.data)
}

func (p *Project) DeleteSecretsBucket(ctx *core.Context) error {
	bucket, err := p.getCreateSecretsBucket(ctx, false)
	if err != nil {
		if errors.Is(err, ErrSecretStorageAlreadySet) {
			return nil
		}
		return err
	}
	if bucket == nil {
		return nil
	}

	return p.cloudflare.DeleteR2Bucket(ctx, bucket)
}

func (p *Project) GetS3CredentialsForBucket(
	ctx *core.Context,
	bucketName string,
	provider string,
) (accessKey, secretKey string, s3Endpoint core.Host, _ error) {
	closestState := ctx.GetClosestState()
	p.lock.Lock(closestState, p)
	defer p.lock.Unlock(closestState, p)

	creds, err := p.cloudflare.GetCreateS3CredentialsForSingleBucket(ctx, bucketName, p.Id())
	if err != nil {
		return "", "", "", fmt.Errorf("%w: %w", cloudflareprovider.ErrNoR2Token, err)
	}
	accessKey = creds.AccessKey()
	secretKey = creds.SecretKey()
	s3Endpoint = creds.S3Endpoint()
	return
}

func (p *Project) CanProvideS3Credentials(s3Provider string) (bool, error) {
	switch s3Provider {
	case "cloudflare":
		return p.cloudflare != nil, nil
	}
	return false, nil
}

func (p *Project) StagingFilesystem() core.SnapshotableFilesystem {
	return p.stagingFilesystem
}

func (p *Project) Configuration() core.ProjectConfiguration {
	return p.config
}

func (p *Project) GetMemberByID(ctx *core.Context, id access.MemberID) (*access.Member, bool) {
	closestState := ctx.GetClosestState()
	p.lock.Lock(closestState, p)
	defer p.lock.Unlock(closestState, p)

	for _, member := range p.members {
		if member.ID() == id {
			return member, true
		}
	}

	return nil, false
}

func (p *Project) GetMemberByName(ctx *core.Context, name string) (*access.Member, bool) {
	closestState := ctx.GetClosestState()
	p.lock.Lock(closestState, p)
	defer p.lock.Unlock(closestState, p)

	for _, member := range p.members {
		if member.Name() == name {
			return member, true
		}
	}

	return nil, false
}

// persisted data
type projectData struct {
	CreationParams CreateProjectParams                       `json:"creationParams"`
	Applications   map[node.ApplicationName]*applicationData `json:"applications,omitempty"`
	Secrets        map[core.SecretName]localSecret           `json:"secrets,omitempty"`
	Members        []access.MemberData                       `json:"members,omitempty"` //names should be unique
}
