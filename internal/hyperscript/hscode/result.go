package hscode

type ParsingResult struct {
	//Node               Node    `json:"node"`
	NodeData           map[string]any `json:"nodeData"` //set by the JS-based parser. May be not set for perf reasons.
	Tokens             []Token        `json:"tokens"`
	TokensNoWhitespace []Token        `json:"tokensNoWhitespace"`
}

type ParsingError struct {
	Message        string  `json:"message"`
	MessageAtToken string  `json:"messageAtToken"`
	Token          Token   `json:"token"`
	Tokens         []Token `json:"tokens"`
}

func (e ParsingError) Error() string {
	return e.Message
}
