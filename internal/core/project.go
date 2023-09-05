package core

type Project interface {
	//CanProvideS3Credentials should return true if the project can provide S3 credentials for
	//the given S3 provider AT THE MOMENT OF THE CALL. If the error is not nil the boolean result
	//should be false.
	CanProvideS3Credentials(s3Provider string) (bool, error)

	GetS3Credentials(ctx *Context, bucketName string, provider string) (accessKey, secretKey string, _ error)
}
