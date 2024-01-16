# Parse Package


## Most Relevant files

- [ast.go](./ast.go)
	- AST Node types
	- `Walk` function
	- Node search functions

- [token.go](./token.go)
	- `Token` struct types
	- Token types and sub-types
	- `GetTokens` function (token list reconstruction)

- [parse_chunk_source.go](./parsed_chunk_source.go)
	- `ParsedChunkSource` type
	- Helper methods to find nodes in the AST and to get positions.

- [parse_chunk.go](./parse_chunk.go) - chunk parsing logic
- [parse_expression.go](./parse_expression.go) - main expression parsing logic
- [parse_statement.go](./parse_statement.go) - main statement parsing logic
- [parse.go](./parse.go) - all other parsing logic (> 9k SLOC)
- [parse_test.go](./parse_test.go)


## Implementation

```go
type parser struct {
	s              []rune //chunk's code
	i              int32  //rune index
	len            int32
	inPattern      bool
	onlyChunkStart bool

	tokens []Token

    // ... other fields
}
```

The parser directly parses Inox code, there is no lexer per se. For example,
when encountering an integer literal the parser directly creates an
`*IntLiteral` node. However valueless tokens such as `\n`, `,` are added as
`Token` values to the `tokens` slice.

```go
type IntLiteral struct {
	NodeBase
	Raw      string
	Value    int64
}
```

The complete 'true' list of tokens is not returned by parsing functions
(`ParseChunk`, `ParseChunk2`, ...).

```go
type Chunk struct {
    //mostly valueless tokens, sorted by position (ascending).
	//EmbeddedModule nodes hold references to subslices of .Tokens.
	Tokens []Token

    // ... other fields
}
```

The true list of tokens can be constructed by calling `GetTokens`. This function
walks over the AST to get the tokens that are not present in `Chunk.Tokens`.

## Options and Timeout

The parser times out after a small duration by default (DEFAULT_TIMEOUT).
