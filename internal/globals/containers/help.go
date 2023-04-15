package internal

import help "github.com/inoxlang/inox/internal/globals/help"

func registerHelp() {
	help.RegisterHelps([]help.TopicHelp{
		{
			Value:         NewGraph,
			Topic:         "Graph",
			RelatedTopics: []string{"Tree"},
			Text:          "the Graph function creates a directed Graph",
		},
		{
			Value:         NewTree,
			Topic:         "Tree",
			RelatedTopics: []string{"Graph"},
			Text:          "the Tree function creates a tree from a udata value",
			Examples: []help.Example{
				{
					Code:        `Tree(udata "root")`,
					Explanation: "creates a Tree with a single node",
				},
				{
					Code:        `Tree(udata "root" { "child" })`,
					Explanation: "creates a Tree with a root + single child",
				},
			},
		},
		{
			Value: NewStack,
			Topic: "Stack",
			Text:  "the Stack function creates a stack from an iterable",
			Examples: []help.Example{
				{
					Code:        `Stack([])`,
					Explanation: "creates an empty stack",
				},
				{
					Code:        `Stack([1])`,
					Explanation: "creates an stack with an element 1",
				},
			},
		},
		{
			Value: NewQueue,
			Topic: "Queue",
			Text:  "the Queue function creates a queue from an iterable",
			Examples: []help.Example{
				{
					Code:        `Queue([])`,
					Explanation: "creates an empty queue",
				},
				{
					Code:        `Queue([1])`,
					Explanation: "creates a queue with an element 1",
				},
			},
		},
		{
			Value: NewSet,
			Topic: "Set",
			Text:  "the Set function creates a set from an iterable, only representable values are allowed",
			Examples: []help.Example{
				{
					Code:        `Set([])`,
					Explanation: "creates an empty set",
				},
				{
					Code:        `Set([1, 1])`,
					Explanation: "creates a queue with an element 1",
				},
				{
					Code:        `Set([1, 1])`,
					Explanation: "creates a queue with an element 1",
				},
			},
		},
		{
			Value: NewMap,
			Topic: "Map",
			Text:  "the Map function creates a map from a list of flat entries",
			Examples: []help.Example{
				{
					Code:        `Map(["key1", 10, "key2", 20])`,
					Explanation: `creates a Map with the entries "key1" -> 10, "key2" -> 20`,
				},
			},
		},
		{
			Value: NewRanking,
			Topic: "Ranking",
			Text: "the Ranking function creates a ranking from a list of flat entries. " +
				"En entry is composed of a value and a floating-point score. The value with the highest score has the first rank (0), values with the same score have the same rank.",
			Examples: []help.Example{
				{
					Code:        `Ranking(["best player", 10.0, "other player", 5.0])`,
					Explanation: `creates a Ranking with the following ranks: rank(0) -> "best player", rank(1) -> "other player"`,
				},
				{
					Code:        `Ranking(["best player", 10.0, "other player", 10.0])`,
					Explanation: `creates a Ranking with the following ranks: rank(0) -> "best player" & "other player"`,
				},
			},
		},
		{
			Value: NewThread,
			Topic: "Thread",
			Text:  "the Thread function creates a thread from an iterable.",
			Examples: []help.Example{
				{
					Code: `Thread([{message: "hello", author_id: "5958"}])`,
				},
			},
		},
	})
}
