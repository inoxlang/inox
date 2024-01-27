package globalnames

const (
	// namespaces
	FS_NS       = "fs"
	HTTP_NS     = "http"
	TCP_NS      = "tcp"
	DNS_NS      = "dns"
	WS_NS       = "ws"
	S3_NS       = "s3"
	CHROME_NS   = "chrome"
	ENV_NS      = "env"
	HTML_NS     = "html"
	INOX_NS     = "inox"
	INOXSH_NS   = "inoxsh"
	INOXLSP_NS  = "inoxlsp"
	STRMANIP_NS = "strmanip"
	RSA_NS      = "rsa"
	INSECURE_NS = "insecure"
	LOG_NS      = "log"

	LS_FN = "ls"

	// transaction
	GET_CURRENT_TX_FN = "get_current_tx"
	START_TX_FN       = "start_tx"

	ERROR_FN = "Error"

	// resource
	READ_FN   = "read"
	CREATE_FN = "create"
	UPDATE_FN = "update"
	DELETE_FN = "delete"
	SERVE_FN  = "serve"

	// events
	EVENT_FN     = "Event"
	EVENT_SRC_FN = "EventSource"

	// watch
	WATCH_RECEIVED_MESSAGES_FN = "watch_received_messages"
	VALUE_HISTORY_FN           = "ValueHistory"
	DYNIF_FN                   = "dynif"
	DYNCALL_FN                 = "dyncall"
	GET_SYSTEM_GRAPH_FN        = "get_system_graph"

	// send & receive values
	SENDVAL_FN = "sendval"

	// crypto
	SHA256_FN         = "sha256"
	SHA384_FN         = "sha384"
	SHA512_FN         = "sha512"
	HASH_PASSWORD_FN  = "hash_password"
	CHECK_PASSWORD_FN = "check_password"
	RAND_FN           = "rand"

	//encodings
	B64_FN  = "b64"
	DB64_FN = "db64"

	HEX_FN   = "hex"
	UNHEX_FN = "unhex"

	// conversion
	TOSTR_FN     = "tostr"
	TOSTRING_FN  = "tostring"
	TORUNE_FN    = "torune"
	TOBYTE_FN    = "tobyte"
	TOFLOAT_FN   = "tofloat"
	TOINT_FN     = "toint"
	TOBYTECOUNT  = "tobytecount"
	TORSTREAM_FN = "torstream"
	TOJSON_FN    = "tojson"
	TOPJSON_FN   = "topjson"
	PARSE_FN     = "parse"
	SPLIT_FN     = "split"

	// time
	AGO_FN        = "ago"
	NOW_FN        = "now"
	TIME_SINCE_FN = "time_since"
	SLEEP_FN      = "sleep"

	// printing
	PRINT_FN  = "print"
	FPRINT_FN = "fprint"
	FMT_FN    = "fmt"

	// bytes & runes
	MKBYTES_FN    = "mkbytes"
	RUNES_FN      = "Runes"
	BYTES_FN      = "Bytes"
	IS_SPACE_FN   = "is_space"
	READER_FN     = "Reader"
	RINGBUFFER_FN = "RingBuffer"

	//string types
	EMAIL_ADDRESS_FN = "EmailAddress"

	//id types
	ULID_FN   = "ULID"
	UUIDv4_FN = "UUIDv4"

	// functional
	IDENTITY_FN        = "idt"
	MAP_ITERABLE_FN    = "map_iterable"
	FILTER_ITERABLE_FN = "filter_iterable"
	GET_AT_MOST_FN     = "get_at_most"
	SOME_FN            = "some"
	ALL_FN             = "all"
	NONE_FN            = "none"
	REPLACE_FN         = "replace"
	FIND_FN            = "find"

	// concurrency & execution
	LTHREADGROUP_FN = "LThreadGroup"
	RUN_FN          = "rune"
	EXEC_FN         = "ex" //command execution
	CANCEL_EXEC_FN  = "cancel_exec"

	// integer
	IS_EVEN_FN = "is_even"
	IS_ODD_FN  = "is_odd"

	// number
	MINOF_FN  = "minof"
	MAXOF_FN  = "maxof"
	MINMAX_FN = "minmax"

	// protocol
	SET_CLIENT_FOR_URL_FN  = "set_client_for_url"
	SET_CLIENT_FOR_HOST_FN = "set_client_for_host"

	// other functions
	ADD_CTX_DATA_FN = "add_ctx_data"
	CTX_DATA_FN     = "ctx_data"
	PROPNAMES_FN    = "propnames"
	GET_FN          = "get"

	ARRAY_FN = "Array"
	LIST_FN  = "List"

	TYPEOF_FN     = "typeof"
	URL_OF_FN     = "url_of"
	LEN_FN        = "len"
	LEN_RANGE_FN  = "len_range"
	IS_MUTABLE_FN = "is_mutable"

	SUM_OPTIONS_FN = "sum_options"
	MIME_FN        = "mime"

	COLOR_FN    = "Color"
	FILEMODE_FN = "FileMode"

	HELP_FN = "help"

	//meta
	MODULE_DIRPATH  = "__mod-dir"
	MODULE_FILEPATH = "__mod-file"

	//other
	PREINIT_DATA       = "preinit-data"
	APP_LISTENING_ADDR = "APP_LISTENING_ADDR"
)
