package learn

import (
	_ "embed"
	"log"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/inoxlang/inox/internal/project/scaffolding"
	"github.com/inoxlang/inox/internal/utils"
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

	for _, series := range TUTORIAL_SERIES {
		for _, tut := range series.Tutorials {
			for name, content := range tut.OtherFiles {
				if content != "" {
					continue
				}

				basename := filepath.Base(name)
				switch basename {
				case "main.css":
					tut.OtherFiles[name] = scaffolding.MAIN_CSS_STYLESHEET
				case "htmx.min.js":
					tut.OtherFiles[name] = scaffolding.HTMX_MIN_JS_PACKAGE
				}
			}
		}
	}
}

type TutorialSeries struct {
	Id          string     `yaml:"id" json:"id"`
	Name        string     `yaml:"name" json:"name"`
	Tutorials   []Tutorial `yaml:"tutorials" json:"tutorials"`
	Description string     `yaml:"description" json:"description"`
}

type Tutorial struct {
	Id                string            `yaml:"id" json:"id"`
	Name              string            `yaml:"name" json:"name"`
	Program           string            `yaml:"program" json:"program"`
	OtherFiles        map[string]string `yaml:"other-files" json:"otherFiles,omitempty"`
	ExpectedOutput    []string          `yaml:"output" json:"output,omitempty"`
	ExpectedLogOutput []string          `yaml:"log-output" json:"logOutput,omitempty"`
}

func ListTutorials() (tutorials []Tutorial) {
	for _, series := range TUTORIAL_SERIES {
		tutorials = append(tutorials, series.Tutorials...)
	}

	return tutorials
}
