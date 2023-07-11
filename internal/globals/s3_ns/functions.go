package s3_ns

import (
	"bytes"
	"errors"
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

	bucket, err := OpenBucket(ctx, u.Host())
	if err != nil {
		return nil, err
	}

	key := u.Path()

	if bucket.fakeBackend != nil {
		obj, err := bucket.fakeBackend.GetObject(bucket.name, string(key), nil)
		if err != nil {
			return nil, err
		}

		return &GetObjectResponse{
			output:      obj,
			fakeBackend: true,
		}, nil
	} else {
		output, err := bucket.client.libClient.GetObject(ctx, bucket.name, string(key)[1:], minio.GetObjectOptions{})

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

	bucket, err := OpenBucket(ctx, u.Host())
	if err != nil {
		return nil, err
	}

	key := string(u.Path())

	if bucket.fakeBackend != nil {
		return nil, errors.New("object listing not supported in s3 memory backend")
	} else {
		prefix := key[1:]
		prefixSlashClount := strings.Count(prefix, "/")

		channel := bucket.client.libClient.ListObjects(ctx, bucket.name, minio.ListObjectsOptions{
			Prefix:    prefix,
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

	bucket, err := OpenBucket(ctx, u.Host())
	if err != nil {
		return nil, err
	}

	key := u.Path()

	if bucket.fakeBackend != nil {
		content, err := reader.ReadAll()
		if err != nil {
			return nil, err
		}
		reader := bytes.NewReader(content.Bytes)
		obj, err := bucket.fakeBackend.PutObject(bucket.name, string(key), map[string]string{}, reader, int64(len(content.Bytes)))
		if err != nil {
			return nil, err
		}

		return &PutObjectResponse{
			output:      obj,
			fakeBackend: true,
		}, nil
	} else {
		//TODO: find way to get size without reading
		content, err := reader.ReadAll()
		if err != nil {
			return nil, err
		}
		reader := bytes.NewReader(content.Bytes)

		output, err := bucket.client.libClient.PutObject(
			ctx,
			bucket.name, string(key)[1:],
			reader, int64(len(content.Bytes)),
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

	bucket, err := OpenBucket(ctx, u.Host())
	if err != nil {
		return err
	}

	key := u.Path()

	if bucket.fakeBackend != nil {
		_, err := bucket.fakeBackend.DeleteObject(bucket.name, string(key))
		return err

	} else {
		return bucket.client.libClient.RemoveObject(ctx, bucket.name, string(key)[1:], minio.RemoveObjectOptions{})
	}

}

func S3GetBucketPolicy(ctx *core.Context, u core.URL) (*GetBucketPolicyResponse, error) {
	if err := checkScheme(u); err != nil {
		return nil, err
	}

	bucket, err := OpenBucket(ctx, u.Host())
	if err != nil {
		return nil, err
	}

	if bucket.fakeBackend != nil {
		return nil, errors.New("bucket policy retrieval not supported in s3 memory backend")
	} else {
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

	bucket, err := OpenBucket(ctx, u.Host())
	if err != nil {
		return err
	}

	if bucket.fakeBackend != nil {
		return errors.New("setting bucket policy is not supported in s3 memory backend")
	} else {
		return bucket.client.libClient.SetBucketPolicy(ctx, bucket.name, policyString)
	}
}

func S3RemoveBucketPolicy(ctx *core.Context, u core.URL) error {
	if err := checkScheme(u); err != nil {
		return err
	}

	bucket, err := OpenBucket(ctx, u.Host())
	if err != nil {
		return err
	}

	if bucket.fakeBackend != nil {
		return errors.New("removing bucket policy is not supported in s3 memory backend")
	} else {
		return bucket.client.libClient.SetBucketPolicy(ctx, bucket.name, "")
	}
}
