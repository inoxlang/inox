package htmx

var HEADERS struct {
	Request  map[string]HeaderInfo `yaml:"request-headers"`
	Response map[string]HeaderInfo `yaml:"response-headers"`
}

type HeaderInfo struct {
	ShortExplanation string `yaml:"short-explanation,omitempty"`
	Documentation    string `yaml:"documentation,omitempty"`
}
