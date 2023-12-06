package binary

const (
	INOX_BINARY_PATH             = "/usr/local/bin/inox"
	OLDINOX_BINARY_PATH          = "/usr/local/bin/oldinox"
	TEMPINOX_BINARY_PATH         = "/usr/local/bin/tempinox"
	INOX_REPO_API_ENDPOINT       = "https://api.github.com/repos/inoxlang/inox"
	REPO_TAGS_API_ENDPOINT       = INOX_REPO_API_ENDPOINT + "/tags"
	RELEASE_BY_TAG_API_ENDPOINT  = INOX_REPO_API_ENDPOINT + "/releases/tags"
	BINARY_ARCHIVE_SHA256_PREFIX = ".sha256"

	INOX_BINARY_PERMS = 0o755

	MAX_INOX_BINARY_SIZE = 150_000_000
)
