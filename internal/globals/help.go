package internal

import (
	core "github.com/inoxlang/inox/internal/core"
	help "github.com/inoxlang/inox/internal/globals/help"
)

func registerHelp() {
	help.RegisterHelps([]help.TopicHelp{
		{
			//functional
			Value:         core.Map,
			Topic:         "map",
			RelatedTopics: []string{"filter", "some", "all", "none"},
			Text:          "the map function creates a list by applying an operation on each element of an iterable",
			Examples: []help.Example{
				{
					Code:   `map([{name: "foo"}], .name)`,
					Output: `["foo"]`,
				},
				{
					Code:   `map([{a: 1, b: 2, c: 3}], .{a,b})`,
					Output: `[{a: 1, b: 2}]`,
				},
				{
					Code:   `map([0, 1, 2], Mapping{0 => "0" 1 => "1"})`,
					Output: `["0", "1", nil]`,
				},
				{
					Code:   `map([97, 98, 99], torune)`,
					Output: `['a', 'b', 'c']`,
				},
				{
					Code:   `map([0, 1, 2], @($ + 1))`,
					Output: `[1, 2, 3]`,
				},
			},
		},
		{
			Value:         core.Filter,
			Topic:         "filter",
			RelatedTopics: []string{"map", "some", "all", "none"},
			Text:          "the filter function creates a list by iterating over an iterable and keeping elements that pass a condition",
			Examples: []help.Example{
				{
					Code:   `filter(["a", "0", 1], %int)`,
					Output: `[1]`,
				},
				{
					Code:   `filter([0, 1, 2], @($ >= 1))`,
					Output: `[1, 2]`,
				},
			},
		},
		{
			Value:         core.Some,
			Topic:         "some",
			RelatedTopics: []string{"map", "filter", "all", "none"},
			Text:          "the some function returns true if and only if at least one element of an iterable passes a condition. For an empty iterable the result is always true.",
			Examples: []help.Example{
				{
					Code:   `some(["a", "0", 1], %int)`,
					Output: `true`,
				},
				{
					Code:   `some([0, 1, 2], @($ == 'a'))`,
					Output: `false`,
				},
			},
		},
		{
			Value:         core.All,
			Topic:         "all",
			RelatedTopics: []string{"map", "filter", "some", "none"},
			Text:          "the all function returns true if and only if all elements of an iterable pass a condition. For an empty iterable the result is always true.",
			Examples: []help.Example{
				{
					Code:   `all([0, 1, "a"], %int)`,
					Output: `false`,
				},
				{
					Code:   `all([0, 1, 2], @($ >= 0))`,
					Output: `true`,
				},
			},
		},
		{
			Value:         core.None,
			Topic:         "none",
			RelatedTopics: []string{"map", "filter", "some", "all"},
			Text:          "the none function returns true if and only if no elements of an iterable pass a condition. For an emptty iterable the result is always true.",
			Examples: []help.Example{
				{
					Code:   `none([0, 1, "a"], %int)`,
					Output: `false`,
				},
				{
					Code:   `none([0, 1, 2], @($ < 0))`,
					Output: `true`,
				},
			},
		},
		//rand
		{
			Value:         _rand,
			Topic:         "rand",
			RelatedTopics: []string{"pseudo_rand"},
			Text: "the rand function generates/pick a random value in a cryptographically secure way. " +
				"If the argument is a pattern a matching value is returned, if the argument is an indexable an element is picked.",
			Examples: []help.Example{
				{
					Code:   `rand(%int(0..10))`,
					Output: `3`,
				},
				{
					Code:   `rand(%str("a"+))`,
					Output: `"aaaaa"`,
				},
				{
					Code:   `rand(["a", "b"])`,
					Output: `"b"`,
				},
			},
		},
		{
			Value: _find,
			Topic: "find",
			Text:  "the find function searches for items matching a pattern at a given location (a string, an iterable, a directory)",
			Examples: []help.Example{
				{
					Code:   "find %`a+` \"a-aa-aaa\"",
					Output: `["a", "aa", "aaa"]`,
				},
				{
					Code:   `find %./**/*.json ./`,
					Output: `[./file.json, ./dir/file.json, ./dir/dir/.file.json]`,
				},
				{
					Code:   `find %int ['1', 2, "3"]`,
					Output: `[2]`,
				},
			},
		},
	})
}
