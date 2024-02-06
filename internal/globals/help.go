package internal

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/help"
)

func registerHelp() {
	help.RegisterHelpValues(map[string]any{
		//transaction

		//resource manipulation
		globalnames.CREATE_FN: _createResource,
		globalnames.READ_FN:   _readResource,
		globalnames.UPDATE_FN: _updateResource,
		globalnames.DELETE_FN: _deleteResource,
		globalnames.GET_FN:    _get,

		//events

		//globalnames.EVENT_FN: _Event,
		//globalnames.EVENT_SRC_FN: core.NewEventSource,

		//watch

		//crypto: registered in crypto.go

		//encoding
		globalnames.B64_FN:  encodeBase64,
		globalnames.DB64_FN: decodeBase64,

		globalnames.HEX_FN:   encodeHex,
		globalnames.UNHEX_FN: decodeHex,

		// conversion
		globalnames.TOSTR_FN:     _tostr,
		globalnames.TOSTRING_FN:  _tostring,
		globalnames.TORUNE_FN:    _torune,
		globalnames.TOBYTE_FN:    _tobyte,
		globalnames.TOFLOAT_FN:   _tofloat,
		globalnames.TOINT_FN:     _toint,
		globalnames.TOBYTECOUNT:  _tobytecount,
		globalnames.TORSTREAM_FN: _torstream,
		globalnames.TOJSON_FN:    core.ToJSON,
		globalnames.TOPJSON_FN:   core.ToPrettyJSON,
		globalnames.PARSE_FN:     _parse,
		globalnames.SPLIT_FN:     _split,

		//time
		globalnames.AGO_FN:   _ago,
		globalnames.NOW_FN:   _now,
		globalnames.SLEEP_FN: core.Sleep,

		//printing
		globalnames.PRINT_FN:  _print,
		globalnames.FPRINT_FN: _fprint,

		//bytes & runes
		globalnames.MKBYTES_FN:    _mkbytes,
		globalnames.RUNES_FN:      _Runes,
		globalnames.BYTES_FN:      _Bytes,
		globalnames.IS_SPACE_FN:   _is_space,
		globalnames.READER_FN:     _Reader,
		globalnames.RINGBUFFER_FN: core.NewRingBuffer,

		//string types
		globalnames.EMAIL_ADDRESS_FN: _EmailAddress,

		//id types
		globalnames.ULID_FN:   _ULID,
		globalnames.UUIDv4_FN: _UUIDV4,

		//functional
		globalnames.MAP_ITERABLE_FN:    core.MapIterable,
		globalnames.FILTER_ITERABLE_FN: core.Filter,
		globalnames.GET_AT_MOST_FN:     core.GetAtMost,
		globalnames.SOME_FN:            core.Some,
		globalnames.ALL_FN:             core.All,
		globalnames.NONE_FN:            core.None,
		globalnames.RAND_FN:            _rand,
		globalnames.FIND_FN:            _find,

		//others
		globalnames.ERROR_FN:        _Error,
		globalnames.ADD_CTX_DATA_FN: _add_ctx_data,
		globalnames.CTX_DATA_FN:     _ctx_data,
	})
}
