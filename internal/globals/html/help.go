package internal

import (
	"strconv"

	help "github.com/inox-project/inox/internal/globals/help"
)

func registerHelp() {
	for i, fn := range []any{_h1, _h2, _h3, _h4} {
		elemName := "h" + strconv.Itoa(i+1)
		name := "html." + elemName
		help.RegisterHelp(help.TopicHelp{
			Value: fn,
			Topic: name,
			Text:  "the " + name + " function creates a " + elemName + " HTML element",
		})
	}
}
