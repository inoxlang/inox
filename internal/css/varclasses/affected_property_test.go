package varclasses

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInferAffectedProperty(t *testing.T) {

	first := func(list []string) string {
		return list[0]
	}

	testCases := [][2]string{
		{"background", "background"},
		{"primary-background", "background"},

		{"bg", "background"},
		{"primary-bg", "background"},

		{"primary-bg-color", "background-color"},
		{"bg-color", "background-color"},

		{"background-" + first(BACKGROUND_PROP_BASES_WITHOUT_DIMINUTIVES), "_same"},
		{"border-image-" + first(BORDER_IMAGE_PROP_BASES), "_same"},
		{"border-img-" + first(BORDER_IMAGE_PROP_BASES), "border-image-" + first(BORDER_IMAGE_PROP_BASES)},

		{"border-color", "_same"},
		{"primary-border-color", "border-color"},

		{first(BORDER_UNIQUE_PROPS), "_same"},
		{first(BORDER_RADIUS_PROPS), "_same"},

		{"border-left-" + first(COMMON_BORDER_PROP_BASES), "_same"},
	}

	for _, testCase := range testCases {
		t.Run(testCase[0], func(t *testing.T) {
			prop := inferAffectedProperty("--" + testCase[0])
			expected := testCase[1]
			if expected == "_same" {
				expected = testCase[0]
			}
			assert.Equal(t, expected, prop)
		})
	}
}
