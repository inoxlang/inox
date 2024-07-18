package globals

import "github.com/inoxlang/inox/internal/core"

var (
	DEFAULT_SCRIPT_LIMITS = []core.Limit{
		// {Name: fs_ns.FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 100_000_000},
		// {Name: fs_ns.FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 100_000_000},

		// {Name: fs_ns.FS_NEW_FILE_RATE_LIMIT_NAME, Kind: core.FrequencyLimit, Value: 100 * core.FREQ_LIMIT_SCALE},
		// {Name: fs_ns.FS_TOTAL_NEW_FILE_LIMIT_NAME, Kind: core.TotalLimit, Value: 10_000},

		// {Name: http_ns.HTTP_REQUEST_RATE_LIMIT_NAME, Kind: core.FrequencyLimit, Value: 100 * core.FREQ_LIMIT_SCALE},
		// {Name: ws_ns.WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 10},
		// {Name: net_ns.TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 10},

		// {Name: s3_ns.OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME, Kind: core.FrequencyLimit, Value: 50 * core.FREQ_LIMIT_SCALE},

		{Name: core.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, Kind: core.TotalLimit, Value: 5},
	}
)
