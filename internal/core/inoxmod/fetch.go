package inoxmod

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/memds"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/sourcecode"
)

var (
	ErrImportedModulesDotExist = errors.New("imported module does not exist")
)

type importedModulesFetchConfig struct {
	recoverFromNonExistingFiles, ignoreBadlyConfiguredImports bool
	timeout                                                   time.Duration
	insecure                                                  bool
	subModuleParsing                                          ModuleParsingConfig
	parentModuleId                                            memds.NodeId
}

func fetchParseImportedModules(parentMod *Module, ctx Context, config importedModulesFetchConfig) (unrecoverableError error) {
	subModuleParsing := config.subModuleParsing
	importStmts := ast.FindNodes(parentMod.MainChunk.Node, (*ast.ImportStatement)(nil), nil)

	stmtSources := map[ResourceName]*ast.ImportStatement{}
	validationStrings := map[ResourceName]string{}

	for _, importStmt := range importStmts {
		//ignore import if the source or the config has an error
		if config.recoverFromNonExistingFiles && (importStmt.Source == nil || importStmt.Source.Base().Err != nil) ||
			(importStmt.Configuration == nil || importStmt.Configuration.Base().Err != nil) {
			continue
		}

		var src ResourceName

		switch importStmt.Source.(type) {
		case *ast.URLLiteral, *ast.AbsolutePathLiteral, *ast.RelativePathLiteral:
			value, err := EvalResourceNameLiteral(importStmt.Source.(ast.SimpleValueLiteral))
			if err != nil {
				panic(err)
			} else {
				src = value
			}
		}

		src, err := GetSourceFromImportSource(src, parentMod, ctx)
		if err != nil {
			return err
		}

		//add the module to the graph if necessary
		var nodeId memds.NodeId
		node, err := subModuleParsing.moduleGraph.GetNode(memds.WithData, src.ResourceName())
		if err != nil && !errors.Is(err, memds.ErrNodeNotFound) {
			return fmt.Errorf("failed to check if module %q is present in the module graph: %w", src.ResourceName(), err)
		} else if errors.Is(err, memds.ErrNodeNotFound) {
			nodeId = subModuleParsing.moduleGraph.AddNode(src.ResourceName())
		} else {
			nodeId = node.Id
		}
		if nodeId == config.parentModuleId {
			return fmt.Errorf("%w: module %s imports itself", ErrImportCycleDetected, src.ResourceName())
		}

		subModuleParsing.moduleGraph.SetEdge(config.parentModuleId, nodeId, struct{}{})

		if err := checkNoCycleOrLongPathInModuleGraph(subModuleParsing.moduleGraph); err != nil {
			return err
		}

		objLiteral, ok := importStmt.Configuration.(*ast.ObjectLiteral)
		if !ok {
			if !config.ignoreBadlyConfiguredImports {
				return errors.New("invalid module import: configuration should be an object")
			}
			continue
		}

		var validationString string = ""
		validationNode, ok := objLiteral.PropValue("validation")
		if ok {
			validationStringLit, ok := validationNode.(*ast.DoubleQuotedStringLiteral)
			if ok {
				validationString = validationStringLit.Value
			} else {
				if !config.ignoreBadlyConfiguredImports {
					return errors.New("invalid module import: <configuration>.validation should be a string")
				}
			}
		}

		stmtSources[src] = importStmt
		validationStrings[src] = validationString
	}

	var (
		wg                          = new(sync.WaitGroup)
		lock                        sync.Mutex
		importedModules             = map[string]*Module{}
		importedModulesByImportStmt = map[*ast.ImportStatement]*Module{}
		importErrors                []Error
		unrecoverableErors          []error
	)

	wg.Add(len(stmtSources))

	childCtx := CreateBoundChildCtx(ctx)
	defer childCtx.CancelGracefully()

	for src := range stmtSources {

		go func(src ResourceName, validationString string) {
			defer wg.Done()

			var importedMod *Module
			var err error

			defer func() {
				if e, ok := recover().(error); ok {
					lock.Lock()
					unrecoverableErors = append(unrecoverableErors, e)
					lock.Unlock()
				} else if e == nil {
					lock.Lock()

					importStmt := stmtSources[src]
					importStmtPosition := parentMod.MainChunk.GetSourcePosition(importStmt.Source.Base().Span)

					if importedMod != nil {
						importedModules[importedMod.Name()] = importedMod
						importedModulesByImportStmt[importStmt] = importedMod
					} else if config.recoverFromNonExistingFiles {
						importErrors = append(importErrors, Error{
							Position:  importStmtPosition,
							BaseError: err,
						})
					} else {
						unrecoverableErors = append(unrecoverableErors, fmt.Errorf("%s: %w", importStmtPosition, err))
					}
					lock.Unlock()
				}
			}()

			importedMod, err = fetchParseImportedModule(childCtx, src, sourceFileDownloadConfig{
				parentModule:     parentMod,
				validation:       validationString,
				insecure:         config.insecure,
				timeout:          config.timeout,
				subModuleParsing: config.subModuleParsing,
			})

			if err != nil { //stop all other fetches
				childCtx.CancelGracefully()
			}

		}(src, validationStrings[src])

	}

	wg.Wait()

	if len(unrecoverableErors) > 0 {
		return errors.Join(unrecoverableErors...)
	}

	parentMod.DirectlyImportedModules = importedModules
	parentMod.DirectlyImportedModulesByStatement = importedModulesByImportStmt

	for _, importedMod := range importedModules {
		parentMod.FileLevelParsingErrors = append(parentMod.FileLevelParsingErrors, importedMod.FileLevelParsingErrors...)
		parentMod.Errors = append(parentMod.Errors, importedMod.Errors...)
	}

	parentMod.Errors = append(parentMod.Errors, importErrors...)

	return nil
}

type sourceFileDownloadConfig struct {
	parentModule *Module
	validation   string
	insecure     bool
	timeout      time.Duration

	subModuleParsing ModuleParsingConfig
}

// fetchParseImportedModule first fetches a module by reading the filesystem or making an HTTP request, then it parses it.
func fetchParseImportedModule(ctx Context, resolvedImportedSrc ResourceName, config sourceFileDownloadConfig) (*Module, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	//check that the resource name is a URL or an absolute path

	switch {
	case resolvedImportedSrc.IsPath():
		name := resolvedImportedSrc.ResourceName()
		if name[0] != '/' {
			return nil, fmt.Errorf("import: invalid source: %q, path should have been made absolute by the caller", name)
		}
	case resolvedImportedSrc.IsURL():
	default:
		return nil, fmt.Errorf("import: invalid source: %q", resolvedImportedSrc.ResourceName())
	}

	if !strings.HasSuffix(resolvedImportedSrc.ResourceName(), inoxconsts.INOXLANG_FILE_EXTENSION) {
		return nil, errors.New(symbolic.IMPORTED_MOD_PATH_MUST_END_WITH_IX)
	}

	timeout := config.timeout
	if timeout == 0 {
		timeout = DEFAULT_FETCH_TIMEOUT
	}

	deadline := time.Now().Add(timeout)

	var b []byte

	moduleCacheLock.Lock()
	unlock := true
	defer func() {
		if unlock {
			moduleCacheLock.Unlock()
		}
	}()

	source := sourcecode.File{
		NameString: resolvedImportedSrc.ResourceName(),
		Resource:   resolvedImportedSrc.ResourceName(),
	}

	if content, ok := moduleCache[config.validation]; !ok || config.validation == "" {
		switch {
		case resolvedImportedSrc.IsPath():
			name := resolvedImportedSrc.ResourceName()

			absSrc, err := filepath.Abs(name)
			if err != nil {
				return nil, err
			}

			source.ResourceDir = filepath.Dir(absSrc)

			file, err := os.OpenFile(name, os.O_RDONLY, 0)
			if err != nil {
				return nil, err
			}
			content, err := io.ReadAll(file)
			if err != nil {
				return nil, err
			}
			b = content
		case resolvedImportedSrc.IsURL():
			source.IsResourceURL = true

			urlString := resolvedImportedSrc.ResourceName()
			u, err := url.Parse(urlString)
			if err != nil {
				return nil, ErrInvalidModuleSourceURL
			}

			pth := u.Path
			if pth == "" {
				return nil, ErrInvalidModuleSourceURL
			}

			isRelative := isPathRelative(pth)
			isDirPath := isPathDirPath(pth)

			if isRelative || isDirPath {
				return nil, ErrInvalidModuleSourceURL
			}

			{
				lastSlashIndex := strings.LastIndex(string(pth), "/")
				source.ResourceDir = string(u.Host) + string(pth)[:lastSlashIndex+1]
			}

			transport := &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 5 * time.Second,
					Deadline:  deadline.Add(-time.Second),
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          1,
				IdleConnTimeout:       1 * time.Second,
				TLSHandshakeTimeout:   5 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			}

			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: config.insecure}
			client := http.Client{
				Timeout:   timeout,
				Transport: transport,
			}

			req, err := http.NewRequest("GET", pth, nil)
			req.Header.Add("Accept", mimeconsts.INOX_CTYPE)

			reqCtx, cancel := context.WithDeadline(ctx, deadline)
			defer cancel()
			req = req.WithContext(reqCtx)

			if err != nil {
				return nil, err
			}

			resp, err := client.Do(req)

			if err != nil {
				return nil, err
			}

			//TODO: sanitize .Status, Content-Type, etc before writing them to the terminal
			var bodyErr error
			b, bodyErr = io.ReadAll(resp.Body)

			if resp.StatusCode != 200 {
				err := fmt.Errorf("failed to get %s: status %d: %s", urlString, resp.StatusCode, resp.Status)
				return nil, err
			}

			// ctype := resp.Header.Get("Content-Type")
			// if ctype != INOX_MIMETYPE {
			// 	return nil, fmt.Errorf("failed to get %s: content-type is '%s'", importURL, ctype)
			// }

			if bodyErr != nil {
				return nil, fmt.Errorf("failed to get %s: failed to read body: %s", urlString, bodyErr.Error())
			}
		}

		//Validate the content.

		array := sha256.Sum256(b)
		hash := array[:]

		if config.validation != "" && !bytes.Equal(hash, []byte(config.validation)) {
			return nil, fmt.Errorf("failed to get %s: validation failed", resolvedImportedSrc.ResourceName())
		}

		content = string(b)
		moduleCache[string(hash)] = content
		source.CodeString = content

		//TODO: limit cache size
	} else {
		source.CodeString = content
	}

	unlock = false
	moduleCacheLock.Unlock()

	return ParseModuleFromSource(source, resolvedImportedSrc, config.subModuleParsing)
}
