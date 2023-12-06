package core

import (
	"fmt"
	"time"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/oklog/ulid/v2"
)

type Project interface {
	Id() ProjectID

	BaseImage() (Image, error)

	ListSecrets(ctx *Context) ([]ProjectSecretInfo, error)

	GetSecrets(ctx *Context) ([]ProjectSecret, error)

	//CanProvideS3Credentials should return true if the project can provide S3 credentials for
	//the given S3 provider AT THE MOMENT OF THE CALL. If the error is not nil the boolean result
	//should be false.
	CanProvideS3Credentials(s3Provider string) (bool, error)

	// GetS3CredentialsForBucket creates the bucket bucketName if necessary & returns credentials to access it,
	// the returned credentials should not work for other buckets.
	GetS3CredentialsForBucket(ctx *Context, bucketName string, provider string) (accessKey, secretKey string, s3Endpoint Host, _ error)
}

type ProjectID string

func RandomProjectID(projectName string) ProjectID {
	return ProjectID(projectName + "-" + ulid.Make().String())
}

type ProjectSecret struct {
	Name          SecretName
	LastModifDate time.Time
	Value         *Secret
}

type ProjectSecretInfo struct {
	Name          SecretName `json:"name"`
	LastModifDate time.Time  `json:"lastModificationDate"`
}

type SecretName string

func SecretNameFrom(name string) (SecretName, error) {
	for _, r := range name {
		if !parse.IsIdentChar(r) {
			return "", fmt.Errorf("invalid char found in secret's name: '%c'", r)
		}
	}
	return SecretName(name), nil
}
