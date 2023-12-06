package project

import (
	"errors"
	"fmt"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
	s3 "github.com/inoxlang/inox/internal/secrets/s3"
)

var (
	ErrFailedToListSecrets     = errors.New("failed to list secrets")
	ErrNoSecretStorage         = errors.New("no secret storage")
	ErrSecretStorageAlreadySet = errors.New("secret storage field already set")
)

func (p *Project) ListSecrets(ctx *core.Context) (info []core.ProjectSecretInfo, _ error) {
	if p.secretStorage == nil {
		return nil, nil
	}
	return p.secretStorage.ListSecrets(ctx)
}

func (p *Project) GetSecrets(ctx *core.Context) (secrets []core.ProjectSecret, _ error) {
	if p.secretStorage == nil {
		return nil, nil
	}

	return p.secretStorage.GetSecrets(ctx)
}

func (p *Project) UpsertSecret(ctx *core.Context, name, value string) error {
	if p.secretStorage == nil {
		return ErrNoSecretStorage
	}

	return p.secretStorage.UpsertSecret(ctx, name, value)
}

func (p *Project) DeleteSecret(ctx *core.Context, name string) error {
	if p.secretStorage == nil {
		return ErrNoSecretStorage
	}

	return p.secretStorage.DeleteSecret(ctx, name)
}

func (p *Project) getSecretsBucketName() string {
	bucketName := "secrets-" + strings.ToLower(string(p.Id()))
	return bucketName
}

// getCreateSecretsBucket returns the bucket storing project secrets, if it does not exists and
// createIfDoesNotExist is true the bucket is created, otherwise a nil bucket is returned (no error).
func (p *Project) getCreateSecretsBucket(ctx *core.Context, createIfDoesNotExist bool) (*s3_ns.Bucket, error) {
	closestState := ctx.GetClosestState()
	p.lock.Lock(closestState, p)
	defer p.lock.Unlock(closestState, p)

	if storage, ok := p.secretStorage.(*s3.S3SecretStorage); ok {
		return storage.Bucket(), nil
	}

	if p.secretStorage != nil {
		return nil, ErrSecretStorageAlreadySet
	}

	if p.cloudflare == nil {
		return nil, ErrNoCloudflareProvider
	}
	cf := p.cloudflare

	accessKey, secretKey, ok := cf.GetHighPermsS3Credentials(ctx)
	if !ok {
		return nil, errors.New("missing Cloudflare R2 token")
	}

	bucketName := p.getSecretsBucketName()
	{
		exists, err := cf.CheckBucketExists(ctx, bucketName)
		if err != nil {
			return nil, err
		}

		if !exists {
			if createIfDoesNotExist {
				err = cf.CreateR2Bucket(ctx, bucketName)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, nil
			}
		}
	}

	bucket, err := s3_ns.OpenBucketWithCredentials(ctx, s3_ns.OpenBucketWithCredentialsInput{
		Provider:   "cloudflare",
		BucketName: bucketName,
		HttpsHost:  cf.S3EndpointForR2(),
		AccessKey:  accessKey,
		SecretKey:  secretKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open bucket: %w", err)
	}
	p.secretStorage = s3.NewStorageFromBucket(bucket)
	return bucket, err
}
