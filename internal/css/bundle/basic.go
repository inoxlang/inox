package bundle

import (
	"context"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/utils"
)

type BundlingParams struct {
	InputFile  string
	Filesystem afs.Filesystem
}

func Bundle(ctx context.Context, params BundlingParams) (stylesheet css.Node, _ error) {
	if utils.IsContextDone(ctx) {
		return css.Node{}, ctx.Err()
	}

	fls := params.Filesystem

	graph, err := css.GetImportGraph(ctx, fls, params.InputFile)

	if err != nil {
		return css.Node{}, ctx.Err()
	}

	stylesheet = css.Node{
		Type: css.Stylesheet,
	}

	visitedFiles := map[*css.LocalFile]struct{}{}

	//post-order traveral.
	err = graph.Walk(css.ImportGraphWalkParams{
		AllowRevisit: false,
		PostHandle: func(node css.Import, localFile *css.LocalFile, importerStack []*css.LocalFile, after bool) (css.GraphTraversalAction, error) {
			//node.IsZero() == leaf local file

			//We only care about the importer: once we have seen an importer once we ignore it all the next times.
			_, ok := visitedFiles[localFile]
			if ok {
				return css.ContinueGraphTraversal, nil
			}
			visitedFiles[localFile] = struct{}{}

			//Copy all definitions in the resulting stylesheet.

			importerStylesheet := localFile.Stylesheet()

			for _, child := range importerStylesheet.Children {
				_import, ok := localFile.TryGetImport(child)
				//Ignore local imports.
				if ok && _import.Kind() == css.LocalImport {
					continue
				}
				stylesheet.Children = append(stylesheet.Children, child)
			}

			return css.ContinueGraphTraversal, nil
		},
	})

	if err != nil {
		return css.Node{}, ctx.Err()
	}

	return stylesheet, nil
}
