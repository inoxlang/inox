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
	PROOF_REPOSITORY_NAME            = "inox.run-challenge"
	UNENCODED_CHALLENGE_VALUE_LENGTH = 32
)

var (
	ErrUnsupportedProofHoster = errors.New("unsupported proof hoster")

	PROOF_HOSTER_NAMES = map[ProofHoster]string{
		Github: "Github",
	}
	PROOF_HOSTER_CHALLENGE_EXPLANATION_TEMPLATES = map[ProofHoster]*template.Template{
		Github: parseTemplate("github-challenge",
			"Go on https://github.com/new and create a public repository named `"+PROOF_REPOSITORY_NAME+
				"` with a description value of {{.ChallValue}}. Then input your Github username (not your display name !)."),
	}
	PROOF_LOCATION_TEMPLATES = map[ProofHoster]*template.Template{
		Github: parseTemplate("github-api-endpoint", "https://api.github.com/repos/{{.Username}}/"+PROOF_REPOSITORY_NAME),
	}
)

type ProofHoster int

const (
	Github ProofHoster = iota + 1
)

func (h ProofHoster) assertValidProofHoster() {
	if !(h >= Github && h <= Github) {
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

type challengeTemplateContext struct {
	ChallValue string
}

type proofLocationTemplateContext struct {
	Username string
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
