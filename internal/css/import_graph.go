package css

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	errImportCycle = errors.New("import cycle")
)

type ImportGraph struct {
	root       *LocalFile
	localFiles map[ /*absolute path*/ string]*LocalFile
}

func (g ImportGraph) Root() *LocalFile {
	return g.root
}

type LocalFile struct {
	absolutePath string
	imports      []Import
}

func (f LocalFile) AbsolutePath() string {
	return f.absolutePath
}

func (f LocalFile) Imports() []Import {
	return f.imports[:len(f.imports):len(f.imports)]
}

type Import struct {
	kind     ImportKind
	atRule   Node
	resource string //absolute path or URL

	localFile *LocalFile //set for local imports.
}

func (i Import) Kind() ImportKind {
	return i.kind
}

// Absolute path or URL.
func (i Import) Resource() string {
	return i.resource
}

func (i Import) Node() Node {
	return i.atRule
}

func (i Import) LocalFile() (*LocalFile, bool) {
	return i.localFile, i.localFile != nil
}

type ImportKind int

const (
	LocalImport    ImportKind = iota
	SameSiteImport ImportKind = iota //CSS file not provided by the current Inox project.
	URLImport      ImportKind = iota
)

func GetImportGraph(ctx context.Context, fls afs.Filesystem, absolutePath string) (*ImportGraph, error) {
	graph := new(ImportGraph)
	if absolutePath == "" {
		return nil, errors.New("empty file path")
	}
	if absolutePath[0] != '/' {
		return nil, errors.New("file path is not absolute")
	}

	absolutePath = filepath.Clean(absolutePath)

	_, err := buildImportGraph(ctx, fls, absolutePath, &[]*LocalFile{}, graph)

	if err != nil {
		return nil, err
	}
	return graph, nil
}

func buildImportGraph(ctx context.Context, fls afs.Filesystem, absoluteFilePath string, importerStack *[]*LocalFile, graph *ImportGraph) (*LocalFile, error) {

	if utils.IsContextDone(ctx) {
		return nil, ctx.Err()
	}

	absoluteFilePath = filepath.Clean(absoluteFilePath)

	for _, importer := range *importerStack {
		if filepath.Clean(importer.absolutePath) == absoluteFilePath {
			return nil, errImportCycle
		}
	}

	if _, ok := graph.localFiles[absoluteFilePath]; ok {
		return nil, nil
	}

	file := &LocalFile{absolutePath: absoluteFilePath}

	if len(*importerStack) == 0 {
		graph.root = file
		graph.localFiles = map[string]*LocalFile{}
	}

	f, err := fls.Open(absoluteFilePath)

	if err != nil {
		return nil, err
	}
	defer f.Close()

	node, err := ParseRead(ctx, f)
	if err != nil {
		return nil, err
	}

	graph.localFiles[absoluteFilePath] = file

	//Add $file to the importer stack
	*importerStack = append(*importerStack, file)
	defer func() {
		*importerStack = (*importerStack)[:len(*importerStack)-1]
	}()

	for _, topLevelNode := range node.Children {
		if !topLevelNode.IsImport() {
			// https://developer.mozilla.org/en-US/docs/Web/CSS/@import
			// An @import rule must be defined at the top of the stylesheet, before any other at-rule
			// (except @charset and @layer) and style declarations, or it will be ignored.
			break
		}

		importNode := topLevelNode

		if len(importNode.Children) != 1 {
			continue
		}

		_import := Import{
			atRule: importNode,
		}

		literal := importNode.Children[0]

		if literal.Error {
			continue
		}

		if literal.Type != String && literal.Type != URL {
			continue
		}

		value := literal.Data
		value = strings.TrimPrefix(value, "url(")
		value = strings.Trim(value, `")` /*cutset*/)

		if strings.HasPrefix(value, "http:") || strings.HasPrefix(value, "https:") { //URL import
			_import.kind = URLImport
			_import.resource = value
		} else { //Local import or same site import
			_import.kind = LocalImport

			importedFilePath, err := resolvePath(value, absoluteFilePath)

			if err != nil {
				return nil, err
			}

			importedFile, err := buildImportGraph(ctx, fls, importedFilePath, importerStack, graph)

			if errors.Is(err, fs.ErrNotExist) {
				//Even the file may be missing we assume it's a same site import.
				_import.kind = SameSiteImport
			} else if err != nil {
				return nil, err
			} else {
				_import.localFile = importedFile
			}

			_import.resource = importedFilePath
		}
		file.imports = append(file.imports, _import)
	}

	return file, nil
}

// resolvePath resolves the absolute path of an imported file.
// The Absolute() method of the filesystem is not used because relative paths may not be supported.
func resolvePath(importedFilePath string, absoluteImporterPath string) (string, error) {

	//Note: we clean the path aftet the validation because that could return '.'.

	err := checkValidPath(importedFilePath)
	if err != nil {
		return "", fmt.Errorf("import in %s: %w", absoluteImporterPath, err)
	}

	err = checkValidPath(absoluteImporterPath)
	if err != nil {
		return "", fmt.Errorf("invalid importer path: %s: %w", absoluteImporterPath, err)
	}

	if absoluteImporterPath[0] != '/' {
		return "", fmt.Errorf("invalid importer path: should be absolute: %s", absoluteImporterPath)
	}

	if importedFilePath[0] == '/' {
		return importedFilePath, nil
	}

	importedFilePath = filepath.Clean(importedFilePath)
	dir := filepath.Dir(absoluteImporterPath)

	return filepath.Join(dir, importedFilePath), nil
}

func checkValidPath(path string) error {
	if path == "" {
		return errors.New("empty file path")
	}

	segments := strings.Split(path, "/")
	for _, segment := range segments {
		if segment == ".." {
			return errors.New("'..' segments are not yet supported in import paths")
		}
	}
	return nil
}
