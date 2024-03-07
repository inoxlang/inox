package htmx

import "strings"

var HEADERS struct {
	Request  map[string]HeaderInfo `yaml:"request-headers"`
	Response map[string]HeaderInfo `yaml:"response-headers"`
}

type HeaderInfo struct {
	Name             string `yaml:"-"`
	ShortExplanation string `yaml:"short-explanation,omitempty"`
	Documentation    string `yaml:"documentation,omitempty"`
}

func GetResponseHeadersByPrefix(prefix string, strict bool) (headers []HeaderInfo) {

	for name, header := range HEADERS.Response {
		//Includes the header if $prefix is a prefix of $name, regardless of case sensitivty.
		//Also includes the header if $name is a prefix of $prefix and $strict is false.
		nameStart := name[:min(len(prefix), len(name))]
		usedPrefix := prefix

		if len(prefix) > len(nameStart) {
			if strict {
				continue
			}
			usedPrefix = prefix[:len(nameStart)]
		}

		if strings.EqualFold(nameStart, usedPrefix) {
			headers = append(headers, header)
		}
	}
	return
}
