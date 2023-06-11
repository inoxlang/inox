package compl

import (
	"path"
	"path/filepath"
	"strings"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/lsp/lsp/defines"

	"github.com/inoxlang/inox/internal/globals/fs_ns"

	"github.com/inoxlang/inox/internal/globals/s3_ns"
	"github.com/inoxlang/inox/internal/utils"

	parse "github.com/inoxlang/inox/internal/parse"
)

func findPathCompletions(ctx *core.Context, pth string) []Completion {
	var completions []Completion

	fls := ctx.GetFileSystem()
	dir := path.Dir(pth)
	base := path.Base(pth)

	if core.Path(pth).IsDirPath() {
		base = ""
	}

	entries, err := fs_ns.ListFiles(ctx, core.Path(dir+"/"))
	if err != nil {
		return nil
	}

	for _, e := range entries {
		name := string(e.BaseName_)
		if strings.HasPrefix(name, base) {
			pth := path.Join(dir, name)

			if !parse.HasPathLikeStart(pth) {
				pth = "./" + pth
			}

			stat, _ := fls.Stat(pth)
			if stat.IsDir() {
				pth += "/"
			}

			completions = append(completions, Completion{
				ShownString: name,
				Value:       pth,
				Kind:        defines.CompletionItemKindConstant,
				Detail:      "%" + core.PATH_PATTERN.Name,
			})
		}
	}

	return completions
}

func findURLCompletions(ctx *core.Context, u core.URL, parent parse.Node) []Completion {
	var completions []Completion

	urlString := string(u)

	if call, ok := parent.(*parse.CallExpression); ok {

		var S3_FNS = []string{"get", "delete", "ls"}

		if memb, ok := call.Callee.(*parse.IdentifierMemberExpression); ok &&
			memb.Left.Name == "s3" &&
			len(memb.PropertyNames) == 1 &&
			utils.SliceContains(S3_FNS, memb.PropertyNames[0].Name) &&
			strings.Contains(urlString, "/") {

			objects, err := s3_ns.S3List(ctx, u)
			if err == nil {
				prefix := urlString[:strings.LastIndex(urlString, "/")+1]
				for _, obj := range objects {

					val := prefix + filepath.Base(obj.Key)
					if strings.HasSuffix(obj.Key, "/") {
						val += "/"
					}

					completions = append(completions, Completion{
						ShownString: obj.Key,
						Value:       val,
						Kind:        defines.CompletionItemKindConstant,
						Detail:      "%" + core.URL_PATTERN.Name,
					})
				}
			}
		}
	}

	return completions
}

func findHostCompletions(ctx *core.Context, prefix string, parent parse.Node) []Completion {
	var completions []Completion

	allData := ctx.GetAllHostResolutionData()

	for host := range allData {
		hostStr := string(host)
		if strings.HasPrefix(hostStr, prefix) {
			completions = append(completions, Completion{
				ShownString: hostStr,
				Value:       hostStr,
				Kind:        defines.CompletionItemKindConstant,
				Detail:      "%" + core.HOST_PATTERN.Name,
			})
		}
	}

	{ //localhost
		scheme, realHost, ok := strings.Cut(prefix, "://")

		var schemes = []string{"http", "https", "file", "ws", "wss"}

		if ok && utils.SliceContains(schemes, scheme) && len(realHost) > 0 && strings.HasPrefix("localhost", realHost) {
			s := strings.Replace(prefix, realHost, "localhost", 1)
			completions = append(completions, Completion{
				ShownString: s,
				Value:       s,
				Kind:        defines.CompletionItemKindConstant,
				Detail:      "%" + core.HOST_PATTERN.Name,
			})
		}

	}

	return completions
}
