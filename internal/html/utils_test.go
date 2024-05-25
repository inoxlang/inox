package html_ns

import (
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func TestComputeNodeAndHeight(t *testing.T) {
	context := &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Div,
		Data:     "div",
	}

	testCases := []struct {
		name           string
		html           string
		expectedWidth  int
		expectedHeight int
	}{
		{
			name: "empty",
			html: `
			<div></div>`,
			expectedWidth:  1,
			expectedHeight: 1,
		},
		{
			name: "single level",
			html: `
			<div>
				<div></div>
				<div></div>
			</div>`,
			expectedWidth:  2,
			expectedHeight: 2,
		},
		{
			name: "multiple levels",
			html: `
			<div>
				<div>
					<title></title>
				</div>
				<div>
					<div>
						<p></p>
						<p></p>
					</div>
					<ul>
						<li></li>
						<li></li>
						<li></li>
					</ul>
				</div>
			</div>`,
			expectedWidth:  5,
			expectedHeight: 4,
		},
		{
			name: "nested elements",
			html: `
			<div>
				<div>
					<div>
						<ul>
							<li></li>
							<li></li>
							<li>
								<ol>
									<li></li>
									<li></li>
								</ol>
							</li>
						</ul>
					</div>
				</div>
			</div>`,
			expectedWidth:  3,
			expectedHeight: 7,
		},
	}

	for _, testCase := range testCases {
		//remove all text nodes
		var b strings.Builder
		for _, ch := range testCase.html {
			if !unicode.IsSpace(ch) {
				b.WriteRune(ch)
			}
		}
		testCase.html = b.String()

		t.Run("width_:"+testCase.name, func(t *testing.T) {
			node, err := html.ParseFragment(strings.NewReader(testCase.html), context)
			assert.NoError(t, err)

			width := computeNodeWidth(node[0])
			assert.Equal(t, testCase.expectedWidth, width)
		})
		t.Run("height :"+testCase.name, func(t *testing.T) {
			node, err := html.ParseFragment(strings.NewReader(testCase.html), context)
			assert.NoError(t, err)

			height := computeNodeHeight(node[0])
			assert.Equal(t, testCase.expectedHeight, height)
		})
	}
}
