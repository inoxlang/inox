package github

const (
	REPO_API_ENDPOINT_TEMPL          = "https://api.github.com/repos/{repo}"
	REPO_TAGS_API_ENDPOINT_TMPL      = REPO_API_ENDPOINT_TEMPL + "/tags"
	RELEASE_BY_TAG_API_ENDPOINT_TMPL = REPO_API_ENDPOINT_TEMPL + "/releases/tags"
)
