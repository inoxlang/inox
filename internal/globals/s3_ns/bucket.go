package s3_ns

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

var (
	ErrCannotResolveBucket = errors.New("cannot resolve bucket")

	//ResourceName -> Bucket
	openBucketMap     = make(map[string]BucketMapItem)
	openBucketMapLock sync.RWMutex
)

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

func OpenBucket(ctx *core.Context, s3Host core.Host) (*Bucket, error) {

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
		bucket := d.Prop(ctx, "bucket")
		if bucket == nil {
			return nil, fmt.Errorf("%w: missing .bucket in resolution data", ErrCannotResolveBucket)
		}

		host := d.Prop(ctx, "host")
		if host == nil {
			return nil, fmt.Errorf("%w: missing .host in resolution data", ErrCannotResolveBucket)
		}

		provider := d.Prop(ctx, "provider")
		if bucket == nil {
			return nil, fmt.Errorf("%w: missing .provider in resolution data", ErrCannotResolveBucket)
		}

		accessKey := d.Prop(ctx, "access-key")
		if accessKey == nil {
			return nil, fmt.Errorf("%w: missing .access-key in resolution data", ErrCannotResolveBucket)
		}

		secretKey := d.Prop(ctx, "secret-key")

		if secretKey == nil {
			return nil, fmt.Errorf("%w: missing .secret-key in resolution data", ErrCannotResolveBucket)
		}

		return OpenBucketWithCredentials(ctx, OpenBucketWithCredentialsInput{
			S3Host:     s3Host,
			HttpsHost:  host.(core.Host),
			BucketName: bucket.(core.StringLike).GetOrBuildString(),
			Provider:   provider.(core.StringLike).GetOrBuildString(),
			AccessKey:  accessKey.(core.StringLike).GetOrBuildString(),
			SecretKey:  secretKey.(core.StringLike).GetOrBuildString(),
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
	_host := string(input.HttpsHost)
	openBucketMapLock.RLock()
	item, ok := openBucketMap[_host]
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
	openBucketMap[_host] = item
	openBucketMapLock.Unlock()

	return bucket, nil
}
