package defines

/**
 * A tagging type for string properties that are actually document URIs.
 */
type DocumentUri string

/**
 * A tagging type for string properties that are actually URIs
 *
 * @since 3.16.0
 */
type URI string

/**
 * An identifier to refer to a change annotation stored with a workspace edit.
 */
type ChangeAnnotationIdentifier string

/**
 * Information about where a symbol is defined.
 *
 * Provides additional metadata over normal [location](#Location) definitions, including the range of
 * the defining symbol
 */
type DefinitionLink LocationLink

/**
 * Information about where a symbol is declared.
 *
 * Provides additional metadata over normal [location](#Location) declarations, including the range of
 * the declaring symbol.
 *
 * Servers should prefer returning `DeclarationLink` over `Declaration` if supported
 * by the client.
 */
type DeclarationLink LocationLink

/**
 * The kind of a code action.
 *
 * Kinds are a hierarchical list of identifiers separated by `.`, e.g. `"refactor.extract.function"`.
 *
 * The set of kinds is open and client needs to announce the kinds it supports to the server during
 * initialization.
 */

/**
 * Position in a text document expressed as zero-based line and character offset.
 * The offsets are based on a UTF-16 string representation. So a string of the form
 * `a𐐀b` the character offset of the character `a` is 0, the character offset of `𐐀`
 * is 1 and the character offset of b is 3 since `𐐀` is represented using two code
 * units in UTF-16.
 *
 * Positions are line end character agnostic. So you can not specify a position that
 * denotes `\r|\n` or `\n|` where `|` represents the character offset.
 */
type Position struct {

	// Line position in a document (zero-based).
	Line uint `json:"line"`

	// Character offset on a line in a document (zero-based). Assuming that the line is
	// represented as a string, the `character` value represents the gap between the
	// `character` and `character + 1`.
	//
	// If the character value is greater than the line length it defaults back to the
	// line length.
	Character uint `json:"character"`
}

/**
 * A range in a text document expressed as (zero-based) start and end positions.
 *
 * If you want to specify a range that contains a line including the line ending
 * character(s) then use an end position denoting the start of the next line.
 * For example:
 * ```ts
 * {
 *     start: { line: 5, character: 23 }
 *     end : { line 6, character : 0 }
 * }
 * ```
 */
type Range struct {

	// The range's start position
	Start Position `json:"start,omitempty"`

	// The range's end position.
	End Position `json:"end,omitempty"`
}

/**
 * Represents a location inside a resource, such as a line
 * inside a text file.
 */
type Location struct {
	Uri DocumentUri `json:"uri,omitempty"`

	Range Range `json:"range,omitempty"`
}

/**
 * Represents the connection of two locations. Provides additional metadata over normal [locations](#Location),
 * including an origin range.
 */
type LocationLink struct {

	// Span of the origin of this link.
	//
	// Used as the underlined span for mouse definition hover. Defaults to the word range at
	// the definition position.
	OriginSelectionRange *Range `json:"originSelectionRange,omitempty"`

	// The target resource identifier of this link.
	TargetUri DocumentUri `json:"targetUri,omitempty"`

	// The full target range of this link. If the target for example is a symbol then target range is the
	// range enclosing this symbol not including leadingtrailing whitespace but everything else
	// like comments. This information is typically used to highlight the range in the editor.
	TargetRange Range `json:"targetRange,omitempty"`

	// The range that should be selected and revealed when this link is being followed, e.g the name of a function.
	// Must be contained by the the `targetRange`. See also `DocumentSymbol#range`
	TargetSelectionRange Range `json:"targetSelectionRange,omitempty"`
}

/**
 * Represents a color in RGBA space.
 */
type Color struct {

	// The red component of this color in the range [0-1].
	Red float32 `json:"red,omitempty"`

	// The green component of this color in the range [0-1].
	Green float32 `json:"green,omitempty"`

	// The blue component of this color in the range [0-1].
	Blue float32 `json:"blue,omitempty"`

	// The alpha component of this color in the range [0-1].
	Alpha float32 `json:"alpha,omitempty"`
}

/**
 * Represents a color range from a document.
 */
type ColorInformation struct {

	// The range in the document where this color appears.
	Range Range `json:"range,omitempty"`

	// The actual color value for this color range.
	Color Color `json:"color,omitempty"`
}

type ColorPresentation struct {

	// The label of this color presentation. It will be shown on the color
	// picker header. By default this is also the text that is inserted when selecting
	// this color presentation.
	Label string `json:"label,omitempty"`

	// An [edit](#TextEdit) which is applied to a document when selecting
	// this presentation for the color.  When `falsy` the [label](#ColorPresentation.label)
	// is used.
	TextEdit *TextEdit `json:"textEdit,omitempty"`

	// An optional array of additional [text edits](#TextEdit) that are applied when
	// selecting this color presentation. Edits must not overlap with the main [edit](#ColorPresentation.textEdit) nor with themselves.
	AdditionalTextEdits *[]TextEdit `json:"additionalTextEdits,omitempty"`
}

/**
 * Represents a folding range. To be valid, start and end line must be bigger than zero and smaller
 * than the number of lines in the document. Clients are free to ignore invalid ranges.
 */
type FoldingRange struct {

	// The zero-based start line of the range to fold. The folded area starts after the line's last character.
	// To be valid, the end must be zero or larger and smaller than the number of lines in the document.
	StartLine uint `json:"startLine,omitempty"`

	// The zero-based character offset from where the folded range starts. If not defined, defaults to the length of the start line.
	StartCharacter *uint `json:"startCharacter,omitempty"`

	// The zero-based end line of the range to fold. The folded area ends with the line's last character.
	// To be valid, the end must be zero or larger and smaller than the number of lines in the document.
	EndLine uint `json:"endLine,omitempty"`

	// The zero-based character offset before the folded range ends. If not defined, defaults to the length of the end line.
	EndCharacter *uint `json:"endCharacter,omitempty"`

	// Describes the kind of the folding range such as `comment' or 'region'. The kind
	// is used to categorize folding ranges and used by commands like 'Fold all comments'. See
	// [FoldingRangeKind](#FoldingRangeKind) for an enumeration of standardized kinds.
	Kind *string `json:"kind,omitempty"`
}

/**
 * Represents a related message and source code location for a diagnostic. This should be
 * used to point to code locations that cause or related to a diagnostics, e.g when duplicating
 * a symbol in a scope.
 */
type DiagnosticRelatedInformation struct {

	// The location of this related diagnostic information.
	Location Location `json:"location,omitempty"`

	// The message of this related diagnostic information.
	Message string `json:"message,omitempty"`
}

/**
 * Structure to capture a description for an error code.
 *
 * @since 3.16.0
 */
type CodeDescription struct {

	// An URI to open with more information about the diagnostic error.
	Href URI `json:"href,omitempty"`
}

/**
 * Represents a diagnostic, such as a compiler error or warning. Diagnostic objects
 * are only valid in the scope of a resource.
 */
type Diagnostic struct {

	// The range at which the message applies
	Range Range `json:"range,omitempty"`

	// The diagnostic's severity. Can be omitted. If omitted it is up to the
	// client to interpret diagnostics as error, warning, info or hint.
	Severity *DiagnosticSeverity `json:"severity,omitempty"`

	// The diagnostic's code, which usually appear in the user interface.
	Code interface{} `json:"code,omitempty"` // int, string,

	// An optional property to describe the error code.
	// Requires the code field (above) to be presentnot null.
	//
	// @since 3.16.0
	CodeDescription *CodeDescription `json:"codeDescription,omitempty"`

	// A human-readable string describing the source of this
	// diagnostic, e.g. 'typescript' or 'super lint'. It usually
	// appears in the user interface.
	Source *string `json:"source,omitempty"`

	// The diagnostic's message. It usually appears in the user interface
	Message string `json:"message,omitempty"`

	// Additional metadata about the diagnostic.
	//
	// @since 3.15.0
	Tags *[]DiagnosticTag `json:"tags,omitempty"`

	// An array of related diagnostic information, e.g. when symbol-names within
	// a scope collide all definitions can be marked via this property.
	RelatedInformation *[]DiagnosticRelatedInformation `json:"relatedInformation,omitempty"`

	// A data entry field that is preserved between a `textDocumentpublishDiagnostics`
	// notification and `textDocumentcodeAction` request.
	//
	// @since 3.16.0
	Data interface{} `json:"data,omitempty"`
}

/**
 * Represents a reference to a command. Provides a title which
 * will be used to represent a command in the UI and, optionally,
 * an array of arguments which will be passed to the command handler
 * function when invoked.
 */
type Command struct {

	// Title of the command, like `save`.
	Title string `json:"title,omitempty"`

	// The identifier of the actual command handler.
	Command string `json:"command,omitempty"`

	// Arguments that the command handler should be
	// invoked with.
	Arguments *[]interface{} `json:"arguments,omitempty"`
}

/**
 * A text edit applicable to a text document.
 */
type TextEdit struct {

	// The range of the text document to be manipulated. To insert
	// text into a document create a range where start === end.
	Range Range `json:"range,omitempty"`

	// The string to be inserted. For delete operations use an
	// empty string.
	NewText string `json:"newText,omitempty"`
}

/**
 * Additional information that describes document changes.
 *
 * @since 3.16.0
 */
type ChangeAnnotation struct {

	// A human-readable string describing the actual change. The string
	// is rendered prominent in the user interface.
	Label string `json:"label,omitempty"`

	// A flag which indicates that user confirmation is needed
	// before applying the change.
	NeedsConfirmation *bool `json:"needsConfirmation,omitempty"`

	// A human-readable string which is rendered less prominent in
	// the user interface.
	Description *string `json:"description,omitempty"`
}

/**
 * A special text edit with an additional change annotation.
 *
 * @since 3.16.0.
 */
type AnnotatedTextEdit struct {
	TextEdit

	// The actual identifier of the change annotation
	AnnotationId ChangeAnnotationIdentifier `json:"annotationId,omitempty"`
}

/**
 * A generic resource operation.
 */
type ResourceOperation struct {

	// The resource operation kind.
	Kind string `json:"kind,omitempty"`

	// An optional annotation identifier describing the operation.
	//
	// @since 3.16.0
	AnnotationId *ChangeAnnotationIdentifier `json:"annotationId,omitempty"`
}

/**
 * Options to create a file.
 */
type CreateFileOptions struct {

	// Overwrite existing file. Overwrite wins over `ignoreIfExists`
	Overwrite *bool `json:"overwrite,omitempty"`

	// Ignore if exists.
	IgnoreIfExists *bool `json:"ignoreIfExists,omitempty"`
}

/**
 * Create file operation.
 */
type CreateFile struct {
	ResourceOperation

	// A create
	Kind interface{} `json:"kind,omitempty"` // 'create'

	// The resource to create.
	Uri DocumentUri `json:"uri,omitempty"`

	// Additional options
	Options *CreateFileOptions `json:"options,omitempty"`
}

/**
 * Rename file options
 */
type RenameFileOptions struct {

	// Overwrite target if existing. Overwrite wins over `ignoreIfExists`
	Overwrite *bool `json:"overwrite,omitempty"`

	// Ignores if target exists.
	IgnoreIfExists *bool `json:"ignoreIfExists,omitempty"`
}

/**
 * Rename file operation
 */
type RenameFile struct {
	ResourceOperation

	// A rename
	Kind interface{} `json:"kind,omitempty"` // 'rename'

	// The old (existing) location.
	OldUri DocumentUri `json:"oldUri,omitempty"`

	// The new location.
	NewUri DocumentUri `json:"newUri,omitempty"`

	// Rename options.
	Options *RenameFileOptions `json:"options,omitempty"`
}

/**
 * Delete file options
 */
type DeleteFileOptions struct {

	// Delete the content recursively if a folder is denoted.
	Recursive *bool `json:"recursive,omitempty"`

	// Ignore the operation if the file doesn't exist.
	IgnoreIfNotExists *bool `json:"ignoreIfNotExists,omitempty"`
}

/**
 * Delete file operation
 */
type DeleteFile struct {
	ResourceOperation

	// A delete
	Kind interface{} `json:"kind,omitempty"` // 'delete'

	// The file to delete.
	Uri DocumentUri `json:"uri,omitempty"`

	// Delete options.
	Options *DeleteFileOptions `json:"options,omitempty"`
}

/**
 * A literal to identify a text document in the client.
 */
type TextDocumentIdentifier struct {

	// The text document's uri.
	Uri DocumentUri `json:"uri,omitempty"`
}

/**
 * A text document identifier to denote a specific version of a text document.
 */
type VersionedTextDocumentIdentifier struct {
	TextDocumentIdentifier

	// The version number of this document.
	Version int `json:"version,omitempty"`
}

/**
 * A text document identifier to optionally denote a specific version of a text document.
 */
type OptionalVersionedTextDocumentIdentifier struct {
	TextDocumentIdentifier

	// The version number of this document. If a versioned text document identifier
	// is sent from the server to the client and the file is not open in the editor
	// (the server has not received an open notification before) the server can send
	// `null` to indicate that the version is unknown and the content on disk is the
	// truth (as specified with document content ownership).
	Version interface{} `json:"version,omitempty"` // int, null,
}

/**
 * An item to transfer a text document from the client to the
 * server.
 */
type TextDocumentItem struct {

	// The text document's uri.
	Uri DocumentUri `json:"uri,omitempty"`

	// The text document's language identifier
	LanguageId string `json:"languageId,omitempty"`

	// The version number of this document (it will increase after each
	// change, including undoredo).
	Version int `json:"version,omitempty"`

	// The content of the opened text document.
	Text string `json:"text,omitempty"`
}

/**
 * A `MarkupContent` literal represents a string value which content is interpreted base on its
 * kind flag. Currently the protocol supports `plaintext` and `markdown` as markup kinds.
 *
 * If the kind is `markdown` then the value can contain fenced code blocks like in GitHub issues.
 * See https://help.github.com/articles/creating-and-highlighting-code-blocks/#syntax-highlighting
 *
 * Here is an example how such a string can be constructed using JavaScript / TypeScript:
 * ```ts
 * let markdown: MarkdownContent = {
 *  kind: MarkupKind.Markdown,
 *	value: [
 *		'# Header',
 *		'Some text',
 *		'```typescript',
 *		'someCode();',
 *		'```'
 *	].join('\n')
 * };
 * ```
 *
 * *Please Note* that clients might sanitize the return markdown. A client could decide to
 * remove HTML from the markdown to avoid script execution.
 */
type MarkupContent struct {

	// The type of the Markup
	Kind MarkupKind `json:"kind,omitempty"`

	// The content itself
	Value string `json:"value,omitempty"`
}

/**
 * A special text edit to provide an insert and a replace operation.
 *
 * @since 3.16.0
 */
type InsertReplaceEdit struct {

	// The string to be inserted.
	NewText string `json:"newText,omitempty"`

	// The range if the insert is requested
	Insert Range `json:"insert,omitempty"`

	// The range if the replace is requested.
	Replace Range `json:"replace,omitempty"`
}

/**
 * Additional details for a completion item label.
 *
 * @since 3.17.0 - proposed state
 */
type CompletionItemLabelDetails struct {

	// An optional string which is rendered less prominently directly after {@link CompletionItem.label label},
	// without any spacing. Should be used for function signatures or type annotations.
	Detail *string `json:"detail,omitempty"`

	// An optional string which is rendered less prominently after {@link CompletionItem.detail}. Should be used
	// for fully qualified names or file path.
	Description *string `json:"description,omitempty"`
}

/**
 * A completion item represents a text snippet that is
 * proposed to complete text that is being typed.
 */
type CompletionItem struct {

	// The label of this completion item.
	//
	// The label property is also by default the text that
	// is inserted when selecting this completion.
	//
	// If label details are provided the label itself should
	// be an unqualified name of the completion item.
	Label string `json:"label,omitempty"`

	// Additional details for the label
	//
	// @since 3.17.0 - proposed state
	LabelDetails *CompletionItemLabelDetails `json:"labelDetails,omitempty"`

	// The kind of this completion item. Based of the kind
	// an icon is chosen by the editor.
	Kind *CompletionItemKind `json:"kind,omitempty"`

	// Tags for this completion item.
	//
	// @since 3.15.0
	Tags *[]CompletionItemTag `json:"tags,omitempty"`

	// A human-readable string with additional information
	// about this item, like type or symbol information.
	Detail *string `json:"detail,omitempty"`

	// A human-readable string that represents a doc-comment.
	Documentation interface{} `json:"documentation,omitempty"` // string, MarkupContent,

	// Indicates if this item is deprecated.
	// @deprecated Use `tags` instead.
	Deprecated *bool `json:"deprecated,omitempty"`

	// Select this item when showing.
	//
	// Note that only one completion item can be selected and that the
	// tool  client decides which item that is. The rule is that the first
	// item of those that match best is selected.
	Preselect *bool `json:"preselect,omitempty"`

	// A string that should be used when comparing this item
	// with other items. When `falsy` the [label](#CompletionItem.label)
	// is used.
	SortText *string `json:"sortText,omitempty"`

	// A string that should be used when filtering a set of
	// completion items. When `falsy` the [label](#CompletionItem.label)
	// is used.
	FilterText *string `json:"filterText,omitempty"`

	// A string that should be inserted into a document when selecting
	// this completion. When `falsy` the [label](#CompletionItem.label)
	// is used.
	//
	// The `insertText` is subject to interpretation by the client side.
	// Some tools might not take the string literally. For example
	// VS Code when code complete is requested in this example `con<cursor position>`
	// and a completion item with an `insertText` of `console` is provided it
	// will only insert `sole`. Therefore it is recommended to use `textEdit` instead
	// since it avoids additional client side interpretation.
	InsertText *string `json:"insertText,omitempty"`

	// The format of the insert text. The format applies to both the `insertText` property
	// and the `newText` property of a provided `textEdit`. If omitted defaults to
	// `InsertTextFormat.PlainText`.
	//
	// Please note that the insertTextFormat doesn't apply to `additionalTextEdits`.
	InsertTextFormat *InsertTextFormat `json:"insertTextFormat,omitempty"`

	// How whitespace and indentation is handled during completion
	// item insertion. If ignored the clients default value depends on
	// the `textDocument.completion.insertTextMode` client capability.
	//
	// @since 3.16.0
	InsertTextMode *InsertTextMode `json:"insertTextMode,omitempty"`

	// An [edit](#TextEdit) which is applied to a document when selecting
	// this completion. When an edit is provided the value of
	// [insertText](#CompletionItem.insertText) is ignored.
	//
	// Most editors support two different operation when accepting a completion item. One is to insert a
	// completion text and the other is to replace an existing text with a completion text. Since this can
	// usually not predetermined by a server it can report both ranges. Clients need to signal support for
	// `InsertReplaceEdits` via the `textDocument.completion.insertReplaceSupport` client capability
	// property.
	//
	// Note 1: The text edit's range as well as both ranges from a insert replace edit must be a
	// [single line] and they must contain the position at which completion has been requested.
	// Note 2: If an `InsertReplaceEdit` is returned the edit's insert range must be a prefix of
	// the edit's replace range, that means it must be contained and starting at the same position.
	//
	// @since 3.16.0 additional type `InsertReplaceEdit`
	TextEdit interface{} `json:"textEdit,omitempty"` // TextEdit, InsertReplaceEdit,

	// An optional array of additional [text edits](#TextEdit) that are applied when
	// selecting this completion. Edits must not overlap (including the same insert position)
	// with the main [edit](#CompletionItem.textEdit) nor with themselves.
	//
	// Additional text edits should be used to change text unrelated to the current cursor position
	// (for example adding an import statement at the top of the file if the completion item will
	// insert an unqualified type).
	AdditionalTextEdits *[]TextEdit `json:"additionalTextEdits,omitempty"`

	// An optional set of characters that when pressed while this completion is active will accept it first and
	// then type that character. Note that all commit characters should have `length=1` and that superfluous
	// characters will be ignored.
	CommitCharacters *[]string `json:"commitCharacters,omitempty"`

	// An optional [command](#Command) that is executed after inserting this completion. Note that
	// additional modifications to the current document should be described with the
	// [additionalTextEdits](#CompletionItem.additionalTextEdits)-property.
	Command *Command `json:"command,omitempty"`

	// A data entry field that is preserved on a completion item between a
	// [CompletionRequest](#CompletionRequest) and a [CompletionResolveRequest](#CompletionResolveRequest).
	Data interface{} `json:"data,omitempty"`
}

/**
 * Represents a collection of [completion items](#CompletionItem) to be presented
 * in the editor.
 */
type CompletionList struct {

	// This list it not complete. Further typing results in recomputing this list.
	IsIncomplete bool `json:"isIncomplete,omitempty"`

	// The completion items.
	Items []CompletionItem `json:"items,omitempty"`
}

/**
 * The result of a hover request.
 */
type Hover struct {

	// The hover's content
	Contents interface{} `json:"contents,omitempty"` // MarkupContent, MarkedString, []MarkedString,

	// An optional range
	Range *Range `json:"range,omitempty"`
}

/**
 * Represents the signature of something callable. A signature
 * can have a label, like a function-name, a doc-comment, and
 * a set of parameters.
 */
type SignatureInformation struct {

	// The label of this signature. Will be shown in
	// the UI.
	Label string `json:"label,omitempty"`

	// The human-readable doc-comment of this signature. Will be shown
	// in the UI but can be omitted.
	Documentation interface{} `json:"documentation,omitempty"` // string, MarkupContent,

	// The parameters of this signature.
	Parameters *[]ParameterInformation `json:"parameters,omitempty"`

	// The index of the active parameter.
	//
	// If provided, this is used in place of `SignatureHelp.activeParameter`.
	//
	// @since 3.16.0
	ActiveParameter *uint `json:"activeParameter,omitempty"`
}

/**
 * Signature help represents the signature of something
 * callable. There can be multiple signature but only one
 * active and only one active parameter.
 */
type SignatureHelp struct {

	// One or more signatures.
	Signatures []SignatureInformation `json:"signatures,omitempty"`

	// The active signature. If omitted or the value lies outside the
	// range of `signatures` the value defaults to zero or is ignored if
	// the `SignatureHelp` has no signatures.
	//
	// Whenever possible implementors should make an active decision about
	// the active signature and shouldn't rely on a default value.
	//
	// In future version of the protocol this property might become
	// mandatory to better express this.
	ActiveSignature *uint `json:"activeSignature,omitempty"`

	// The active parameter of the active signature. If omitted or the value
	// lies outside the range of `signatures[activeSignature].parameters`
	// defaults to 0 if the active signature has parameters. If
	// the active signature has no parameters it is ignored.
	// In future version of the protocol this property might become
	// mandatory to better express the active parameter if the
	// active signature does have any.
	ActiveParameter *uint `json:"activeParameter,omitempty"`
}

/**
 * Value-object that contains additional information when
 * requesting references.
 */
type ReferenceContext struct {

	// Include the declaration of the current symbol.
	IncludeDeclaration bool `json:"includeDeclaration,omitempty"`
}

/**
 * A document highlight is a range inside a text document which deserves
 * special attention. Usually a document highlight is visualized by changing
 * the background color of its range.
 */
type DocumentHighlight struct {

	// The range this highlight applies to.
	Range Range `json:"range,omitempty"`

	// The highlight kind, default is [text](#DocumentHighlightKind.Text).
	Kind *DocumentHighlightKind `json:"kind,omitempty"`
}

/**
 * Represents information about programming constructs like variables, classes,
 * interfaces etc.
 */
type SymbolInformation struct {

	// The name of this symbol.
	Name string `json:"name,omitempty"`

	// The kind of this symbol.
	Kind SymbolKind `json:"kind,omitempty"`

	// Tags for this completion item.
	//
	// @since 3.16.0
	Tags *[]SymbolTag `json:"tags,omitempty"`

	// Indicates if this symbol is deprecated.
	//
	// @deprecated Use tags instead
	Deprecated *bool `json:"deprecated,omitempty"`

	// The location of this symbol. The location's range is used by a tool
	// to reveal the location in the editor. If the symbol is selected in the
	// tool the range's start information is used to position the cursor. So
	// the range usually spans more than the actual symbol's name and does
	// normally include thinks like visibility modifiers.
	//
	// The range doesn't have to denote a node range in the sense of a abstract
	// syntax tree. It can therefore not be used to re-construct a hierarchy of
	// the symbols.
	Location Location `json:"location,omitempty"`

	// The name of the symbol containing this symbol. This information is for
	// user interface purposes (e.g. to render a qualifier in the user interface
	// if necessary). It can't be used to re-infer a hierarchy for the document
	// symbols.
	ContainerName *string `json:"containerName,omitempty"`
}

/**
 * Represents programming constructs like variables, classes, interfaces etc.
 * that appear in a document. Document symbols can be hierarchical and they
 * have two ranges: one that encloses its definition and one that points to
 * its most interesting range, e.g. the range of an identifier.
 */
type DocumentSymbol struct {

	// The name of this symbol. Will be displayed in the user interface and therefore must not be
	// an empty string or a string only consisting of white spaces.
	Name string `json:"name,omitempty"`

	// More detail for this symbol, e.g the signature of a function.
	Detail *string `json:"detail,omitempty"`

	// The kind of this symbol.
	Kind SymbolKind `json:"kind,omitempty"`

	// Tags for this document symbol.
	//
	// @since 3.16.0
	Tags *[]SymbolTag `json:"tags,omitempty"`

	// Indicates if this symbol is deprecated.
	//
	// @deprecated Use tags instead
	Deprecated *bool `json:"deprecated,omitempty"`

	// The range enclosing this symbol not including leadingtrailing whitespace but everything else
	// like comments. This information is typically used to determine if the the clients cursor is
	// inside the symbol to reveal in the symbol in the UI.
	Range Range `json:"range,omitempty"`

	// The range that should be selected and revealed when this symbol is being picked, e.g the name of a function.
	// Must be contained by the the `range`.
	SelectionRange Range `json:"selectionRange,omitempty"`

	// Children of this symbol, e.g. properties of a class.
	Children *[]DocumentSymbol `json:"children,omitempty"`
}

/**
 * Contains additional diagnostic information about the context in which
 * a [code action](#CodeActionProvider.provideCodeActions) is run.
 */
type CodeActionContext struct {

	// An array of diagnostics known on the client side overlapping the range provided to the
	// `textDocumentcodeAction` request. They are provided so that the server knows which
	// errors are currently presented to the user for the given range. There is no guarantee
	// that these accurately reflect the error state of the resource. The primary parameter
	// to compute code actions is the provided range.
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`

	// Requested kind of actions to return.
	//
	// Actions not of this kind are filtered out by the client before being shown. So servers
	// can omit computing them.
	Only *[]CodeActionKind `json:"only,omitempty"`
}

/**
 * A code action represents a change that can be performed in code, e.g. to fix a problem or
 * to refactor code.
 *
 * A CodeAction must set either `edit` and/or a `command`. If both are supplied, the `edit` is applied first, then the `command` is executed.
 */
type CodeAction struct {

	// A short, human-readable, title for this code action.
	Title string `json:"title,omitempty"`

	// The kind of the code action.
	//
	// Used to filter code actions.
	Kind *CodeActionKind `json:"kind,omitempty"`

	// The diagnostics that this code action resolves.
	Diagnostics *[]Diagnostic `json:"diagnostics,omitempty"`

	// Marks this as a preferred action. Preferred actions are used by the `auto fix` command and can be targeted
	// by keybindings.
	//
	// A quick fix should be marked preferred if it properly addresses the underlying error.
	// A refactoring should be marked preferred if it is the most reasonable choice of actions to take.
	//
	// @since 3.15.0
	IsPreferred *bool `json:"isPreferred,omitempty"`

	// Marks that the code action cannot currently be applied.
	//
	// Clients should follow the following guidelines regarding disabled code actions:
	//
	// - Disabled code actions are not shown in automatic [lightbulb](https:code.visualstudio.comdocseditoreditingevolved#_code-action)
	// code action menu.
	//
	// - Disabled actions are shown as faded out in the code action menu when the user request a more specific type
	// of code action, such as refactorings.
	//
	// - If the user has a [keybinding](https:code.visualstudio.comdocseditorrefactoring#_keybindings-for-code-actions)
	// that auto applies a code action and only a disabled code actions are returned, the client should show the user an
	// error message with `reason` in the editor.
	//
	// @since 3.16.0
	Disabled *struct {

		// Human readable description of why the code action is currently disabled.
		//
		// This is displayed in the code actions UI.
		Reason string `json:"reason,omitempty"`
	} `json:"disabled,omitempty"`

	// The workspace edit this code action performs.
	Edit *WorkspaceEdit `json:"edit,omitempty"`

	// A command this code action executes. If a code action
	// provides a edit and a command, first the edit is
	// executed and then the command.
	Command *Command `json:"command,omitempty"`

	// A data entry field that is preserved on a code action between
	// a `textDocumentcodeAction` and a `codeActionresolve` request.
	//
	// @since 3.16.0
	Data interface{} `json:"data,omitempty"`
}

/**
 * A code lens represents a [command](#Command) that should be shown along with
 * source text, like the number of references, a way to run tests, etc.
 *
 * A code lens is _unresolved_ when no command is associated to it. For performance
 * reasons the creation of a code lens and resolving should be done to two stages.
 */
type CodeLens struct {

	// The range in which this code lens is valid. Should only span a single line.
	Range Range `json:"range,omitempty"`

	// The command this code lens represents.
	Command *Command `json:"command,omitempty"`

	// A data entry field that is preserved on a code lens item between
	// a [CodeLensRequest](#CodeLensRequest) and a [CodeLensResolveRequest]
	// (#CodeLensResolveRequest)
	Data interface{} `json:"data,omitempty"`
}

/**
 * Value-object describing what options formatting should use.
 */
type FormattingOptions struct {

	// Size of a tab in spaces.
	TabSize uint `json:"tabSize,omitempty"`

	// Prefer spaces over tabs.
	InsertSpaces bool `json:"insertSpaces,omitempty"`

	// Trim trailing whitespaces on a line.
	//
	// @since 3.15.0
	TrimTrailingWhitespace *bool `json:"trimTrailingWhitespace,omitempty"`

	// Insert a newline character at the end of the file if one does not exist.
	//
	// @since 3.15.0
	InsertFinalNewline *bool `json:"insertFinalNewline,omitempty"`

	// Trim all newlines after the final newline at the end of the file.
	//
	// @since 3.15.0
	TrimFinalNewlines *bool `json:"trimFinalNewlines,omitempty"`

	// Signature for further properties.
	Key interface{} `json:"key,omitempty"` // bool, int, string, undefined,
}

/**
 * A document link is a range in a text document that links to an internal or external resource, like another
 * text document or a web site.
 */
type DocumentLink struct {

	// The range this link applies to.
	Range Range `json:"range,omitempty"`

	// The uri this link points to.
	Target *string `json:"target,omitempty"`

	// The tooltip text when you hover over this link.
	//
	// If a tooltip is provided, is will be displayed in a string that includes instructions on how to
	// trigger the link, such as `{0} (ctrl + click)`. The specific instructions vary depending on OS,
	// user settings, and localization.
	//
	// @since 3.15.0
	Tooltip *string `json:"tooltip,omitempty"`

	// A data entry field that is preserved on a document link between a
	// DocumentLinkRequest and a DocumentLinkResolveRequest.
	Data interface{} `json:"data,omitempty"`
}

/**
 * A selection range represents a part of a selection hierarchy. A selection range
 * may have a parent selection range that contains it.
 */
type SelectionRange struct {

	// The [range](#Range) of this selection range.
	Range Range `json:"range,omitempty"`

	// The parent selection range containing this range. Therefore `parent.range` must contain `this.range`.
	Parent *SelectionRange `json:"parent,omitempty"`
}

/**
 * Represents programming constructs like functions or constructors in the context
 * of call hierarchy.
 *
 * @since 3.16.0
 */
type CallHierarchyItem struct {

	// The name of this item.
	Name string `json:"name,omitempty"`

	// The kind of this item.
	Kind SymbolKind `json:"kind,omitempty"`

	// Tags for this item.
	Tags *[]SymbolTag `json:"tags,omitempty"`

	// More detail for this item, e.g. the signature of a function.
	Detail *string `json:"detail,omitempty"`

	// The resource identifier of this item.
	Uri DocumentUri `json:"uri,omitempty"`

	// The range enclosing this symbol not including leadingtrailing whitespace but everything else, e.g. comments and code.
	Range Range `json:"range,omitempty"`

	// The range that should be selected and revealed when this symbol is being picked, e.g. the name of a function.
	// Must be contained by the [`range`](#CallHierarchyItem.range).
	SelectionRange Range `json:"selectionRange,omitempty"`

	// A data entry field that is preserved between a call hierarchy prepare and
	// incoming calls or outgoing calls requests.
	Data interface{} `json:"data,omitempty"`
}

/**
 * Represents an incoming call, e.g. a caller of a method or constructor.
 *
 * @since 3.16.0
 */
type CallHierarchyIncomingCall struct {

	// The item that makes the call.
	From CallHierarchyItem `json:"from,omitempty"`

	// The ranges at which the calls appear. This is relative to the caller
	// denoted by [`this.from`](#CallHierarchyIncomingCall.from).
	FromRanges []Range `json:"fromRanges,omitempty"`
}

/**
 * Represents an outgoing call, e.g. calling a getter from a method or a method from a constructor etc.
 *
 * @since 3.16.0
 */
type CallHierarchyOutgoingCall struct {

	// The item that is called.
	To CallHierarchyItem `json:"to,omitempty"`

	// The range at which this item is called. This is the range relative to the caller, e.g the item
	// passed to [`provideCallHierarchyOutgoingCalls`](#CallHierarchyItemProvider.provideCallHierarchyOutgoingCalls)
	// and not [`this.to`](#CallHierarchyOutgoingCall.to).
	FromRanges []Range `json:"fromRanges,omitempty"`
}

/**
 * @since 3.16.0
 */
type SemanticTokensLegend struct {

	// The token types a server uses.
	TokenTypes []string `json:"tokenTypes,omitempty"`

	// The token modifiers a server uses.
	TokenModifiers []string `json:"tokenModifiers,omitempty"`
}

/**
 * @since 3.16.0
 */
type SemanticTokens struct {

	// An optional result id. If provided and clients support delta updating
	// the client will include the result id in the next semantic token request.
	// A server can then instead of computing all semantic tokens again simply
	// send a delta.
	ResultId *string `json:"resultId,omitempty"`

	// The actual tokens.
	Data []uint `json:"data,omitempty"`
}

/**
 * @since 3.16.0
 */
type SemanticTokensEdit struct {

	// The start offset of the edit.
	Start uint `json:"start,omitempty"`

	// The count of elements to remove.
	DeleteCount uint `json:"deleteCount,omitempty"`

	// The elements to insert.
	Data *[]uint `json:"data,omitempty"`
}

/**
 * @since 3.16.0
 */
type SemanticTokensDelta struct {
	ResultId *string `json:"resultId,omitempty"`

	// The semantic token edits to transform a previous result into a new result.
	Edits []SemanticTokensEdit `json:"edits,omitempty"`
}

/**
 * @since 3.17.0 - proposed state
 */
type TypeHierarchyItem struct {

	// The name of this item.
	Name string `json:"name,omitempty"`

	// The kind of this item.
	Kind SymbolKind `json:"kind,omitempty"`

	// Tags for this item.
	Tags *[]SymbolTag `json:"tags,omitempty"`

	// More detail for this item, e.g. the signature of a function.
	Detail *string `json:"detail,omitempty"`

	// The resource identifier of this item.
	Uri DocumentUri `json:"uri,omitempty"`

	// The range enclosing this symbol not including leadingtrailing whitespace
	// but everything else, e.g. comments and code.
	Range Range `json:"range,omitempty"`

	// The range that should be selected and revealed when this symbol is being
	// picked, e.g. the name of a function. Must be contained by the
	// [`range`](#TypeHierarchyItem.range).
	SelectionRange Range `json:"selectionRange,omitempty"`

	// A data entry field that is preserved between a type hierarchy prepare and
	// supertypes or subtypes requests. It could also be used to identify the
	// type hierarchy in the server, helping improve the performance on
	// resolving supertypes and subtypes.
	Data interface{} `json:"data,omitempty"`
}

/**
 * Provide inline value as text.
 *
 * @since 3.17.0 - proposed state
 */
type InlineValueText struct {

	// The document range for which the inline value applies.
	Range Range `json:"range,omitempty"`

	// The text of the inline value.
	Text string `json:"text,omitempty"`
}

/**
 * Provide inline value through a variable lookup.
 * If only a range is specified, the variable name will be extracted from the underlying document.
 * An optional variable name can be used to override the extracted name.
 *
 * @since 3.17.0 - proposed state
 */
type InlineValueVariableLookup struct {

	// The document range for which the inline value applies.
	// The range is used to extract the variable name from the underlying document.
	Range Range `json:"range,omitempty"`

	// If specified the name of the variable to look up.
	VariableName *string `json:"variableName,omitempty"`

	// How to perform the lookup.
	CaseSensitiveLookup bool `json:"caseSensitiveLookup,omitempty"`
}

/**
 * Provide an inline value through an expression evaluation.
 * If only a range is specified, the expression will be extracted from the underlying document.
 * An optional expression can be used to override the extracted expression.
 *
 * @since 3.17.0 - proposed state
 */
type InlineValueEvaluatableExpression struct {

	// The document range for which the inline value applies.
	// The range is used to extract the evaluatable expression from the underlying document.
	Range Range `json:"range,omitempty"`

	// If specified the expression overrides the extracted expression.
	Expression *string `json:"expression,omitempty"`
}

/**
 * @since 3.17.0 - proposed state
 */
type InlineValuesContext struct {

	// The document range where execution has stopped.
	// Typically the end position of the range denotes the line where the inline values are shown.
	StoppedLocation Range `json:"stoppedLocation,omitempty"`
}

type integer int

var integerStringMap = map[integer]string{
	integerMIN_VALUE: "MIN_VALUE",
	integerMAX_VALUE: "MAX_VALUE",
}

func (i integer) String() string {
	if s, ok := integerStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	integerMIN_VALUE integer = -2147483648

	integerMAX_VALUE integer = 2147483647
)

type uinteger int

var uintegerStringMap = map[uinteger]string{
	uintegerMIN_VALUE: "MIN_VALUE",
	uintegerMAX_VALUE: "MAX_VALUE",
}

func (i uinteger) String() string {
	if s, ok := uintegerStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	uintegerMIN_VALUE uinteger = 0

	uintegerMAX_VALUE uinteger = 2147483647
)

/**
 * Enum of known range kinds
 */
type FoldingRangeKind string

const (
	/**
	 * Folding range for a comment
	 */
	FoldingRangeKindComment FoldingRangeKind = "comment"
	/**
	 * Folding range for a imports or includes
	 */
	FoldingRangeKindImports FoldingRangeKind = "imports"
	/**
	 * Folding range for a region (e.g. `#region`)
	 */
	FoldingRangeKindRegion FoldingRangeKind = "region"
)

/**
 * The diagnostic's severity.
 */
type DiagnosticSeverity int

var diagnosticSeverityStringMap = map[DiagnosticSeverity]string{
	DiagnosticSeverityError:       "Error",
	DiagnosticSeverityWarning:     "Warning",
	DiagnosticSeverityInformation: "Information",
	DiagnosticSeverityHint:        "Hint",
}

func (i DiagnosticSeverity) String() string {
	if s, ok := diagnosticSeverityStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * Reports an error.
	 */
	DiagnosticSeverityError DiagnosticSeverity = 1
	/**
	 * Reports a warning.
	 */
	DiagnosticSeverityWarning DiagnosticSeverity = 2
	/**
	 * Reports an information.
	 */
	DiagnosticSeverityInformation DiagnosticSeverity = 3
	/**
	 * Reports a hint.
	 */
	DiagnosticSeverityHint DiagnosticSeverity = 4
)

/**
 * The diagnostic tags.
 *
 * @since 3.15.0
 */
type DiagnosticTag int

var diagnosticTagStringMap = map[DiagnosticTag]string{
	DiagnosticTagUnnecessary: "Unnecessary",
	DiagnosticTagDeprecated:  "Deprecated",
}

func (i DiagnosticTag) String() string {
	if s, ok := diagnosticTagStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * Unused or unnecessary code.
	 *
	 * Clients are allowed to render diagnostics with this tag faded out instead of having
	 * an error squiggle.
	 */
	DiagnosticTagUnnecessary DiagnosticTag = 1
	/**
	 * Deprecated or obsolete code.
	 *
	 * Clients are allowed to rendered diagnostics with this tag strike through.
	 */
	DiagnosticTagDeprecated DiagnosticTag = 2
)

/**
 * A workspace edit represents changes to many resources managed in the workspace. The edit
 * should either provide `changes` or `documentChanges`. If documentChanges are present
 * they are preferred over `changes` if the client can handle versioned document edits.
 *
 * Since version 3.13.0 a workspace edit can contain resource operations as well. If resource
 * operations are present clients need to execute the operations in the order in which they
 * are provided. So a workspace edit for example can consist of the following two changes:
 * (1) a create file a.txt and (2) a text document edit which insert text into file a.txt.
 *
 * An invalid sequence (e.g. (1) delete file a.txt and (2) insert text into file a.txt) will
 * cause failure of the operation. How the client recovers from the failure is described by
 * the client capability: `workspace.workspaceEdit.failureHandling`
 */
type WorkspaceEdit struct {
	/**
	 * Holds changes to existing resources.
	 */
	Changes *map[string] /*uri*/ []TextEdit `json:"changes,omitempty"`
	/**
	 * Depending on the client capability `workspace.workspaceEdit.resourceOperations` document changes
	 * are either an array of `TextDocumentEdit`s to express changes to n different text documents
	 * where each text document edit addresses a specific version of a text document. Or it can contain
	 * above `TextDocumentEdit`s mixed with create, rename and delete file / folder operations.
	 *
	 * Whether a client supports versioned document edits is expressed via
	 * `workspace.workspaceEdit.documentChanges` client capability.
	 *
	 * If a client neither supports `documentChanges` nor `workspace.workspaceEdit.resourceOperations` then
	 * only plain `TextEdit`s using the `changes` property are supported.
	 */
	DocumentChanges *[]interface{} `json:"documentChanges,omitempty"` // (TextDocumentEdit | CreateFile | RenameFile | DeleteFile)
	/**
	 * A map of change annotations that can be referenced in `AnnotatedTextEdit`s or create, rename and
	 * delete file / folder operations.
	 *
	 * Whether clients honor this property depends on the client capability `workspace.changeAnnotationSupport`.
	 *
	 * @since 3.16.0
	 */
	ChangeAnnotations *map[ChangeAnnotationIdentifier]ChangeAnnotation `json:"changeAnnotations,omitempty"`
}

/**
 * Represents a parameter of a callable-signature. A parameter can
 * have a label and a doc-comment.
 */
type ParameterInformation struct {
	/**
	 * The label of this parameter information.
	 *
	 * Either a string or an inclusive start and exclusive end offsets within its containing
	 * signature label. (see SignatureInformation.label). The offsets are based on a UTF-16
	 * string representation as `Position` and `Range` does.
	 *
	 * *Note*: a label of type string should be a substring of its containing signature label.
	 * Its intended use case is to highlight the parameter label part in the `SignatureInformation.label`.
	 */
	Label interface{} `json:"label,omitempty"` // string | [int, int]
	/**
	 * The human-readable doc-comment of this signature. Will be shown
	 * in the UI but can be omitted.
	 */
	Documentation interface{} `json:"documentation,omitempty"` // string | MarkupContent
}

/**
 * Describes textual changes on a text document. A TextDocumentEdit describes all changes
 * on a document version Si and after they are applied move the document to version Si+1.
 * So the creator of a TextDocumentEdit doesn't need to sort the array of edits or do any
 * kind of ordering. However the edits must be non overlapping.
 */
type TextDocumentEdit struct {
	/**
	 * The text document to change.
	 */
	TextDocument OptionalVersionedTextDocumentIdentifier `json:"textDocument,omitempty"`

	/**
	 * The edits to be applied.
	 *
	 * @since 3.16.0 - support for AnnotatedTextEdit. This is guarded using a
	 * client capability.
	 */
	Edits []interface{} `json:"edits,omitempty"` // (TextEdit | AnnotatedTextEdit)[]
}

/**
 * A simple text document. Not to be implemented. The document keeps the content
 * as string.
 *
 * @deprecated Use the text document from the new vscode-languageserver-textdocument package.
 */
type TextDocument struct {

	// The associated URI for this document. Most documents have the __file__-scheme, indicating that they
	// represent files on disk. However, some documents may have other schemes indicating that they are not
	// available on disk.
	//
	// @readonly
	Uri DocumentUri `json:"uri,omitempty"`

	// The identifier of the language associated with this document.
	//
	// @readonly
	LanguageId string `json:"languageId,omitempty"`

	// The version number of this document (it will increase after each
	// change, including undoredo).
	//
	// @readonly
	Version int `json:"version,omitempty"`

	// The number of lines in this document.
	//
	// @readonly
	LineCount uint `json:"lineCount,omitempty"`
}

/**
 * Describes the content type that a client supports in various
 * result literals like `Hover`, `ParameterInfo` or `CompletionItem`.
 *
 * Please note that `MarkupKinds` must not start with a `$`. This kinds
 * are reserved for internal usage.
 */
type MarkupKind string

const (
	/**
	 * Plain text is supported as a content format
	 */
	MarkupKindPlainText MarkupKind = "plaintext"
	/**
	 * Markdown is supported as a content format
	 */
	MarkupKindMarkdown MarkupKind = "markdown"
)

/**
 * The kind of a completion entry.
 */
type CompletionItemKind int

var completionItemKindStringMap = map[CompletionItemKind]string{
	CompletionItemKindText:          "Text",
	CompletionItemKindMethod:        "Method",
	CompletionItemKindFunction:      "Function",
	CompletionItemKindConstructor:   "Constructor",
	CompletionItemKindField:         "Field",
	CompletionItemKindVariable:      "Variable",
	CompletionItemKindClass:         "Class",
	CompletionItemKindInterface:     "Interface",
	CompletionItemKindModule:        "Module",
	CompletionItemKindProperty:      "Property",
	CompletionItemKindUnit:          "Unit",
	CompletionItemKindValue:         "Value",
	CompletionItemKindEnum:          "Enum",
	CompletionItemKindKeyword:       "Keyword",
	CompletionItemKindSnippet:       "Snippet",
	CompletionItemKindColor:         "Color",
	CompletionItemKindFile:          "File",
	CompletionItemKindReference:     "Reference",
	CompletionItemKindFolder:        "Folder",
	CompletionItemKindEnumMember:    "EnumMember",
	CompletionItemKindConstant:      "Constant",
	CompletionItemKindStruct:        "Struct",
	CompletionItemKindEvent:         "Event",
	CompletionItemKindOperator:      "Operator",
	CompletionItemKindTypeParameter: "TypeParameter",
}

func (i CompletionItemKind) String() string {
	if s, ok := completionItemKindStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	CompletionItemKindText CompletionItemKind = 1

	CompletionItemKindMethod CompletionItemKind = 2

	CompletionItemKindFunction CompletionItemKind = 3

	CompletionItemKindConstructor CompletionItemKind = 4

	CompletionItemKindField CompletionItemKind = 5

	CompletionItemKindVariable CompletionItemKind = 6

	CompletionItemKindClass CompletionItemKind = 7

	CompletionItemKindInterface CompletionItemKind = 8

	CompletionItemKindModule CompletionItemKind = 9

	CompletionItemKindProperty CompletionItemKind = 10

	CompletionItemKindUnit CompletionItemKind = 11

	CompletionItemKindValue CompletionItemKind = 12

	CompletionItemKindEnum CompletionItemKind = 13

	CompletionItemKindKeyword CompletionItemKind = 14

	CompletionItemKindSnippet CompletionItemKind = 15

	CompletionItemKindColor CompletionItemKind = 16

	CompletionItemKindFile CompletionItemKind = 17

	CompletionItemKindReference CompletionItemKind = 18

	CompletionItemKindFolder CompletionItemKind = 19

	CompletionItemKindEnumMember CompletionItemKind = 20

	CompletionItemKindConstant CompletionItemKind = 21

	CompletionItemKindStruct CompletionItemKind = 22

	CompletionItemKindEvent CompletionItemKind = 23

	CompletionItemKindOperator CompletionItemKind = 24

	CompletionItemKindTypeParameter CompletionItemKind = 25
)

/**
 * Defines whether the insert text in a completion item should be interpreted as
 * plain text or a snippet.
 */
type InsertTextFormat int

var insertTextFormatStringMap = map[InsertTextFormat]string{
	InsertTextFormatPlainText: "PlainText",
	InsertTextFormatSnippet:   "Snippet",
}

func (i InsertTextFormat) String() string {
	if s, ok := insertTextFormatStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * The primary text to be inserted is treated as a plain string.
	 */
	InsertTextFormatPlainText InsertTextFormat = 1
	/**
	 * The primary text to be inserted is treated as a snippet.
	 *
	 * A snippet can define tab stops and placeholders with `$1`, `$2`
	 * and `${3:foo}`. `$0` defines the final tab stop, it defaults to
	 * the end of the snippet. Placeholders with equal identifiers are linked,
	 * that is typing in one will update others too.
	 *
	 * See also: https://microsoft.github.io/language-server-protocol/specifications/specification-current/#snippet_syntax
	 */
	InsertTextFormatSnippet InsertTextFormat = 2
)

/**
 * Completion item tags are extra annotations that tweak the rendering of a completion
 * item.
 *
 * @since 3.15.0
 */
type CompletionItemTag int

var completionItemTagStringMap = map[CompletionItemTag]string{
	CompletionItemTagDeprecated: "Deprecated",
}

func (i CompletionItemTag) String() string {
	if s, ok := completionItemTagStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * Render a completion as obsolete, usually using a strike-out.
	 */
	CompletionItemTagDeprecated CompletionItemTag = 1
)

/**
 * How whitespace and indentation is handled during completion
 * item insertion.
 *
 * @since 3.16.0
 */
type InsertTextMode int

var insertTextModeStringMap = map[InsertTextMode]string{
	InsertTextModeAsIs:              "asIs",
	InsertTextModeAdjustIndentation: "adjustIndentation",
}

func (i InsertTextMode) String() string {
	if s, ok := insertTextModeStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * The insertion or replace strings is taken as it is. If the
	 * value is multi line the lines below the cursor will be
	 * inserted using the indentation defined in the string value.
	 * The client will not apply any kind of adjustments to the
	 * string.
	 */
	InsertTextModeAsIs InsertTextMode = 1
	/**
	 * The editor adjusts leading whitespace of new lines so that
	 * they match the indentation up to the cursor of the line for
	 * which the item is accepted.
	 *
	 * Consider a line like this: <2tabs><cursor><3tabs>foo. Accepting a
	 * multi line completion item is indented using 2 tabs and all
	 * following lines inserted will be indented using 2 tabs as well.
	 */
	InsertTextModeAdjustIndentation InsertTextMode = 2
)

/**
 * A document highlight kind.
 */
type DocumentHighlightKind int

var documentHighlightKindStringMap = map[DocumentHighlightKind]string{
	DocumentHighlightKindText:  "Text",
	DocumentHighlightKindRead:  "Read",
	DocumentHighlightKindWrite: "Write",
}

func (i DocumentHighlightKind) String() string {
	if s, ok := documentHighlightKindStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * A textual occurrence.
	 */
	DocumentHighlightKindText DocumentHighlightKind = 1
	/**
	 * Read-access of a symbol, like reading a variable.
	 */
	DocumentHighlightKindRead DocumentHighlightKind = 2
	/**
	 * Write-access of a symbol, like writing to a variable.
	 */
	DocumentHighlightKindWrite DocumentHighlightKind = 3
)

/**
 * A symbol kind.
 */
type SymbolKind int

var symbolKindStringMap = map[SymbolKind]string{
	SymbolKindFile:          "File",
	SymbolKindModule:        "Module",
	SymbolKindNamespace:     "Namespace",
	SymbolKindPackage:       "Package",
	SymbolKindClass:         "Class",
	SymbolKindMethod:        "Method",
	SymbolKindProperty:      "Property",
	SymbolKindField:         "Field",
	SymbolKindConstructor:   "Constructor",
	SymbolKindEnum:          "Enum",
	SymbolKindInterface:     "Interface",
	SymbolKindFunction:      "Function",
	SymbolKindVariable:      "Variable",
	SymbolKindConstant:      "Constant",
	SymbolKindString:        "String",
	SymbolKindNumber:        "Number",
	SymbolKindBoolean:       "Boolean",
	SymbolKindArray:         "Array",
	SymbolKindObject:        "Object",
	SymbolKindKey:           "Key",
	SymbolKindNull:          "Null",
	SymbolKindEnumMember:    "EnumMember",
	SymbolKindStruct:        "Struct",
	SymbolKindEvent:         "Event",
	SymbolKindOperator:      "Operator",
	SymbolKindTypeParameter: "TypeParameter",
}

func (i SymbolKind) String() string {
	if s, ok := symbolKindStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	SymbolKindFile SymbolKind = 1

	SymbolKindModule SymbolKind = 2

	SymbolKindNamespace SymbolKind = 3

	SymbolKindPackage SymbolKind = 4

	SymbolKindClass SymbolKind = 5

	SymbolKindMethod SymbolKind = 6

	SymbolKindProperty SymbolKind = 7

	SymbolKindField SymbolKind = 8

	SymbolKindConstructor SymbolKind = 9

	SymbolKindEnum SymbolKind = 10

	SymbolKindInterface SymbolKind = 11

	SymbolKindFunction SymbolKind = 12

	SymbolKindVariable SymbolKind = 13

	SymbolKindConstant SymbolKind = 14

	SymbolKindString SymbolKind = 15

	SymbolKindNumber SymbolKind = 16

	SymbolKindBoolean SymbolKind = 17

	SymbolKindArray SymbolKind = 18

	SymbolKindObject SymbolKind = 19

	SymbolKindKey SymbolKind = 20

	SymbolKindNull SymbolKind = 21

	SymbolKindEnumMember SymbolKind = 22

	SymbolKindStruct SymbolKind = 23

	SymbolKindEvent SymbolKind = 24

	SymbolKindOperator SymbolKind = 25

	SymbolKindTypeParameter SymbolKind = 26
)

/**
 * Symbol tags are extra annotations that tweak the rendering of a symbol.
 * @since 3.16
 */
type SymbolTag int

var symbolTagStringMap = map[SymbolTag]string{
	SymbolTagDeprecated: "Deprecated",
}

func (i SymbolTag) String() string {
	if s, ok := symbolTagStringMap[i]; ok {
		return s
	}
	return "unknown"
}

const (
	/**
	 * Render a symbol as obsolete, usually using a strike-out.
	 */
	SymbolTagDeprecated SymbolTag = 1
)

/**
 * A set of predefined code action kinds
 */
type CodeActionKind string

const (
	/**
	 * Empty kind.
	 */
	CodeActionKindEmpty CodeActionKind = ""
	/**
	 * Base kind for quickfix actions: 'quickfix'
	 */
	CodeActionKindQuickFix CodeActionKind = "quickfix"
	/**
	 * Base kind for refactoring actions: 'refactor'
	 */
	CodeActionKindRefactor CodeActionKind = "refactor"
	/**
	 * Base kind for refactoring extraction actions: 'refactor.extract'
	 *
	 * Example extract actions:
	 *
	 * - Extract method
	 * - Extract function
	 * - Extract variable
	 * - Extract interface from class
	 * - ...
	 */
	CodeActionKindRefactorExtract CodeActionKind = "refactor.extract"
	/**
	 * Base kind for refactoring inline actions: 'refactor.inline'
	 *
	 * Example inline actions:
	 *
	 * - Inline function
	 * - Inline variable
	 * - Inline constant
	 * - ...
	 */
	CodeActionKindRefactorInline CodeActionKind = "refactor.inline"
	/**
	 * Base kind for refactoring rewrite actions: 'refactor.rewrite'
	 *
	 * Example rewrite actions:
	 *
	 * - Convert JavaScript function to class
	 * - Add or remove parameter
	 * - Encapsulate field
	 * - Make method static
	 * - Move method to base class
	 * - ...
	 */
	CodeActionKindRefactorRewrite CodeActionKind = "refactor.rewrite"
	/**
	 * Base kind for source actions: `source`
	 *
	 * Source code actions apply to the entire file.
	 */
	CodeActionKindSource CodeActionKind = "source"
	/**
	 * Base kind for an organize imports source action: `source.organizeImports`
	 */
	CodeActionKindSourceOrganizeImports CodeActionKind = "source.organizeImports"
	/**
	 * Base kind for auto-fix source actions: `source.fixAll`.
	 *
	 * Fix all actions automatically fix errors that have a clear fix that do not require user input.
	 * They should not suppress errors or perform unsafe fixes such as generating new types or classes.
	 *
	 * @since 3.15.0
	 */
	CodeActionKindSourceFixAll CodeActionKind = "source.fixAll"
)

/**
 * A set of predefined token types. This set is not fixed
 * an clients can specify additional token types via the
 * corresponding client capabilities.
 *
 * @since 3.16.0
 */
type SemanticTokenTypes string

const (
	SemanticTokenTypesNamespace SemanticTokenTypes = "namespace"
	/**
	 * Represents a generic type. Acts as a fallback for types which can't be mapped to
	 * a specific type like class or enum.
	 */
	SemanticTokenTypesType SemanticTokenTypes = "type"

	SemanticTokenTypesClass SemanticTokenTypes = "class"

	SemanticTokenTypesEnum SemanticTokenTypes = "enum"

	SemanticTokenTypesInterface SemanticTokenTypes = "interface"

	SemanticTokenTypesStruct SemanticTokenTypes = "struct"

	SemanticTokenTypesTypeParameter SemanticTokenTypes = "typeParameter"

	SemanticTokenTypesParameter SemanticTokenTypes = "parameter"

	SemanticTokenTypesVariable SemanticTokenTypes = "variable"

	SemanticTokenTypesProperty SemanticTokenTypes = "property"

	SemanticTokenTypesEnumMember SemanticTokenTypes = "enumMember"

	SemanticTokenTypesEvent SemanticTokenTypes = "event"

	SemanticTokenTypesFunction SemanticTokenTypes = "function"

	SemanticTokenTypesMethod SemanticTokenTypes = "method"

	SemanticTokenTypesMacro SemanticTokenTypes = "macro"

	SemanticTokenTypesKeyword SemanticTokenTypes = "keyword"

	SemanticTokenTypesModifier SemanticTokenTypes = "modifier"

	SemanticTokenTypesComment SemanticTokenTypes = "comment"

	SemanticTokenTypesString SemanticTokenTypes = "string"

	SemanticTokenTypesNumber SemanticTokenTypes = "number"

	SemanticTokenTypesRegexp SemanticTokenTypes = "regexp"

	SemanticTokenTypesOperator SemanticTokenTypes = "operator"
	/**
	 * @since 3.17.0
	 */
	SemanticTokenTypesDecorator SemanticTokenTypes = "decorator"
)

/**
 * A set of predefined token modifiers. This set is not fixed
 * an clients can specify additional token types via the
 * corresponding client capabilities.
 *
 * @since 3.16.0
 */
type SemanticTokenModifiers string

const (
	SemanticTokenModifiersDeclaration SemanticTokenModifiers = "declaration"

	SemanticTokenModifiersDefinition SemanticTokenModifiers = "definition"

	SemanticTokenModifiersReadonly SemanticTokenModifiers = "readonly"

	SemanticTokenModifiersStatic SemanticTokenModifiers = "static"

	SemanticTokenModifiersDeprecated SemanticTokenModifiers = "deprecated"

	SemanticTokenModifiersAbstract SemanticTokenModifiers = "abstract"

	SemanticTokenModifiersAsync SemanticTokenModifiers = "async"

	SemanticTokenModifiersModification SemanticTokenModifiers = "modification"

	SemanticTokenModifiersDocumentation SemanticTokenModifiers = "documentation"

	SemanticTokenModifiersDefaultLibrary SemanticTokenModifiers = "defaultLibrary"
)
