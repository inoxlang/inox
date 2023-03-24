package internal

import help "github.com/inox-project/inox/internal/globals/help"

func registerHelp() {
	help.RegisterHelps([]help.TopicHelp{
		{
			Topic:         "chrome",
			RelatedTopics: []string{"chome.Handle"},
			Text:          "chrome namespace",
		},
	})

	help.RegisterHelps([]help.TopicHelp{
		{
			Value: NewHandle,
			Topic: "chrome.Handle",
			Text:  "the Handle function creates a new Chrome handle",
			Examples: []help.Example{
				{
					Code: `chrome.Handle!()`,
				},
			},
		},
	})
}
