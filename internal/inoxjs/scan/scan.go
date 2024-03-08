package scan

import (
	"regexp"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/codebase/scan"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/inoxjs"
	"github.com/inoxlang/inox/internal/parse"
	"golang.org/x/exp/maps"
)

var (
	CSS_SCOPE_INLINE_DETECTION_PATTERN = regexp.MustCompile(`(\b|^\s*)me\b`)
	SURREAL_DETECTION_PATTERN          = regexp.MustCompile(`(\b|^\s*)(me|any)\(`)
	PREACT_SIGNALS_DETECTION_PATTERN   = regexp.MustCompile(`(signal|computed|effect|batch|untracked)\(`)
)

type Configuration struct {
	TopDirectories []string
	MaxFileSize    int64 //defaults to codebasescan.DEFAULT_MAX_SCANNED_INOX_FILE_SIZE
	Fast           bool  //if true the scan will be faster but will use more CPU and memory.
	InoxChunkCache *parse.ChunkCache
}

type ScanResult struct {
	Libraries []string
}

func ScanCodebase(ctx *core.Context, fls afs.Filesystem, config Configuration) (ScanResult, error) {

	libs := map[string]struct{}{}

	handleFile := func(path, fileContent string, n *parse.Chunk) error {
		parse.Walk(n, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

			xmlText, ok := node.(*parse.XMLText)
			if ok {
				if strings.Contains(xmlText.Value, inoxjs.TEXT_INTERPOLATION_OPENING_DELIMITER) {
					libs[inoxjs.INOX_COMPONENT_LIB_NAME] = struct{}{}
				}
				return parse.ContinueTraversal, nil
			}

			elem, ok := node.(*parse.XMLElement)
			if !ok {
				return parse.ContinueTraversal, nil
			}

			switch elem.EstimatedRawElementType {
			case parse.CssStyleElem:
				if CSS_SCOPE_INLINE_DETECTION_PATTERN.MatchString(elem.RawElementContent) {
					libs[inoxjs.CSS_INLINE_SCOPE_LIB_NAME] = struct{}{}
				}
			case parse.JsScript:
				if SURREAL_DETECTION_PATTERN.MatchString(elem.RawElementContent) {
					libs[inoxjs.SURREAL_LIB_NAME] = struct{}{}
				}
				if PREACT_SIGNALS_DETECTION_PATTERN.MatchString(elem.RawElementContent) {
					libs[inoxjs.PREACT_SIGNALS_LIB_NAME] = struct{}{}
				}
				if strings.Contains(elem.RawElementContent, inoxjs.INIT_COMPONENT_FN_NAME+"(") {
					libs[inoxjs.INOX_COMPONENT_LIB_NAME] = struct{}{}
				}
			}

			return parse.ContinueTraversal, nil
		}, nil)

		return nil
	}

	err := scan.ScanCodebase(ctx, fls, scan.Configuration{
		TopDirectories:     []string{"/"},
		ChunkCache:         config.InoxChunkCache,
		FileHandlers:       []scan.FileHandler{handleFile},
		FileParsingTimeout: 50 * time.Millisecond,
	})

	if err != nil {
		return ScanResult{}, nil
	}

	return ScanResult{
		Libraries: maps.Keys(libs),
	}, nil
}
