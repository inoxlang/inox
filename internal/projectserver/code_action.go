package projectserver

import (
	"fmt"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
)

var quickfixKind = defines.CodeActionKindQuickFix

func getCodeActions(
	session *jsonrpc.Session, diagnostics []defines.Diagnostic, _range defines.Range,
	doc defines.TextDocumentIdentifier, fpath string, fls *Filesystem,
) (*[]defines.CodeAction, error) {

	chunk, err := core.ParseFileChunk(fpath, fls)
	if err != nil {
		return nil, err
	}

	var codeActions []defines.CodeAction

	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == nil {
			continue
		}

		action, ok := tryGetMissingPermissionAction(doc, diagnostic, chunk)
		if ok {
			codeActions = append(codeActions, action)
		}

	}

	return &codeActions, nil
}

func tryGetMissingPermissionAction(doc defines.TextDocumentIdentifier, diagnostic defines.Diagnostic, chunk *parse.ParsedChunk) (action defines.CodeAction, actionOk bool) {
	indentUnit := chunk.EstimatedIndentationUnit()
	if indentUnit == "" {
		indentUnit = strings.Repeat(" ", 4)
	}

	switch *diagnostic.Severity {
	case defines.DiagnosticSeverityWarning:
		if strings.Contains(diagnostic.Message, symbolic.POSSIBLE_MISSING_PERM_TO_CREATE_A_LTHREAD) {

			var textEdit defines.TextEdit

			permsObject, ok := getPermissionsObject(chunk)
			if ok {
				permsSpan := permsObject.Span

				createPropValue, ok := permsObject.PropValue("create")
				if ok {
					objLit, ok := createPropValue.(*parse.ObjectLiteral)
					objSpan := objLit.Span
					if ok {
						lastChar := chunk.Runes()[objSpan.End-1]
						if lastChar != '}' {
							return
						}

						line, col := chunk.GetIncludedEndSpanLineColumn(objSpan)
						endLine, endCol := chunk.GetEndSpanLineColumn(objSpan)

						textEdit.Range = rangeToLspRange(parse.SourcePositionRange{
							StartLine:   line,
							StartColumn: col,
							EndLine:     endLine,
							EndColumn:   endCol,
							Span:        parse.NodeSpan{Start: objSpan.End - 1, End: objSpan.End},
						})
						textEdit.NewText = "\n" + indentUnit + indentUnit + "threads: {}" + indentUnit + "}\n" + indentUnit
					} else {
						return
					}
				} else {
					lastChar := chunk.Runes()[permsSpan.End-1]
					if lastChar != '}' {
						return
					}

					line, col := chunk.GetIncludedEndSpanLineColumn(permsSpan)
					endLine, endCol := chunk.GetEndSpanLineColumn(permsSpan)

					textEdit.Range = rangeToLspRange(parse.SourcePositionRange{
						StartLine:   line,
						StartColumn: col,
						EndLine:     endLine,
						EndColumn:   endCol,
						Span:        parse.NodeSpan{Start: permsSpan.End - 1, End: permsSpan.End},
					})
					textEdit.NewText = indentUnit + "create: {threads: {}}\n" + indentUnit + "}"
				}

			} else {
				editSpanStart := int32(0)
				textEdit.NewText = fmt.Sprintf("manifest {\n%spermissions: {\n%s%screate: {threads: {}}\n%s}\n}", indentUnit, indentUnit, indentUnit, indentUnit)

				newline := false
				if chunk.Node.GlobalConstantDeclarations != nil {
					editSpanStart = chunk.Node.GlobalConstantDeclarations.Span.End
					newline = true
				}

				if chunk.Node.Preinit != nil {
					editSpanStart = chunk.Node.Preinit.Span.End
					newline = true
				}

				if newline {
					textEdit.NewText = "\n\n" + textEdit.NewText
				}

				editSpan := parse.NodeSpan{Start: editSpanStart, End: editSpanStart}
				editLine, editCol := chunk.GetSpanLineColumn(editSpan)

				textEdit.Range = rangeToLspRange(parse.SourcePositionRange{
					StartLine:   editLine,
					StartColumn: editCol,
					EndLine:     editLine,
					EndColumn:   editCol + 1,
					Span:        editSpan,
				})
			}

			action = defines.CodeAction{
				Title: "Add Missing Permission",
				Kind:  &quickfixKind,
				Edit: &defines.WorkspaceEdit{
					Changes: &map[string][]defines.TextEdit{
						string(doc.Uri): {textEdit},
					},
				},
			}

			actionOk = true
		}
	}

	return
}

func getPermissionsObject(chunk *parse.ParsedChunk) (*parse.ObjectLiteral, bool) {
	if chunk.Node.Manifest == nil {
		return nil, false
	}

	obj, ok := chunk.Node.Manifest.Object.(*parse.ObjectLiteral)
	if !ok {
		return nil, false
	}

	for _, prop := range obj.Properties {
		if prop.HasImplicitKey() || prop.Name() != core.MANIFEST_PERMS_SECTION_NAME {
			continue
		}
		permsObject, ok := prop.Value.(*parse.ObjectLiteral)
		return permsObject, ok
	}

	return nil, false
}
