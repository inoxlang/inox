package s3

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/inoxlang/inox/internal/core"
	s3_ns "github.com/inoxlang/inox/internal/globals/s3_ns"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/secrets"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = secrets.SecretStorage((*S3SecretStorage)(nil))
)

type S3SecretStorage struct {
	bucket *s3_ns.Bucket
}

func NewStorageFromBucket(bucket *s3_ns.Bucket) *S3SecretStorage {
	return &S3SecretStorage{
		bucket: bucket,
	}
}

func (s *S3SecretStorage) Bucket() *s3_ns.Bucket {
	return s.bucket
}

func (s *S3SecretStorage) ListSecrets(ctx *core.Context) (info []core.ProjectSecretInfo, _ error) {
	objects, err := s.bucket.ListObjects(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", secrets.ErrFailedToListSecrets, err)
	}

	for _, obj := range objects {
		name, err := core.SecretNameFrom(obj.Key)
		//ignore object if its name is not a valid secret name
		if err != nil {
			continue
		}
		info = append(info, core.ProjectSecretInfo{
			Name:          name,
			LastModifDate: obj.LastModified,
		})
	}

	return
}

// GetSecrets implements secrets.SecretStorage.
func (s *S3SecretStorage) GetSecrets(ctx *core.Context) (secrets []core.ProjectSecret, _ error) {
	objects, err := s.bucket.ListObjects(ctx, "")
	if err != nil {
		return nil, err
	}

	//TODO: investigate why this fix is needed
	objects = utils.FilterMapSlice(objects, func(s *s3_ns.ObjectInfo) (*s3_ns.ObjectInfo, bool) {
		if s.Key == "" {
			return nil, false
		}
		return s, true
	})

	wg := new(sync.WaitGroup)
	wg.Add(len(objects))

	secretNames := map[string]core.SecretName{}

	//keep objects with a valid secret name
	objects = utils.FilterSlice(objects, func(obj *s3_ns.ObjectInfo) bool {
		if obj.Key == "" {
			return false
		}

		name, err := core.SecretNameFrom(obj.Key)
		//ignore object if its name is not a valid secret name
		if err != nil {
			return false
		}
		secretNames[obj.Key] = name
		return true
	})

	secrets = make([]core.ProjectSecret, len(objects))
	errs := make([]error, len(objects))

	var lock sync.Mutex

	for i, obj := range objects {
		go func(i int, info *s3_ns.ObjectInfo) {
			defer utils.Recover()

			defer wg.Done()
			resp, err := s.bucket.GetObject(ctx, info.Key)
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

			secrets[i] = core.ProjectSecret{
				Name:          secretNames[info.Key],
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

func (s *S3SecretStorage) UpsertSecret(ctx *core.Context, name string, value string) error {
	for _, r := range name {
		if !parse.IsIdentChar(r) {
			return fmt.Errorf("invalid char found in secret's name: '%c'", r)
		}
	}

	_, err := s.bucket.PutObject(ctx, name, strings.NewReader(value))
	if err != nil {
		return fmt.Errorf("failed to add secret %q: %w", name, err)
	}
	return nil
}

func (s *S3SecretStorage) DeleteSecret(ctx *core.Context, name string) error {
	for _, r := range name {
		if !parse.IsIdentChar(r) {
			return fmt.Errorf("invalid char found in secret's name: '%c'", r)
		}
	}

	err := s.bucket.DeleteObject(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to delete secret %q: %w", name, err)
	}
	return nil
}
