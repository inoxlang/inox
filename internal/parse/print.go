package parse

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/inoxlang/inox/internal/utils"
)

type PrintConfig struct {
	//Compact   bool
	KeepLeadingSpace  bool
	KeepTrailingSpace bool
	CacheResult       bool
}

// Print prints $node to $w and returns the number of written bytes.
func Print(node Node, chunk *Chunk, w io.Writer, config PrintConfig) (int, error) {
	tokens := GetTokens(node, chunk, false)
	singleSpace := []byte{' '}

	totalN := 0
	if len(tokens) == 0 {
		return 0, nil
	}

	end := 0
	isLeadingRegularSpacePrinted := false
	isLeadingNewline := true

	for i, token := range tokens {
		if token.Type != NEWLINE {
			isLeadingNewline = false
		}

		if config.KeepLeadingSpace && !isLeadingRegularSpacePrinted {
			//print regular leading space.
			isLeadingRegularSpacePrinted = true
			spaceLength := int(token.Span.Start) - end
			space := bytes.Repeat(singleSpace, int(spaceLength))
			n, err := w.Write(space)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		if i > 0 && !isLeadingNewline && (!config.KeepLeadingSpace || isLeadingRegularSpacePrinted) {
			//print space before token.
			spaceLength := int(token.Span.Start) - end
			space := bytes.Repeat(singleSpace, int(spaceLength))
			n, err := w.Write(space)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		if token.Type == NEWLINE && isLeadingNewline && !config.KeepLeadingSpace {
			//do not print leading newline.
			end = int(token.Span.End)
			continue
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

	if config.KeepTrailingSpace {
		space := bytes.Repeat(singleSpace, int(node.Base().Span.End)-end)
		n, err := w.Write(space)
		totalN += n
		if err != nil {
			return totalN, err
		}
	}

	return totalN, nil
}

// SPrint stringifies $node.
func SPrint(node Node, chunk *Chunk, config PrintConfig) string {
	buff := bytes.Buffer{}

	Print(node, chunk, &buff, config)
	return buff.String()
}

func PrintPath[S ~string](w io.Writer, path S) (int, error) {
	s := string(path)

	quote := ContainsSpace(s) || strings.ContainsFunc(s, IsDelim)
	if quote {
		var b []byte

		i := strings.Index(s, "/")
		b = append(b, path[:i+1]...)
		b = append(b, '`')
		b = append(b, path[i+1:]...)
		b = append(b, '`')

		return w.Write(b)
	} else {
		return w.Write(utils.StringAsBytes(path))
	}
}

func PrintPathPattern[S ~string](w io.Writer, path S) (totalN int, err error) {
	b := [1]byte{'%'}

	var n int

	totalN, err = w.Write(b[:])
	if err != nil {
		return
	}

	if path[0] == '%' {
		path = path[1:]
	}

	s := string(path)

	quote := ContainsSpace(s) || strings.ContainsFunc(s, IsDelim)

	if quote {
		var b []byte

		i := strings.Index(s, "/")
		b = append(b, s[:i+1]...)
		b = append(b, '`')
		b = append(b, s[i+1:]...)
		b = append(b, '`')

		n, err = w.Write(b)
		totalN += n
		return
	} else {
		n, err = w.Write(utils.StringAsBytes(s))
		totalN += n
		return
	}
}
