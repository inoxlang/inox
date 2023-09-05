package s3_ns

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"golang.org/x/exp/slices"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

var (
	ErrCannotResolveBucket = errors.New("cannot resolve bucket")

	//<mem URL> -> Bucket
	//<https URL> -> Public Bucket
	//<https URL> ':' <bucket name> -> Bucket
	openBucketMap     = make(map[string]BucketMapItem)
	openBucketMapLock sync.RWMutex
)

func init() {
	core.RegisterStaticallyCheckHostResolutionDataFn("s3", func(project core.Project, node parse.Node) (errorMsg string) {
		const MAIN_ERR_MSG = "invalid host resolution data: accepted values are: HTTPS hosts (public buckets), mem:// URLs and object literals"
		switch n := node.(type) {
		case *parse.HostLiteral:
			if !strings.HasPrefix(n.Value, "https") {
				return MAIN_ERR_MSG
			}
			return ""
		case *parse.URLLiteral:
			if !strings.HasPrefix(n.Value, "mem") {
				return MAIN_ERR_MSG
			}
			return ""
		case *parse.ObjectLiteral:
			s3Provider := ""
			if providerNode, hasProvider := n.PropValue("provider"); hasProvider {
				if strLit, ok := providerNode.(*parse.QuotedStringLiteral); ok {
					s3Provider = strLit.Value
				} else {
					return ".provider should be a quoted string literal"
				}
			} else {
				return "missing .provider in resolution data"
			}

			if bucketNode, hasBucket := n.PropValue("bucket"); hasBucket {
				if _, ok := bucketNode.(*parse.QuotedStringLiteral); !ok {
					return ".bucket should be a quoted string literal"
				}
			} else {
				return "missing .bucket in resolution data"
			}

			if _, hasHost := n.PropValue("host"); !hasHost {
				return "missing .host in resolution data"
			}

			_, hasAccessKey := n.PropValue("access-key")
			secretKeyNode, hasSecretKey := n.PropValue("secret-key")

			if !hasAccessKey {
				if hasSecretKey {
					return "missing .access-key in resolution data"
				}
				if project == nil || reflect.ValueOf(project).IsNil() {
					return "missing .access-key & .secret-key in resolution data"
				}
				ok, err := project.CanProvideS3Credentials(s3Provider)
				if err != nil {
					return fmt.Sprintf("failed to statically check resolution data: %s", err.Error())
				}
				if !ok {
					return "missing .access-key & .secret-key in resolution data (project cannot provide S3 credentials as of now)"
				}
			} else if !hasSecretKey {
				return "missing .secret-key in resolution data"
			}

			if hasSecretKey && parse.NodeIsStringLiteral(secretKeyNode) {
				return ".secret-key should be a secret, not a string"
			}

			return "" //no error
		default:
			return MAIN_ERR_MSG
		}

	})
}

type BucketMapItem = struct {
	WithCredentials, Public *Bucket
}

type Bucket struct {
	closed uint32

	s3Host core.Host //optional
	name   string

	client      *S3Client
	fakeBackend gofakes3.Backend
}

func (b *Bucket) Name() string {
	return b.name
}

func (b *Bucket) Close() {
	if !atomic.CompareAndSwapUint32(&b.closed, 0, 1) {
		return
	}
}

func (b *Bucket) RemoveAllObjects(ctx context.Context) {
	objectChan := b.client.libClient.ListObjects(ctx, b.name, minio.ListObjectsOptions{Recursive: true})
	//note: minio.Client.RemoveObjects does not check the .Error field of channel items

	for range b.client.libClient.RemoveObjects(ctx, b.name, objectChan, minio.RemoveObjectsOptions{}) {
	}
}

type OpenBucketOptions struct {
	//if true and the host resolution data of s3Host is an object without the .access-key & .secret-key properties
	//try to get the .Project of the main state & calls Project.GetS3Credentials.
	AllowGettingCredentialsFromProject bool
}

func OpenBucket(ctx *core.Context, s3Host core.Host, opts OpenBucketOptions) (*Bucket, error) {

	switch s3Host.Scheme() {
	case "https":
		return openPublicBucket(ctx, s3Host, s3Host)
	case "s3":
	default:
		return nil, ErrCannotResolveBucket
	}

	data := ctx.GetHostResolutionData(s3Host)

	switch d := data.(type) {
	case core.Host:
		switch d.Scheme() {
		case "https":
			return openPublicBucket(ctx, s3Host, d)
		}
		return nil, ErrCannotResolveBucket
	case core.URL:
		switch d.Scheme() {
		case "mem":
			return openInMemoryBucket(ctx, s3Host, d)
		}
		return nil, ErrCannotResolveBucket
	case *core.Object:
		var proj core.Project
		mainState := ctx.GetClosestState().MainState
		if mainState != nil {
			proj = mainState.Project
		}
		propNames := d.PropertyNames(ctx)
		credentialsProvided := true

		if !slices.Contains(propNames, "access-key") {
			credentialsProvided = false
			if slices.Contains(propNames, "secret-key") {
				return nil, fmt.Errorf("%w: missing .access-key in resolution data", ErrCannotResolveBucket)
			}
			if !opts.AllowGettingCredentialsFromProject || proj == nil || reflect.ValueOf(proj).IsNil() {
				return nil, fmt.Errorf("%w: missing .access-key & .secret-key in resolution data", ErrCannotResolveBucket)
			}
		} else if !slices.Contains(propNames, "secret-key") {
			return nil, fmt.Errorf("%w: missing .secret-key in resolution data", ErrCannotResolveBucket)
		}

		if !slices.Contains(propNames, "bucket") {
			return nil, fmt.Errorf("%w: missing .bucket in resolution data", ErrCannotResolveBucket)
		}

		if !slices.Contains(propNames, "host") {
			return nil, fmt.Errorf("%w: missing .host in resolution data", ErrCannotResolveBucket)
		}

		if !slices.Contains(propNames, "provider") {
			return nil, fmt.Errorf("%w: missing .provider in resolution data", ErrCannotResolveBucket)
		}

		bucket := d.Prop(ctx, "bucket").(core.StringLike).GetOrBuildString()
		host := d.Prop(ctx, "host").(core.Host)
		provider := d.Prop(ctx, "provider").(core.StringLike).GetOrBuildString()

		var accessKey string
		var secretKey string
		if credentialsProvided {
			accessKey = d.Prop(ctx, "access-key").(core.StringLike).GetOrBuildString()
			secretKey = d.Prop(ctx, "secret-key").(*core.Secret).StringValue().GetOrBuildString()
		} else {
			var err error
			accessKey, secretKey, err = proj.GetS3Credentials(ctx, bucket, provider)

			if err != nil {
				return nil, fmt.Errorf("%w: failed to get S3 credentials from project: %w", ErrCannotResolveBucket, err)
			}
		}

		return OpenBucketWithCredentials(ctx, OpenBucketWithCredentialsInput{
			S3Host:     s3Host,
			HttpsHost:  host,
			BucketName: bucket,
			Provider:   provider,
			AccessKey:  accessKey,
			SecretKey:  secretKey,
		})
	default:
		return nil, ErrCannotResolveBucket
	}
}

func openInMemoryBucket(ctx *core.Context, s3Host core.Host, memURL core.URL) (*Bucket, error) {
	openBucketMapLock.RLock()
	item, ok := openBucketMap[string(memURL)]
	openBucketMapLock.RUnlock()

	if ok && item.WithCredentials != nil {
		return item.WithCredentials, nil
	}

	backend := s3mem.New()
	if err := backend.CreateBucket(string(memURL.Host())); err != nil {
		return nil, err
	}

	bucket := &Bucket{
		s3Host:      s3Host,
		name:        string(memURL.Host()),
		fakeBackend: backend,
	}

	openBucketMapLock.Lock()
	openBucketMap[string(memURL)] = BucketMapItem{WithCredentials: bucket}
	openBucketMapLock.Unlock()

	return bucket, nil
}

func openPublicBucket(ctx *core.Context, s3Host core.Host, httpsHost core.Host) (*Bucket, error) {
	_host := string(httpsHost)

	openBucketMapLock.RLock()
	item, ok := openBucketMap[_host]
	openBucketMapLock.RUnlock()

	if ok && item.Public != nil {
		return item.Public, nil
	}

	subdomain, endpoint, _ := strings.Cut(httpsHost.WithoutScheme(), ".")

	s3Client, err := minio.New(endpoint, &minio.Options{
		Secure: true,
	})

	if err != nil {
		return nil, err
	}

	bucket := &Bucket{
		s3Host: s3Host,
		name:   subdomain,
		client: &S3Client{libClient: s3Client},
	}

	item.Public = bucket
	openBucketMapLock.Lock()
	openBucketMap[_host] = item
	openBucketMapLock.Unlock()

	return bucket, nil
}

type OpenBucketWithCredentialsInput struct {
	S3Host core.Host //optional

	Provider   string
	HttpsHost  core.Host
	BucketName string

	AccessKey, SecretKey string
}

func OpenBucketWithCredentials(ctx *core.Context, input OpenBucketWithCredentialsInput) (*Bucket, error) {
	mapKey := string(input.HttpsHost) + ":" + input.BucketName
	openBucketMapLock.RLock()
	item, ok := openBucketMap[mapKey]
	openBucketMapLock.RUnlock()

	if ok && item.WithCredentials != nil {
		return item.WithCredentials, nil
	}

	var endpoint string
	var region string
	var lookup minio.BucketLookupType

	if input.HttpsHost.Scheme() != "https" {
		return nil, fmt.Errorf("bucket endpoint should have a https:// scheme")
	}

	switch strings.ToLower(input.Provider) {
	case "cloudflare":
		endpoint = input.HttpsHost.WithoutScheme()
		lookup = minio.BucketLookupPath
		region = "auto"
	default:
		return nil, fmt.Errorf("S3 provider %q is not supported", input.Provider)
	}

	s3Client, err := minio.New(endpoint, &minio.Options{
		Region:       region,
		Creds:        credentials.NewStaticV4(input.AccessKey, input.SecretKey, ""),
		Secure:       true,
		BucketLookup: lookup,
	})

	// if input.CreateBucketIfNotExist {
	// 	ok, err := s3Client.BucketExists(ctx, input.BucketName)
	// 	401 unauthorized is returned when checking if a non-existing R2 bucket exists.
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to check if bucket exists: %w", err)
	// 	}
	// 	if !ok {
	// 		err := s3Client.MakeBucket(ctx, input.BucketName, minio.MakeBucketOptions{
	// 			Region: "auto",
	// 		})
	// 		if err != nil {
	// 			return nil, fmt.Errorf("failed to create bucket: %w", err)
	// 		}
	// 		time.Sleep(time.Second)
	// 	}
	// }

	if err != nil {
		return nil, err
	}

	bucket := &Bucket{
		s3Host: input.S3Host,
		name:   input.BucketName,
		client: &S3Client{libClient: s3Client},
	}

	item.WithCredentials = bucket
	openBucketMapLock.Lock()
	openBucketMap[mapKey] = item
	openBucketMapLock.Unlock()

	return bucket, nil
}
