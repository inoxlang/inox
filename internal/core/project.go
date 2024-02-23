package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/inoxlang/inox/internal/parse"
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

	Configuration() ProjectConfiguration

	//DevDatabasesDirOnOsFs returns a directory in the OS filesystem where some development databases can be stored.
	//The directory is dedidcated to a single project member.
	DevDatabasesDirOnOsFs(ctx *Context, memberAuthToken string) (string, error)
}

type ProjectID string

func RandomProjectID(projectName string) ProjectID {
	return ProjectID(projectName + "-" + ulid.Make().String())
}

func (id ProjectID) Validate() error {
	s := string(id)

	for _, r := range s {
		if unicode.IsSpace(r) {
			return errors.New("unexpected space(s) in project ID")
		}
	}

	lastDashIndex := strings.LastIndex(s, "-")
	if lastDashIndex < 0 {
		return errors.New("missing `-<ULID>` (e.g. `-01HPWQNC2Q6Y8NJKWR24TJK9NE`) at end of project ID")
	}
	if lastDashIndex == len(s)-1 {
		return errors.New("missing <ULID> (e.g. `01HPWQNC2Q6Y8NJKWR24TJK9NE`) after last `-` in project ID")
	}

	if lastDashIndex == 0 {
		return errors.New("missing name before `-` in project ID")
	}

	projectULID := s[lastDashIndex+1:]
	_, err := ulid.ParseStrict(projectULID)
	if err != nil {
		return errors.New("invalid ULID in project ID")
	}

	return nil
}

type ProjectConfiguration interface {
	AreExposedWebServersAllowed() bool
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
