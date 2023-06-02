package parse

import (
	"bytes"
	"fmt"
	"io"

	"github.com/inoxlang/inox/internal/utils"
)

type PrintConfig struct {
	//Compact   bool
	TrimStart bool
}

func Print(node Node, w io.Writer, config PrintConfig) (int, error) {
	tokens := GetTokens(node, false)

	totalN := 0
	if len(tokens) == 0 {
		return 0, nil
	}

	end := 0

	for i, token := range tokens {

		if i > 0 || !config.TrimStart {
			spaceLength := int(token.Span.Start) - end
			space := bytes.Repeat([]byte{' '}, int(spaceLength))
			n, err := w.Write(space)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		n, err := w.Write(utils.StringAsBytes(token.Str()))
		totalN += n
		if err != nil {
			return totalN, err
		}
		end = int(token.Span.End)
	}

	// trailing space

	defer func() {
		if err := recover(); err != nil {
			fmt.Println("!", node, end, tokens)
		}
	}()

	space := bytes.Repeat([]byte{' '}, int(node.Base().Span.End)-end)
	n, err := w.Write(space)
	totalN += n
	if err != nil {
		return totalN, err
	}

	return totalN, nil
}

func SPrint(node Node, config PrintConfig) string {
	buff := bytes.Buffer{}

	Print(node, &buff, config)
	return buff.String()
}
