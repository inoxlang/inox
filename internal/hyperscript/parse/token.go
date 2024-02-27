package parse

type Token struct {
	Type  string `json:"type"` //can be empty
	Value string `json:"value"`

	Start int `json:"start"`
	End   int `json:"end"`

	Line   int `json:"line"`
	Column int `json:"column"`
}
