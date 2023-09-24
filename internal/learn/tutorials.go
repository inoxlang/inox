package learn

import (
	_ "embed"
	"log"

	"github.com/inoxlang/inox/internal/utils"
	"gopkg.in/yaml.v3"
)

var (
	//go:embed tutorials.yaml
	TUTORIALS_YAML string

	tutorials map[string]Tutorial
)

func init() {
	if err := yaml.Unmarshal(utils.StringAsBytes(TUTORIALS_YAML), &tutorials); err != nil {
		log.Panicf("error while parsing tutorials.yaml: %s", err)
	}
}

type Tutorial struct {
	Name              string   `yaml:"name"`
	Program           string   `yaml:"program"`
	ExpectedOutput    []string `yaml:"output"`
	ExpectedLogOutput []string `yaml:"log-output"`
}
