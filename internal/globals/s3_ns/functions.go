package s3_ns

import (
	"bytes"
	"errors"
	"io"
	"strings"

	core "github.com/inoxlang/inox/internal/core"
	jsoniter "github.com/json-iterator/go"
	"github.com/minio/minio-go/v7"
)

var (
	ErrOnlyS3httpsURLsSupported = errors.New("only s3:// & https:// urls are supported")
)

func checkScheme(u core.URL) error {
	if u.Scheme() != "s3" && u.Scheme() != "https" && u.Scheme() != "http" {
		return ErrOnlyS3httpsURLsSupported
	}
	return nil
}

func S3Get(ctx *core.Context, u core.URL) (*GetObjectResponse, error) {
	if err := checkScheme(u); err != nil {
		return nil, err
	}

	bucket, err := OpenBucket(ctx, u.Host(), OpenBucketOptions{})
	if err != nil {
		return nil, err
	}

	key := u.Path()

	return bucket.GetObject(ctx, string(key))
}

func (b *Bucket) GetObject(ctx *core.Context, key string) (*GetObjectResponse, error) {
	key = toObjectKey(key)

	if b.fakeBackend != nil {
		obj, err := b.fakeBackend.GetObject(b.name, key, nil)
		if err != nil {
			return nil, err
		}

		return &GetObjectResponse{
			output:      obj,
			fakeBackend: true,
		}, nil
	} else {
		output, err := core.DoIO2(ctx, func() (*minio.Object, error) {
			return b.client.libClient.GetObject(ctx, b.name, key, minio.GetObjectOptions{})
		})

		if err != nil {
			return nil, err
		}

		return &GetObjectResponse{
			output:      output,
			fakeBackend: false,
		}, nil
	}
}

func S3List(ctx *core.Context, u core.URL) ([]*ObjectInfo, error) {
	if err := checkScheme(u); err != nil {
		return nil, err
	}

	bucket, err := OpenBucket(ctx, u.Host(), OpenBucketOptions{})
	if err != nil {
		return nil, err
	}

	key := string(u.Path())
	return bucket.ListObjects(ctx, key)
}

func (b *Bucket) ListObjects(ctx *core.Context, key string) ([]*ObjectInfo, error) {
	if key != "" {
		key = toObjectKey(key)
	}

	if b.fakeBackend != nil {
		return nil, errors.New("object listing not supported in s3 memory backend")
	} else {
		prefixSlashClount := strings.Count(key, "/")

		ctx.PauseCPUTimeDecrementation()
		defer ctx.ResumeCPUTimeDecrementation()

		channel := b.client.libClient.ListObjects(ctx, b.name, minio.ListObjectsOptions{
			Prefix:    key,
			Recursive: false,
		})

		var objects []*ObjectInfo
		for obj := range channel {
			if strings.HasSuffix(obj.Key, "/") && strings.Count(obj.Key, "/") == prefixSlashClount {
				continue
			}
			objects = append(objects, &ObjectInfo{ObjectInfo: obj})
		}

		return objects, nil
	}
}

func S3put(ctx *core.Context, u core.URL, readable core.Readable) (*PutObjectResponse, error) {
	if err := checkScheme(u); err != nil {
		return nil, err
	}
	reader := readable.Reader()

	bucket, err := OpenBucket(ctx, u.Host(), OpenBucketOptions{})
	if err != nil {
		return nil, err
	}

	key := u.Path()
	ctx.PauseCPUTimeDecrementation()
	defer ctx.ResumeCPUTimeDecrementation()
	return bucket.PutObject(ctx, string(key), reader)
}

func (bucket *Bucket) PutObject(ctx *core.Context, key string, body io.Reader) (*PutObjectResponse, error) {
	key = toObjectKey(key)

	if bucket.fakeBackend != nil {
		content, err := io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		reader := bytes.NewReader(content)
		obj, err := bucket.fakeBackend.PutObject(bucket.name, string(key), map[string]string{}, reader, int64(len(content)))
		if err != nil {
			return nil, err
		}

		return &PutObjectResponse{
			output:      obj,
			fakeBackend: true,
		}, nil
	} else {
		//TODO: find way to get size without reading
		content, err := io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		reader := bytes.NewReader(content)

		ctx.PauseCPUTimeDecrementation()
		defer ctx.ResumeCPUTimeDecrementation()

		output, err := bucket.client.libClient.PutObject(
			ctx,
			bucket.name, string(key),
			reader, int64(len(content)),
			minio.PutObjectOptions{},
		)

		if err != nil {
			return nil, err
		}

		return &PutObjectResponse{
			output:      output,
			fakeBackend: false,
		}, nil
	}
}

func S3Delete(ctx *core.Context, u core.URL, readable core.Readable) error {
	if err := checkScheme(u); err != nil {
		return err
	}

	bucket, err := OpenBucket(ctx, u.Host(), OpenBucketOptions{})
	if err != nil {
		return err
	}

	key := u.Path()
	return bucket.DeleteObject(ctx, string(key))
}

func (bucket *Bucket) DeleteObject(ctx *core.Context, key string) error {
	key = toObjectKey(key)

	if bucket.fakeBackend != nil {
		_, err := bucket.fakeBackend.DeleteObject(bucket.name, key)
		return err

	} else {
		ctx.PauseCPUTimeDecrementation()
		defer ctx.ResumeCPUTimeDecrementation()

		return bucket.client.libClient.RemoveObject(ctx, bucket.name, key, minio.RemoveObjectOptions{})
	}
}

func S3GetBucketPolicy(ctx *core.Context, u core.URL) (*GetBucketPolicyResponse, error) {
	if err := checkScheme(u); err != nil {
		return nil, err
	}

	bucket, err := OpenBucket(ctx, u.Host(), OpenBucketOptions{})
	if err != nil {
		return nil, err
	}

	if bucket.fakeBackend != nil {
		return nil, errors.New("bucket policy retrieval not supported in s3 memory backend")
	} else {
		ctx.PauseCPUTimeDecrementation()
		defer ctx.ResumeCPUTimeDecrementation()

		output, err := bucket.client.libClient.GetBucketPolicy(ctx, bucket.name)

		if err != nil {
			return nil, err
		}

		return &GetBucketPolicyResponse{
			s: output,
		}, nil
	}

}

func S3SetBucketPolicy(ctx *core.Context, u core.URL, policy core.Value) error {
	if err := checkScheme(u); err != nil {
		return err
	}

	var policyString string

	switch p := policy.(type) {
	case core.Str:
		policyString = string(p)
	case *core.Object:
		stream := jsoniter.NewStream(jsoniter.ConfigCompatibleWithStandardLibrary, nil, 0)
		config := core.JSONSerializationConfig{
			ReprConfig: &core.ReprConfig{},
		}
		if err := p.WriteJSONRepresentation(ctx, stream, config, 0); err != nil {
			return errors.New("invalid policy description: pass a string or an object with a JSON representation")
		}
		policyString = string(stream.Buffer())
	default:
		return errors.New("invalid policy description: pass a string or an object with a JSON representation")
	}

	if strings.TrimSpace(policyString) == " " {
		return errors.New("invalid policy description: empty")
	}

	bucket, err := OpenBucket(ctx, u.Host(), OpenBucketOptions{})
	if err != nil {
		return err
	}

	if bucket.fakeBackend != nil {
		return errors.New("setting bucket policy is not supported in s3 memory backend")
	} else {
		ctx.PauseCPUTimeDecrementation()
		defer ctx.ResumeCPUTimeDecrementation()

		return bucket.client.libClient.SetBucketPolicy(ctx, bucket.name, policyString)
	}
}

func S3RemoveBucketPolicy(ctx *core.Context, u core.URL) error {
	if err := checkScheme(u); err != nil {
		return err
	}

	bucket, err := OpenBucket(ctx, u.Host(), OpenBucketOptions{})
	if err != nil {
		return err
	}

	if bucket.fakeBackend != nil {
		return errors.New("removing bucket policy is not supported in s3 memory backend")
	} else {
		ctx.PauseCPUTimeDecrementation()
		defer ctx.ResumeCPUTimeDecrementation()

		return bucket.client.libClient.SetBucketPolicy(ctx, bucket.name, "")
	}
}
