package project

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrFailedToListSecrets = errors.New("failed to list secrets")
)

type ProjectSecret struct {
	Name          string
	LastModifDate time.Time
	Value         *core.Secret
}

type ProjectSecretInfo struct {
	Name          string    `json:"name"`
	LastModifDate time.Time `json:"lastModificationDate"`
}

func (p *Project) ListSecrets(ctx *core.Context) (info []ProjectSecretInfo, _ error) {
	bucket, err := p.getCreateSecretsBucket(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToListSecrets, err)
	}

	if bucket == nil {
		return nil, nil
	}

	objects, err := bucket.ListObjects(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToListSecrets, err)
	}
	return utils.MapSlice(objects, func(o *s3_ns.ObjectInfo) ProjectSecretInfo {
		return ProjectSecretInfo{
			Name:          o.Key,
			LastModifDate: o.LastModified,
		}
	}), nil
}

func (p *Project) ListSecrets2(ctx *core.Context) (secrets []ProjectSecret, _ error) {
	bucket, err := p.getCreateSecretsBucket(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToListSecrets, err)
	}

	if bucket == nil {
		return nil, nil
	}

	objects, err := bucket.ListObjects(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToListSecrets, err)
	}

	wg := new(sync.WaitGroup)
	wg.Add(len(objects))

	secrets = make([]ProjectSecret, len(objects))
	errs := make([]error, len(objects))

	var lock sync.Mutex

	for i, obj := range objects {
		go func(i int, info *s3_ns.ObjectInfo) {
			defer wg.Done()
			resp, err := bucket.GetObject(ctx, info.Key)
			if err != nil {
				lock.Lock()
				errs[i] = err
				lock.Unlock()
				return
			}
			content, err := resp.ReadAll()
			if err != nil {
				lock.Lock()
				errs[i] = err
				lock.Unlock()
				return
			}

			secretValue, err := core.SECRET_STRING_PATTERN.NewSecret(ctx, string(content))
			if err != nil {
				lock.Lock()
				errs[i] = err
				lock.Unlock()
				return
			}

			secrets[i] = ProjectSecret{
				Name:          info.Key,
				Value:         secretValue,
				LastModifDate: info.LastModified,
			}
		}(i, obj)
	}

	wg.Wait()

	errString := ""
	for _, err := range errs {
		if err != nil {
			if errString != "" {
				errString += "\n"
			}
			errString += err.Error()
		}
	}

	if errString != "" {
		return nil, errors.New(errString)
	}
	return secrets, nil
}

func (p *Project) UpsertSecret(ctx *core.Context, name, value string) error {
	for _, r := range name {
		if !parse.IsIdentChar(r) {
			return fmt.Errorf("invalid char found in secret's name: '%c'", r)
		}
	}
	bucket, err := p.getCreateSecretsBucket(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to add secret %q: %w", name, err)
	}

	_, err = bucket.PutObject(ctx, name, strings.NewReader(value))
	if err != nil {
		return fmt.Errorf("failed to add secret %q: %w", name, err)
	}
	return nil
}

func (p *Project) DeleteSecret(ctx *core.Context, name string) error {
	for _, r := range name {
		if !parse.IsIdentChar(r) {
			return fmt.Errorf("invalid char found in secret's name: '%c'", r)
		}
	}
	bucket, err := p.getCreateSecretsBucket(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to delete secret %q: %w", name, err)
	}

	err = bucket.DeleteObject(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to delete secret %q: %w", name, err)
	}
	return nil
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

	if p.secretsBucket != nil {
		return p.secretsBucket, nil
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
		if cf.highPermsTokens.R2Token == nil {
			return nil, ErrNoR2Token
		}

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
	p.secretsBucket = bucket
	return bucket, err
}
