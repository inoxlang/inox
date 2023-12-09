package project

import (
	"bufio"
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/project/cloudflareprovider"
	"github.com/inoxlang/inox/internal/secrets"
)

const (
	PROJECTS_KV_PREFIX = "/projects"

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
	id   core.ProjectID
	lock core.SmartLock

	//filesystems and images

	//TODO: add base filesystem (VCS ?)
	liveFilesystem core.SnapshotableFilesystem

	//tokens and secrets

	tempTokens                *TempProjectTokens
	storeSecretsInProjectData bool
	secretStorage             secrets.SecretStorage //can be nil, always nil if storeSecretsInProjectData is true

	//providers

	cloudflare *cloudflareprovider.Cloudflare //can be nil
	data       projectData

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

func getProjectKvKey(id core.ProjectID) core.Path {
	return core.PathFrom(PROJECTS_KV_PREFIX + "/" + string(id))
}

type DevSideProjectConfig struct {
	Cloudflare *cloudflareprovider.DevSideConfig `json:"cloudflare,omitempty"`
}

// NewDummyProject creates a project without any providers or tokens,
// the returned project should only be used in test.
func NewDummyProject(name string, fls core.SnapshotableFilesystem) *Project {
	return &Project{
		id:                        core.RandomProjectID(name),
		liveFilesystem:            fls,
		storeSecretsInProjectData: true,
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

func (p *Project) LiveFilesystem() core.SnapshotableFilesystem {
	return p.liveFilesystem
}

func (p *Project) BaseImage() (core.Image, error) {
	snapshot, err := p.liveFilesystem.TakeFilesystemSnapshot(core.FilesystemSnapshotConfig{
		GetContent: func(ChecksumSHA256 [32]byte) core.AddressableContent {
			return nil
		},
		InclusionFilters: []core.PathPattern{"/**/*.ix", "/static/..."},
	})

	if err != nil {
		return nil, err
	}

	return &Image{
		filesystem: snapshot,
	}, nil
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

func (p *Project) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
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

// persisted data
type projectData struct {
	CreationParams CreateProjectParams                       `json:"creationParams"`
	Applications   map[node.ApplicationName]*applicationData `json:"applications,omitempty"`
	Secrets        map[core.SecretName]localSecret           `json:"secrets,omitempty"`
}
