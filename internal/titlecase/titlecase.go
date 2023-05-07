// Titlecase package capitalizes all words in the string to Title Caps attempting to
// be smart about small words like a/an/the.
//
// These small words will also be uncapitalized, so titlecase also
// works with uppercase strings.
//
// The list of small words which are not capped comes from the New York
// Times Manual of Style, plus 'vs' and 'v'.
//
// Titlecase is a port of Python's titlecase module in Go.
//
// Thanks to Stuart Colville for the Python version: https://pypi.python.org/pypi/titlecase.
// And John Gruber for the original version in Perl: http://daringfireball.net/2008/05/title_case.
//
package titlecase

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	punct = "!\"#$%&'‘()*+,\\-./:;?@[\\]_`{|}~"
	small = `a|an|and|as|at|but|by|en|for|if|in|of|on|or|the|to|v\.?|via|vs\.?`

	reLineBreak          = regexp.MustCompile("[\r\n]+")
	reWhiteSpace         = regexp.MustCompile("[\t ]+")
	reSmallWords         = regexp.MustCompile(fmt.Sprintf("^(?i:%s)$", small))
	reSmallFirst         = regexp.MustCompile(fmt.Sprintf("^([%s]*)(?i:(%s))\\b", punct, small))
	reSmallLast          = regexp.MustCompile(fmt.Sprintf("\\b(?i:(%s))[%s]?$", small, punct))
	reMacMc              = regexp.MustCompile(`^([Mm]a?c)(\w+)`)
	reInlinePeriod       = regexp.MustCompile(`(?i:[a-z][.][a-z])`)
	reCapFirst           = regexp.MustCompile(fmt.Sprintf("^[%s]*?([A-Za-z\\pL])", punct))
	reSubphrase          = regexp.MustCompile(fmt.Sprintf("([:.;?!][ ])(?i:(%s))", small))
	reAllCaps            = regexp.MustCompile(fmt.Sprintf("^[A-Z\\s%s]+$", punct))
	reUpperCaseInitials  = regexp.MustCompile("^(?:[A-Z]{1}\\.{1}|[A-Z]{1}\\.{1}[A-Z]{1})+$")
	reUpperCaseElsewhere = regexp.MustCompile(fmt.Sprintf("[%s]*?[a-zA-Z]+[A-Z]+?", punct))
	reApostropheSecond   = regexp.MustCompile("^(?i:([dol]{1})['‘]{1}([a-z]+))$")
)

func Title(s string) string {

	lines := reLineBreak.Split(s, -1)
	newLines := make([]string, 0, len(lines))

	for _, line := range lines {
		words := reWhiteSpace.Split(line, -1)
		newWords := make([]string, 0, len(words))
		allCaps := reAllCaps.MatchString(line)

		for _, word := range words {

			if allCaps {
				if reUpperCaseInitials.MatchString(word) {
					newWords = append(newWords, word)
					continue
				}
				word = strings.ToLower(word)
			}

			if reApostropheSecond.MatchString(word) {
				newWords = append(newWords, reApostropheSecond.ReplaceAllStringFunc(word, strings.Title))
				continue
			}

			if reInlinePeriod.MatchString(word) || reUpperCaseElsewhere.MatchString(word) {
				newWords = append(newWords, word)
				continue
			}

			if reSmallWords.MatchString(word) {
				newWords = append(newWords, strings.ToLower(word))
				continue
			}

			if match := reMacMc.FindStringSubmatch(word); match != nil {
				newWords = append(newWords, fmt.Sprintf("%s%s",
					strings.Title(match[1]),
					strings.Title(match[2]),
				))
				continue
			}

			if strings.Contains(word, "/") && !strings.Contains(word, "//") {
				ws := make([]string, 0)
				for _, w := range strings.Split(word, "/") {
					ws = append(ws, reCapFirst.ReplaceAllStringFunc(w, strings.Title))
				}
				newWords = append(newWords, strings.Join(ws, "/"))
				continue
			}

			if strings.Contains(word, "-") {
				ws := make([]string, 0)
				for _, w := range strings.Split(word, "-") {
					ws = append(ws, reCapFirst.ReplaceAllStringFunc(w, strings.Title))
				}
				newWords = append(newWords, strings.Join(ws, "-"))
				continue
			}

			newWords = append(newWords, reCapFirst.ReplaceAllStringFunc(word, strings.ToTitle))
		}

		newLine := strings.Join(newWords, " ")

		newLine = reSmallFirst.ReplaceAllStringFunc(newLine, strings.Title) // wrong? regex has two capture groups
		newLine = reSmallLast.ReplaceAllStringFunc(newLine, strings.Title)
		newLine = reSubphrase.ReplaceAllStringFunc(newLine, strings.Title)

		newLines = append(newLines, newLine)
	}

	return strings.Join(newLines, "\n")
}
