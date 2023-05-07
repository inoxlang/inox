package titlecase

import (
	"regexp"
	"testing"
)

var tests = [][]string{
	[]string{
		"word/word",
		"Word/Word",
	},
	[]string{
		"dance with me/let’s face the music and dance",
		"Dance With Me/Let’s Face the Music and Dance",
	},
	[]string{
		"34th 3rd 2nd",
		"34th 3rd 2nd",
	},
	[]string{
		"Q&A with steve jobs: 'that's what happens in technology'",
		"Q&A With Steve Jobs: 'That's What Happens in Technology'",
	},
	[]string{
		"What is AT&T's problem?",
		"What Is AT&T's Problem?",
	},
	[]string{
		"Apple deal with AT&T falls through",
		"Apple Deal With AT&T Falls Through",
	},
	[]string{
		"this v that",
		"This v That",
	},
	[]string{
		"this v. that",
		"This v. That",
	},
	[]string{
		"this vs that",
		"This vs That",
	},
	[]string{
		"this vs. that",
		"This vs. That",
	},
	[]string{
		"The SEC's Apple probe: what you need to know",
		"The SEC's Apple Probe: What You Need to Know",
	},
	[]string{
		"'by the Way, small word at the start but within quotes.'",
		"'By the Way, Small Word at the Start but Within Quotes.'",
	},
	[]string{
		"Small word at end is nothing to be afraid of",
		"Small Word at End Is Nothing to Be Afraid Of",
	},
	[]string{
		"Starting Sub-Phrase With a Small Word: a Trick, Perhaps?",
		"Starting Sub-Phrase With a Small Word: A Trick, Perhaps?",
	},
	[]string{
		"Sub-Phrase With a Small Word in Quotes: 'a Trick, Perhaps?'",
		"Sub-Phrase With a Small Word in Quotes: 'A Trick, Perhaps?'",
	},
	[]string{
		`sub-phrase with a small word in quotes: "a trick, perhaps?"`,
		`Sub-Phrase With a Small Word in Quotes: "A Trick, Perhaps?"`,
	},
	[]string{
		`"Nothing to Be Afraid of?"`,
		`"Nothing to Be Afraid Of?"`,
	},
	[]string{
		`"Nothing to be Afraid Of?"`,
		`"Nothing to Be Afraid Of?"`,
	},
	[]string{
		`a thing`,
		`A Thing`,
	},
	[]string{
		"2lmc Spool: 'gruber on OmniFocus and vapo(u)rware'",
		"2lmc Spool: 'Gruber on OmniFocus and Vapo(u)rware'",
	},
	[]string{
		`this is just an example.com`,
		`This Is Just an example.com`,
	},
	[]string{
		`this is something listed on del.icio.us`,
		`This Is Something Listed on del.icio.us`,
	},
	[]string{
		`iTunes should be unmolested`,
		`iTunes Should Be Unmolested`,
	},
	[]string{
		`reading between the lines of steve jobs’s ‘thoughts on music’`,
		`Reading Between the Lines of Steve Jobs’s ‘Thoughts on Music’`,
	},
	[]string{
		`seriously, ‘repair permissions’ is voodoo`,
		`Seriously, ‘Repair Permissions’ Is Voodoo`,
	},
	[]string{
		`generalissimo francisco franco: still dead; kieren McCarthy: still a jackass`,
		`Generalissimo Francisco Franco: Still Dead; Kieren McCarthy: Still a Jackass`,
	},
	[]string{
		"O'Reilly should be untouched",
		"O'Reilly Should Be Untouched",
	},
	[]string{
		"my name is o'reilly",
		"My Name Is O'Reilly",
	},
	[]string{
		"WASHINGTON, D.C. SHOULD BE FIXED BUT MIGHT BE A PROBLEM",
		"Washington, D.C. Should Be Fixed but Might Be a Problem",
	},
	[]string{
		"THIS IS ALL CAPS AND SHOULD BE ADDRESSED",
		"This Is All Caps and Should Be Addressed",
	},
	[]string{
		"Mr McTavish went to MacDonalds",
		"Mr McTavish Went to MacDonalds",
	},
	[]string{
		"this shouldn't\nget mangled",
		"This Shouldn't\nGet Mangled",
	},
	[]string{
		"this is http://foo.com",
		"This Is http://foo.com",
	},
	[]string{
		"é mesmo",
		"É Mesmo",
	},
}

func TestTitle(t *testing.T) {
	for _, d := range tests {
		in := d[0]
		out := d[1]
		res := Title(in)
		if res != out {
			t.Errorf("Invalid title for %+q. Got %+q, want %+q", in, res, out)
		}
	}
}

type regexTestGroup struct {
	re    *regexp.Regexp
	tests []regexTest
}

type regexTest struct {
	input string
	match bool
}

var regexTests = []regexTestGroup{
	regexTestGroup{
		reLineBreak,
		[]regexTest{
			{"\n", true}, {"\r\n", true},
			{" ", false}, {"\t", false},
		},
	},
	regexTestGroup{
		reWhiteSpace,
		[]regexTest{
			{"\t", true}, {" ", true}, {"  ", true}, {"\t ", true},
			{"\n", false}, {"\r\n", false},
		},
	},
	regexTestGroup{
		reAllCaps,
		[]regexTest{
			{"hi", false}, {"Hi", false},
			{"HI", true}, {"HI + GO", true},
		},
	},
	regexTestGroup{
		reSmallWords,
		[]regexTest{
			{"a", true}, {"if", true}, {"v", true}, {"v.", true},
			{"vs", true}, {"vs.", true}, {"BY", true}, {"By", true},
			{"not", false}, {"lua", false},
		},
	},
	regexTestGroup{
		reSmallFirst,
		[]regexTest{
			{"a table", true}, {"the car", true}, {"The language", true},
			{"yes. at my house", false},
		},
	},
	regexTestGroup{
		reSmallLast,
		[]regexTest{
			{"afraid of", true}, {"afraid of?", true},
			{"a house", false},
		},
	},
	regexTestGroup{
		reMacMc,
		[]regexTest{
			{"macdonalds", true}, {"MacDonalds", true},
			{"mccarthy", true}, {"McCarthy", true},
			{"sommacbar", false}, {"somemcbar", false},
		},
	},
	regexTestGroup{
		reInlinePeriod,
		[]regexTest{
			{"hello example.com", true},
			{"del.icio.us website", true},
			{"hi go lang.", false},
		},
	},
	regexTestGroup{
		reCapFirst,
		[]regexTest{
			{"a thing", true}, {"ops. a thing", true}, {"jobs’s", true},
		},
	},
	regexTestGroup{
		reSubphrase,
		[]regexTest{
			{"listen: to work!", true},
			{"subphase; a trick '", true},
			{"subphase? a trick '", true},
			{"subphase. a trick '", true},
			{"subphase! A trick '", true},
		},
	},
	regexTestGroup{
		reAllCaps,
		[]regexTest{
			{"HI THERE", true}, {"ALL, CAPS!", true},
			{"nope", false},
		},
	},
	regexTestGroup{
		reUpperCaseInitials,
		[]regexTest{
			{"D.C", true}, {"B.", true}, {"I.G.O.R.", true},
			{"d.c", false},
		},
	},
	regexTestGroup{
		reUpperCaseElsewhere,
		[]regexTest{
			{"iTunes", true}, {"Something", false},
		},
	},
	regexTestGroup{
		reApostropheSecond,
		[]regexTest{
			{"o'reilly", true}, {"O'Reilly", true}, {"o‘reilly", true},
			{"d'wayne", true}, {"l'orange", true}, //{"L'Oréal", true}, FIXME: doesn't work
			{"ops'nops", false},
		},
	},
}

func TestRegexes(t *testing.T) {
	for _, tg := range regexTests {
		for _, tt := range tg.tests {
			got := tg.re.MatchString(tt.input)
			if got != tt.match {
				t.Errorf("Regexp %q with %q expected %v got %v", tg.re, tt.input, tt.match, got)
			}
		}
	}
}
