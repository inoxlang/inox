package regexutils

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/utils"
)

type IntegerRangeRegexConfig struct {
	CapturingGroup bool

	//optional
	NegativeOnlyPrefix, PositiveOnlyPrefix, IntersectedPrefix string
}

// RegexForRange returns a regex (not between ^$) that matches all numbers in the range [min, max].
// The main logic of the implementation is the same as https://github.com/voronind/range-regex (by Dmitry Voronin, BSD 2-Clause license).
func RegexForRange(min, max int64, conf ...IntegerRangeRegexConfig) string {
	config := IntegerRangeRegexConfig{}
	if len(conf) > 0 {
		config = conf[0]
	}

	isHardMinimum := min == math.MinInt64
	if isHardMinimum {
		if max == math.MinInt64 {
			prefix := "(?:"
			if config.CapturingGroup {
				prefix = "("
			}

			negativeOnlyPrefix := "-"
			if config.NegativeOnlyPrefix != "" {
				negativeOnlyPrefix = config.NegativeOnlyPrefix
			}

			minInt64withoutSign := strconv.FormatInt(math.MinInt64, 10)[1:]

			return prefix + negativeOnlyPrefix + minInt64withoutSign + ")"
		}

		min++
	}

	splitToRanges := func(min, max int64) []int64 {
		stopsSet := map[int64]struct{}{max: {}}

		ninesCount := 1
		stop := replaceEndWithNines(min, ninesCount)
		for min <= stop && stop < max {
			stopsSet[stop] = struct{}{}

			ninesCount++
			stop = replaceEndWithNines(min, ninesCount)
		}

		zerosCount := 1
		stop = replaceEndWithZeros(max+1, zerosCount) - 1
		for min < stop && stop <= max {
			stopsSet[stop] = struct{}{}

			zerosCount++
			stop = replaceEndWithZeros(max+1, zerosCount) - 1
		}

		var stops []int64
		for stop := range stopsSet {
			stops = append(stops, stop)
		}

		sort.Slice(stops, func(i, j int) bool { return stops[i] < stops[j] })
		return stops
	}

	splitToPatterns := func(min, max int64) []string {
		subPatterns := []string{}
		start := min
		for _, stop := range splitToRanges(min, max) {
			subPatterns = append(subPatterns, rangeToPattern(start, stop))
			start = stop + 1
		}

		return subPatterns
	}

	var positiveSubpatterns []string
	var negativeSubpatterns []string

	if min < 0 {
		absMin := int64(1)
		if max < 0 {
			absMin = utils.Abs(max)
		}
		absMax := utils.Abs(min)

		negativeSubpatterns = splitToPatterns(absMin, absMax)
	}

	if max >= 0 {
		if min <= 0 {
			positiveSubpatterns = splitToPatterns(0, max)
		} else {
			positiveSubpatterns = splitToPatterns(min, max)
		}
	}

	negativeOnlyPrefix := "-"
	if config.NegativeOnlyPrefix != "" {
		negativeOnlyPrefix = config.NegativeOnlyPrefix
	}

	positiveOnlyPrefix := ""
	if config.PositiveOnlyPrefix != "" {
		positiveOnlyPrefix = config.PositiveOnlyPrefix
	}

	intersectedPrefix := "-?"
	if config.IntersectedPrefix != "" {
		intersectedPrefix = config.IntersectedPrefix
	}

	negativeOnlySubpatterns := utils.MapSlice(getSharedUnsharedElements(negativeSubpatterns, positiveSubpatterns, false), func(s string) string {
		return negativeOnlyPrefix + s
	})
	positiveOnlySubpatterns := utils.MapSlice(getSharedUnsharedElements(positiveSubpatterns, negativeSubpatterns, false), func(s string) string {
		return positiveOnlyPrefix + s
	})
	intersectedSubpatterns := utils.MapSlice(getSharedUnsharedElements(negativeSubpatterns, positiveSubpatterns, true), func(s string) string {
		return intersectedPrefix + s
	})

	subpatterns := append(append(negativeOnlySubpatterns, intersectedSubpatterns...), positiveOnlySubpatterns...)

	if isHardMinimum {
		subpatterns = append(subpatterns, strconv.FormatInt(math.MinInt64, 10))
	}

	prefix := "(?:"
	if config.CapturingGroup {
		prefix = "("
	}

	return prefix + strings.Join(subpatterns, "|") + ")"
}

func rangeToPattern(start, stop int64) string {
	digits := func(n int64) []int64 {
		if n == 0 {
			return []int64{0}
		}

		var digits []int64
		for n > 0 {
			remainder := n % 10
			digits = append([]int64{remainder}, digits...)
			n = n / 10
		}

		return digits
	}

	pattern := ""
	anyDigitCount := 0
	startDigits := digits(start)
	stopDigits := digits(stop)

	for i := 0; i < len(stopDigits); i++ {
		if startDigits[i] == stopDigits[i] {
			pattern += strconv.FormatInt(startDigits[i], 10)
		} else if startDigits[i] != 0 || stopDigits[i] != 9 {
			pattern += fmt.Sprintf("[%d-%d]", startDigits[i], stopDigits[i])
		} else {
			anyDigitCount++
		}
	}

	if anyDigitCount != 0 {
		pattern += `\d`
	}

	if anyDigitCount > 1 {
		pattern += fmt.Sprintf("{%d}", anyDigitCount)
	}

	return pattern
}

func getSharedUnsharedElements(slice1, slice2 []string, shared bool) []string {
	var res []string
	for _, val1 := range slice1 {
		areShared := false
		for _, val2 := range slice2 {
			if val1 == val2 {
				areShared = true
				break
			}
		}
		if areShared == shared {
			res = append(res, val1)
		}
	}
	return res
}

func replaceEndWithNines(integer int64, ninesCount int) int64 {
	integerStr := strconv.FormatInt(integer, 10)
	prefix := ""
	if len(integerStr) >= ninesCount {
		prefix = integerStr[:len(integerStr)-ninesCount]
	}
	suffix := ""
	for i := 0; i < ninesCount; i++ {
		suffix += "9"
	}
	result, _ := strconv.ParseInt(prefix+suffix, 10, 64)
	return result
}

func replaceEndWithZeros(integer int64, zerosCount int) int64 {
	return integer - integer%int64(math.Pow10(zerosCount))
}
