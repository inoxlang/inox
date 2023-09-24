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

	TUTORIAL_SERIES []TutorialSeries
)

func init() {
	if err := yaml.Unmarshal(utils.StringAsBytes(TUTORIALS_YAML), &TUTORIAL_SERIES); err != nil {
		log.Panicf("error while parsing tutorials.yaml: %s", err)
	}
}

type TutorialSeries struct {
	Id         string     `yaml:"id" json:"id"`
	Name       string     `yaml:"name" json:"name"`
	Tutorials  []Tutorial `yaml:"tutorials" json:"tutorials"`
	Descriptio string     `yaml:"description" json:"description"`
}

type Tutorial struct {
	Id                string   `yaml:"id" json:"id"`
	Name              string   `yaml:"name" json:"name"`
	Program           string   `yaml:"program" json:"program"`
	ExpectedOutput    []string `yaml:"output" json:"output"`
	ExpectedLogOutput []string `yaml:"log-output" json:"logOutput"`
}

func ListTutorials() (tutorials []Tutorial) {
	for _, series := range TUTORIAL_SERIES {
		tutorials = append(tutorials, series.Tutorials...)
	}

	return tutorials
}
