package sourcecode

type ParsingError struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func (err ParsingError) Error() string {
	return err.Message
}

type ParsingErrorAggregation struct {
	Message        string          `json:"completeMessage"`
	Errors         []*ParsingError `json:"errors"`
	ErrorPositions []PositionRange `json:"errorPositions"`
}

func (err ParsingErrorAggregation) Error() string {
	return err.Message
}
