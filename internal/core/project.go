package core

type Project interface {
	GetS3Credentials(ctx *Context, bucketName string, provider string) (accessKey, secretKey string, _ error)
}
