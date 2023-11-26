package account

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"text/template"

	"github.com/inoxlang/inox/internal/utils"
)

const (
	PROOF_REPOSITORY_BASE_NAME       = "inox.run-iamhuman"
	UNENCODED_CHALLENGE_VALUE_LENGTH = 6
)

var (
	ErrUnsupportedProofHoster = errors.New("unsupported proof hoster")

	PROOF_HOSTER_NAMES = map[ProofHoster]string{
		Github: "Github",
		Gitlab: "Gitlab",
	}
	PROOF_REPOSITORY_NAME_TEMPLATES = map[ProofHoster]*template.Template{
		Github: parseTemplate("github-repo-name", PROOF_REPOSITORY_BASE_NAME+"-{{.ChallValue}}"),
		Gitlab: parseTemplate("gitlab-repo-name", PROOF_REPOSITORY_BASE_NAME+"-{{.ChallValue}}"),
	}
	PROOF_HOSTER_CHALLENGE_EXPLANATION_TEMPLATES = map[ProofHoster]*template.Template{
		Github: parseTemplate("github-challenge",
			"Go to https://github.com/new?name={{.RepoName}} and click on Create Repository (public). One donce, input your GitHub username (not your display name !)."),
		Gitlab: parseTemplate("gitlab-challenge",
			"Go to https://gitlab.com/projects/new#blank_project and create a public repository named `{{.RepoName}}` "+
				"with a description value of {{.ChallValue}}. Make sure to select your username for the `Project URL` parameter. Once done, input your GitLab username (not your display name !)."),
	}
	PROOF_LOCATION_TEMPLATES = map[ProofHoster]*template.Template{
		Github: parseTemplate("github-api-endpoint", "https://api.github.com/repos/{{.Username}}/{{.RepoName}}"),
		Gitlab: parseTemplate("gitlab-api-endpoint", "https://gitlab.com/api/v4/projects/{{.Username}}%2F{{.RepoName}}"),
	}
)

type ProofHoster int

const (
	Github ProofHoster = iota + 1
	Gitlab
)

func (h ProofHoster) assertValidProofHoster() {
	if !(h >= Github && h <= Gitlab) {
		panic(fmt.Errorf("invalid integer representation of a proof hoster: %d", h))
	}
}

func (h ProofHoster) String() string {
	h.assertValidProofHoster()
	return PROOF_HOSTER_NAMES[h]
}

func getProofHosterByName(name string) (ProofHoster, error) {
	for provider, providerName := range PROOF_HOSTER_NAMES {
		if providerName == name {
			provider.assertValidProofHoster()
			return provider, nil
		}
	}
	return -1, ErrUnsupportedProofHoster
}

type ProofRepoNameTemplateContext struct {
	ChallValue string
}

type challengeTemplateContext struct {
	RepoName   string
	ChallValue string
}

type proofLocationTemplateContext struct {
	Username string
	RepoName string
}

func randomChallengeValue() (string, error) {
	value := [UNENCODED_CHALLENGE_VALUE_LENGTH]byte{}
	_, err := rand.Read(value[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func parseTemplate(name string, templ string) *template.Template {
	return utils.Must(template.New(name).Parse(templ))
}
