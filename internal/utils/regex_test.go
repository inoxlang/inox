package utils

import (
	"regexp"
	"regexp/syntax"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegexForRange(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	t.Run("", func(t *testing.T) {
		assert.Equal(t, "(?:1)", RegexForRange(1, 1))
		assert.Equal(t, "(?:[0-1])", RegexForRange(0, 1))
		assert.Equal(t, "(?:-1)", RegexForRange(-1, -1))
		assert.Equal(t, "(?:-1|0)", RegexForRange(-1, 0))
		assert.Equal(t, "(?:-1|[0-1])", RegexForRange(-1, 1))
		assert.Equal(t, "(?:-[2-4])", RegexForRange(-4, -2))
		assert.Equal(t, "(?:-[1-3]|[0-1])", RegexForRange(-3, 1))
		assert.Equal(t, "(?:-[1-2]|0)", RegexForRange(-2, 0))
		assert.Equal(t, "(?:[0-2])", RegexForRange(0, 2))
		assert.Equal(t, "(?:-1|[0-3])", RegexForRange(-1, 3))
		assert.Equal(t, "(?:6566[6-7])", RegexForRange(65666, 65667))
		assert.Equal(t, `(?:1[2-9]|[2-9]\d|[1-9]\d{2}|[1-2]\d{3}|3[0-3]\d{2}|34[0-4]\d|345[0-6])`, RegexForRange(12, 3456))
		assert.Equal(t, `(?:[1-9]|1\d)`, RegexForRange(1, 19))
		assert.Equal(t, `(?:[1-9]|[1-9]\d)`, RegexForRange(1, 99))

		assert.Equal(t, `(?:-[1-9]|\d)`, RegexForRange(-9, 9))
		assert.Equal(t, `(?:-[1-9]|-?1\d|\d)`, RegexForRange(-19, 19))
		assert.Equal(t, `(?:-[1-9]|-?[1-2]\d|\d)`, RegexForRange(-29, 29))
		assert.Equal(t, `(?:-[1-9]|-?[1-9]\d|\d)`, RegexForRange(-99, 99))
		assert.Equal(t, `(?:-[1-9]|-?[1-9]\d|-?[1-9]\d{2}|\d)`, RegexForRange(-999, 999))
		assert.Equal(t, `(?:-[1-9]|-?[1-9]\d|-?[1-9]\d{2}|-?[1-9]\d{3}|\d)`, RegexForRange(-9999, 9999))

		assert.Equal(t, "(1)", RegexForRange(1, 1, IntegerRangeRegexConfig{CapturingGroup: true}))
		assert.Equal(t, "(?:@1)", RegexForRange(1, 1, IntegerRangeRegexConfig{PositiveOnlyPrefix: "@"}))
		assert.Equal(t, "(?:@1)", RegexForRange(-1, -1, IntegerRangeRegexConfig{NegativeOnlyPrefix: "@"}))
		assert.Equal(t, `(?:-[1-9]|@1\d|\d)`, RegexForRange(-19, 19, IntegerRangeRegexConfig{IntersectedPrefix: "@"}))
	})

	checkMatches := func(t *testing.T, regex string, min int64, max int64, from_min int64, to_max int64) {
		compiledRegex := regexp.MustCompile("^" + regex + "$")

		for nr := from_min; nr <= to_max; nr++ {
			if min <= nr && nr <= max {
				assert.Regexp(t, compiledRegex, strconv.FormatInt(nr, 10))
			} else {
				assert.NotRegexp(t, compiledRegex, strconv.FormatInt(nr, 10))
			}
		}
	}

	t.Run("", func(t *testing.T) {
		regex := RegexForRange(1, 1)
		checkMatches(t, regex, 1, 1, 0, 100)

		regex = RegexForRange(65443, 65443)
		checkMatches(t, regex, 65443, 65443, 65000, 66000)

		regex = RegexForRange(192, 100020000300000)
		checkMatches(t, regex, 192, 1000, 0, 1000)
		//verify(t, regex, 100019999300000, 100020000300000, 100019999300000, 100020000400000)

		regex = RegexForRange(10331, 20381)
		checkMatches(t, regex, 10331, 20381, 0, 99999)

		regex = RegexForRange(10031, 20081)
		checkMatches(t, regex, 10031, 20081, 0, 99999)

		regex = RegexForRange(10301, 20101)
		checkMatches(t, regex, 10301, 20101, 0, 99999)

		regex = RegexForRange(1030, 20101)
		checkMatches(t, regex, 1030, 20101, 0, 99999)

		regex = RegexForRange(102, 111)
		checkMatches(t, regex, 102, 111, 0, 1000)

		regex = RegexForRange(102, 110)
		checkMatches(t, regex, 102, 110, 0, 1000)

		regex = RegexForRange(102, 130)
		checkMatches(t, regex, 102, 130, 0, 1000)

		regex = RegexForRange(4173, 7981)
		checkMatches(t, regex, 4173, 7981, 0, 99999)

		regex = RegexForRange(3, 7)
		checkMatches(t, regex, 3, 7, 0, 99)

		regex = RegexForRange(1, 9)
		checkMatches(t, regex, 1, 9, 0, 1000)

		regex = RegexForRange(1000, 8632)
		checkMatches(t, regex, 1000, 8632, 0, 99999)

		regex = RegexForRange(13, 8632)
		checkMatches(t, regex, 13, 8632, 0, 10000)

		regex = RegexForRange(9, 11)
		checkMatches(t, regex, 9, 11, 0, 100)

		regex = RegexForRange(90, 98099)
		checkMatches(t, regex, 90, 98099, 0, 99999)

		regex = RegexForRange(19, 21)
		checkMatches(t, regex, 19, 21, 0, 100)

		regex = RegexForRange(999, 10000)
		checkMatches(t, regex, 999, 10000, 1, 20000)
	})
}

func TestReplaceEndWithNines(t *testing.T) {
	assert.Equal(t, int64(99), replaceEndWithNines(10, 2))
	assert.Equal(t, int64(999), replaceEndWithNines(10, 3))
	assert.Equal(t, int64(9999), replaceEndWithNines(10, 4))

	assert.Equal(t, int64(199), replaceEndWithNines(100, 2))
	assert.Equal(t, int64(299), replaceEndWithNines(200, 2))
	assert.Equal(t, int64(1099), replaceEndWithNines(1000, 2))
	assert.Equal(t, int64(1999), replaceEndWithNines(1000, 3))
}

func TestReplaceEndWithZeros(t *testing.T) {
	assert.Equal(t, int64(0), replaceEndWithZeros(19, 2))
	assert.Equal(t, int64(0), replaceEndWithZeros(19, 3))

	assert.Equal(t, int64(100), replaceEndWithZeros(199, 2))
	assert.Equal(t, int64(200), replaceEndWithZeros(299, 2))
	assert.Equal(t, int64(1900), replaceEndWithZeros(1999, 2))
	assert.Equal(t, int64(1000), replaceEndWithZeros(1999, 3))
}

func TestValueExistsInBoth(t *testing.T) {
	assert.Equal(t, []string(nil), getSharedUnsharedElements([]string{}, []string{}, true))
	assert.Equal(t, []string(nil), getSharedUnsharedElements([]string{}, []string{}, false))

	assert.Equal(t, []string(nil), getSharedUnsharedElements([]string{"a"}, []string{}, true))
	assert.Equal(t, []string{"a"}, getSharedUnsharedElements([]string{"a"}, []string{}, false))
	assert.Equal(t, []string{"a"}, getSharedUnsharedElements([]string{"a"}, []string{"a"}, true))
}

func TestTurnCapturingGroupsIntoNonCapturing(t *testing.T) {
	turn := func(s string) string {
		regex := Must(syntax.Parse(s, syntax.Perl))
		return TurnCapturingGroupsIntoNonCapturing(regex).String()
	}

	assert.Equal(t, "(?:)", turn("()"))
	assert.Equal(t, "(?:)", turn("(?:)"))
	assert.Equal(t, "a", turn("(?:a)"))
	assert.Equal(t, "a", turn("(a)"))
	assert.Equal(t, "\\Aa(?-m:$)", turn("^a$")) //equivalent, fix ?
	assert.Equal(t, "\\(\\)", turn("\\(\\)"))
	//assert.Equal(t, "", turn("\\\\(\\\\)"))
	assert.Equal(t, "[\\(-\\)]", turn("[()]"))

	assert.Equal(t, "[a-z]", turn("([a-z])"))
	assert.Equal(t, "(?:[a-z]0*)?c", turn("([a-z]0*)?c"))
	assert.Equal(t, "(?:[a-z]0*(?:ab)+)?c", turn("([a-z]0*(?:ab)+)?c"))
}
