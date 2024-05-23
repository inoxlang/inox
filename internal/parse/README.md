# Parse Package

This package implements the parsing of Inox code files and provides functions/methods to find nodes and print ASTs.

## Implementation

```go
type parser struct {
	s              []rune //chunk's code
	i              int32  //rune index
	len            int32
	inQuotedRegion   bool
	inUnquotedRegion bool //unquoted inside quoted
	onlyChunkStart   bool

	tokens []ast.Token

	// ... other fields
}
```

The parser directly parses Inox code, there is no lexer per se. For example,
when encountering an integer literal the parser directly creates an
`*ast.IntLiteral` node. However valueless tokens such as `\n`, `,` are added as
`ast.Token` values to the `tokens` slice.

```go
type ast.IntLiteral struct {
	ast.NodeBase
	Raw      string
	Value    int64
}
```

The complete 'true' list of tokens is not returned by parsing functions
(`ParseChunk`, `ParseChunk2`, ...).

```go
type Chunk struct {
	//Mostly valueless tokens, sorted by position (ascending).
	//ast.EmbeddedModule nodes hold references to subslices of .Tokens.
	Tokens []ast.Token

	// ... other fields
}
```

The true list of tokens can be constructed by calling `GetTokens`. This function
walks over the AST to get the tokens that are not present in `Chunk.Tokens`.

## Options and Timeout

The parser times out after a small duration by default (DEFAULT_TIMEOUT).
