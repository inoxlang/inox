package project

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

type ProjectSecretInfo struct {
	Name          string `json:"name"`
	LastModifDate string `json:"lastModificationDate"`
}

func (p *Project) ListSecrets(ctx *core.Context) (info []ProjectSecretInfo, _ error) {
	bucket, err := p.getCreateSecretsBucket(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	objects, err := bucket.ListObjects(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}
	return utils.MapSlice(objects, func(o *s3_ns.ObjectInfo) ProjectSecretInfo {
		return ProjectSecretInfo{
			Name:          o.Key,
			LastModifDate: o.LastModified.Format(time.RFC3339),
		}
	}), nil
}

func (p *Project) UpsertSecret(ctx *core.Context, name, value string) error {
	for _, r := range name {
		if !parse.IsIdentChar(r) {
			return fmt.Errorf("invalid char found in secret: '%c'", r)
		}
	}
	bucket, err := p.getCreateSecretsBucket(ctx)
	if err != nil {
		return fmt.Errorf("failed to add secret %q: %w", name, err)
	}

	_, err = bucket.PutObject(ctx, name, strings.NewReader(value))
	if err != nil {
		return fmt.Errorf("failed to add secret %q: %w", name, err)
	}
	return nil
}

func (p *Project) getCreateSecretsBucket(ctx *core.Context) (*s3_ns.Bucket, error) {
	closestState := ctx.GetClosestState()
	p.lock.Lock(closestState, p)
	defer p.lock.Unlock(closestState, p)

	if p.secretsBucket != nil {
		return p.secretsBucket, nil
	}

	if p.devSideConfig.Cloudflare == nil || p.devSideConfig.Cloudflare.AccountID == "" {
		return nil, errors.New("missing Cloudflare account ID")
	}
	accountId := p.devSideConfig.Cloudflare.AccountID

	tokens, err := p.TempProjectTokens(ctx)
	if err != nil {
		return nil, err
	}

	bucketName := "secrets-" + strings.ToLower(string(p.Id()))
	accessKey, secretKey, ok := tokens.Cloudflare.GetS3AccessKeySecretKey()
	if !ok {
		return nil, errors.New("missing Cloudflare R2 token")
	}

	err = CreateR2BucketIfNotExist(ctx, bucketName, *tokens.Cloudflare, accountId)
	if err != nil {
		return nil, err
	}

	bucket, err := s3_ns.OpenBucketWithCredentials(ctx, s3_ns.OpenBucketWithCredentialsInput{
		Provider:   "cloudflare",
		BucketName: bucketName,
		HttpsHost:  core.Host("https://" + p.devSideConfig.Cloudflare.AccountID + ".r2.cloudflarestorage.com"),
		AccessKey:  accessKey,
		SecretKey:  secretKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open bucket: %w", err)
	}
	p.secretsBucket = bucket
	return bucket, err
}
