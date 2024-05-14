package hscode

import "github.com/inoxlang/inox/internal/sourcecode"

type ParsedFile struct {
	Result *ParsingResult //nil if error
	Error  *ParsingError
	sourcecode.ParsedChunkSourceBase
}
