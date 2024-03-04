package gen

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/tailwind"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_MAX_SCANNED_INOX_FILE_SIZE = 1_000_000
)

type ScanConfiguration struct {
	TopDirectory string
	MaxFileSize  int64 //defaults to DEFAULT_MAX_SCANNED_INOX_FILE_SIZE
	Fast         bool  //if true the scan will be faster but will use more CPU and memory.
}

// ScanForTailwindRulesToInclude scans for Tailwind class names in 'class' attributes in Inox files.
func ScanForTailwindRulesToInclude(ctx *core.Context, fls afs.Filesystem, config ScanConfiguration) (rules map[string]tailwind.Ruleset, _ error) {

	rules = map[string]tailwind.Ruleset{}

	maxFileSize := utils.DefaultIfZero(config.MaxFileSize, DEFAULT_MAX_SCANNED_INOX_FILE_SIZE)

	if err := ctx.CheckHasPermission(core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")}); err != nil {
		return nil, err
	}

	err := core.WalkDirLow(fls, config.TopDirectory, func(path string, d fs.DirEntry, err error) error {

		if ctx.IsDoneSlowCheck() {
			return ctx.Err()
		}

		//Ignore non-Inox files.
		if d.IsDir() || filepath.Ext(path) != inoxconsts.INOXLANG_FILE_EXTENSION {
			return nil
		}

		//Ignore large files.
		stat, err := fls.Stat(path)
		if err != nil {
			if os.IsNotExist(err) { //The file may have been deleted by the developer.
				return nil
			}
			return err
		}

		if stat.Size() > maxFileSize { //ignore file
			return nil
		}

		//Open and read the file.

		f, err := fls.Open(path)
		if err != nil {
			if os.IsNotExist(err) { //The file may have been deleted by the developer.
				return nil
			}
			return err
		}

		var content []byte

		func() {
			defer f.Close()
			content, err = io.ReadAll(io.LimitReader(f, maxFileSize))
		}()

		if err != nil {
			return err
		}

		//Parse the file.

		result, err := parse.ParseChunk(string(content), path)
		if result == nil { //critical error
			return nil
		}

		for _, rule := range findTailwindRulesToInclude(result) {
			rules[rule.Name] = rule
		}

		if !config.Fast {
			runtime.Gosched()
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return
}

func findTailwindRulesToInclude(chunk *parse.Chunk) (rulesets []tailwind.Ruleset) {
	//Search for tailwind class names in 'class' attributes.

	parse.Walk(chunk, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
		attr, ok := node.(*parse.XMLAttribute)
		if !ok {
			return parse.ContinueTraversal, nil
		}

		ident, ok := attr.Name.(*parse.IdentifierLiteral)
		if !ok || ident.Name != "class" {
			return parse.Prune, nil
		}

		attrValue := ""

		switch v := attr.Value.(type) {
		case *parse.QuotedStringLiteral:
			attrValue = v.Value
		case *parse.MultilineStringLiteral:
			attrValue = v.Value
			//TODO: support string templates
		default:
			return parse.Prune, nil
		}

		classNames := strings.Split(attrValue, " ")
		for _, name := range classNames {
			name = strings.TrimSpace(name)
			ruleset, ok := tailwind.GetRuleset("." + name)
			if ok {
				rulesets = append(rulesets, ruleset)
			}
		}

		return parse.ContinueTraversal, nil
	}, nil)

	return
}
