package scan

import (
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/codebase/scan"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/htmx"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/maps"
)

type Configuration struct {
	TopDirectories []string
	MaxFileSize    int64 //defaults to codebasescan.DEFAULT_MAX_SCANNED_INOX_FILE_SIZE
	Fast           bool  //if true the scan will be faster but will use more CPU and memory.
	InoxChunkCache *parse.ChunkCache
}

type ScanResult struct {
	UsedExtensions []string
}

func ScanCodebase(ctx *core.Context, fls afs.Filesystem, config Configuration) (ScanResult, error) {

	extensions := map[string]struct{}{}

	handleFile := func(path, fileContent string, n *parse.Chunk) error {
		parse.Walk(n, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {

			xmlAttr, ok := node.(*parse.XMLAttribute)
			if !ok {
				return parse.ContinueTraversal, nil
			}

			ident, ok := xmlAttr.Name.(*parse.IdentifierLiteral)
			if !ok || !strings.HasPrefix(ident.Name, "hx-") {
				return parse.ContinueTraversal, nil
			}

			name := ident.Name

			switch {
			case htmx.JSONFORM_SHORTHAND_ATTRIBUTE_PATTERN.MatchString(name):
				extensions[htmx.JSONFORM_EXT_NAME] = struct{}{}
			case name == "hx-ext":
				names := strings.Split(xmlAttr.ValueIfStringLiteral(), ",")
				names = utils.MapSlice(names, strings.TrimSpace)
				for _, name := range names {
					extensions[name] = struct{}{}
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
		UsedExtensions: maps.Keys(extensions),
	}, nil
}
