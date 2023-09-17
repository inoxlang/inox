package s3_ns

const (
	MISSING_ACCESS_KEY_IN_RESOL_DATA                                           = "missing .access-key in resolution data"
	MISSING_SECRET_KEY_IN_RESOL_DATA                                           = "missing .secret-key in resolution data"
	MISSING_ACCESS_KEY_SECRET_KEY_HOST_IN_RESOL_DATA_NO_PROJ_FOUND             = "missing .access-key, .secret-key and .host in resolution data (no project found to provide credentials)"
	MISSING_ACCESS_KEY_SECRET_KEY_HOST_IN_RESOL_DATA_PROJ_CANNOT_PROVIDE_CREDS = "missing .access-key, .secret-key and .host in resolution data (project not able to provide credentials for the given provider)"

	MISSING_HOST_IN_RESOL_DATA                                = "missing .host in resolution data"
	HOST_SHOULD_NOT_BE_IN_RESOL_DATA_SINCE_CREDS_NOT_PROVIDED = ".host should not be in resolution data since it is provided by the project"

	MISSING_BUCKET_IN_RESOL_DATA   = "missing .bucket in resolution data"
	MISSING_PROVIDER_IN_RESOL_DATA = "missing .provider in resolution data"
)
