package s3_ns

import (
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
				return MISSING_PROVIDER_IN_RESOL_DATA
			}

			if bucketNode, hasBucket := n.PropValue("bucket"); hasBucket {
				if _, ok := bucketNode.(*parse.QuotedStringLiteral); !ok {
					return ".bucket should be a quoted string literal"
				}
			} else {
				return MISSING_BUCKET_IN_RESOL_DATA
			}

			_, hasHost := n.PropValue("host")
			_, hasAccessKey := n.PropValue("access-key")
			secretKeyNode, hasSecretKey := n.PropValue("secret-key")

			if !hasAccessKey {
				if hasSecretKey {
					return MISSING_ACCESS_KEY_IN_RESOL_DATA
				}

				if hasHost {
					return HOST_SHOULD_NOT_BE_IN_RESOL_DATA_SINCE_CREDS_NOT_PROVIDED
				}

				if project == nil || reflect.ValueOf(project).IsNil() {
					return MISSING_ACCESS_KEY_SECRET_KEY_HOST_IN_RESOL_DATA_NO_PROJ_FOUND
				}

				ok, err := project.CanProvideS3Credentials(s3Provider)
				if err != nil {
					return fmt.Sprintf("failed to statically check resolution data: %s", err.Error())
				}
				if !ok {
					return MISSING_ACCESS_KEY_SECRET_KEY_HOST_IN_RESOL_DATA_PROJ_CANNOT_PROVIDE_CREDS
				}
			} else if !hasSecretKey {
				return MISSING_SECRET_KEY_IN_RESOL_DATA
			} else if !hasHost {
				return MISSING_HOST_IN_RESOL_DATA
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

func (b *Bucket) RemoveAllObjects(ctx *core.Context) {
	objectChan := b.client.ListObjectsLive(ctx, b.name, minio.ListObjectsOptions{Recursive: true})
	//note: minio.Client.RemoveObjects does not check the .Error field of channel items

	for range b.client.RemoveObjects(ctx, b.name, objectChan, minio.RemoveObjectsOptions{}) {
	}
}

type OpenBucketOptions struct {
	//if true and the host resolution data of s3Host is an object without the .access-key & .secret-key properties
	//calls .Project.GetS3CredentialsForBucket.
	// The project is not retrieved from the main state because the context might not have
	// an associated state or the state could be temporary.
	AllowGettingCredentialsFromProject bool

	Project core.Project
}

func OpenBucket(ctx *core.Context, host core.Host, opts OpenBucketOptions) (*Bucket, error) {

	switch host.Scheme() {
	case "https":
		return openPublicBucket(ctx, host, "")
	case "s3":
	default:
		return nil, ErrCannotResolveBucket
	}

	s3Host := host
	data := ctx.GetHostResolutionData(s3Host)

	switch d := data.(type) {
	case core.Host:
		switch d.Scheme() {
		case "https":
			return openPublicBucket(ctx, d, s3Host)
		}
		return nil, ErrCannotResolveBucket
	case core.URL:
		switch d.Scheme() {
		case "mem":
			return openInMemoryBucket(ctx, s3Host, d)
		}
		return nil, ErrCannotResolveBucket
	case *core.Object:
		proj := opts.Project
		propNames := d.PropertyNames(ctx)
		credentialsProvided := true

		hasHost := slices.Contains(propNames, "host")

		if !slices.Contains(propNames, "access-key") {
			credentialsProvided = false
			if slices.Contains(propNames, "secret-key") {
				return nil, fmt.Errorf("%w: missing .access-key in resolution data", ErrCannotResolveBucket)
			}
			if hasHost {
				return nil, fmt.Errorf("%w: %s", ErrCannotResolveBucket, HOST_SHOULD_NOT_BE_IN_RESOL_DATA_SINCE_CREDS_NOT_PROVIDED)
			}
			if !opts.AllowGettingCredentialsFromProject || proj == nil || reflect.ValueOf(proj).IsNil() {
				return nil, fmt.Errorf("%w: %s", ErrCannotResolveBucket, MISSING_ACCESS_KEY_SECRET_KEY_HOST_IN_RESOL_DATA_NO_PROJ_FOUND)
			}

		} else if !slices.Contains(propNames, "secret-key") {
			return nil, fmt.Errorf("%w: missing .secret-key in resolution data", ErrCannotResolveBucket)
		} else if !hasHost {
			return nil, fmt.Errorf("%w: %s", ErrCannotResolveBucket, MISSING_HOST_IN_RESOL_DATA)
		}

		if !slices.Contains(propNames, "bucket") {
			return nil, fmt.Errorf("%w: %s", ErrCannotResolveBucket, MISSING_BUCKET_IN_RESOL_DATA)
		}

		if !slices.Contains(propNames, "provider") {
			return nil, fmt.Errorf("%w: %s", ErrCannotResolveBucket, MISSING_PROVIDER_IN_RESOL_DATA)
		}

		bucket := d.Prop(ctx, "bucket").(core.StringLike).GetOrBuildString()
		provider := d.Prop(ctx, "provider").(core.StringLike).GetOrBuildString()

		var accessKey string
		var secretKey string
		var host core.Host

		if credentialsProvided {
			accessKey = d.Prop(ctx, "access-key").(core.StringLike).GetOrBuildString()
			secretKey = d.Prop(ctx, "secret-key").(*core.Secret).StringValue().GetOrBuildString()
			host = d.Prop(ctx, "host").(core.Host)
		} else { //ask the project to provide the credentials
			var err error
			accessKey, secretKey, host, err = proj.GetS3CredentialsForBucket(ctx, bucket, provider)

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

func openPublicBucket(ctx *core.Context, httpsHost core.Host, optionalS3Host core.Host) (*Bucket, error) {
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
		s3Host: optionalS3Host,
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
