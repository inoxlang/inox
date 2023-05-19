package permkind

type InternalPermissionTypename string

const (
	ROUTINE_PERM_TYPENAME          InternalPermissionTypename = "routine"
	HTTP_PERM_TYPENAME             InternalPermissionTypename = "http"
	WEBSOCKET_PERM_TYPENAME        InternalPermissionTypename = "websocket"
	DNS_PERM_TYPENAME              InternalPermissionTypename = "dns"
	TCP_PERM_TYPENAME              InternalPermissionTypename = "tcp"
	FS_PERM_TYPENAME               InternalPermissionTypename = "filesystem"
	GLOBAL_VAR_PERM_TYPENAME       InternalPermissionTypename = "global-var"
	CMD_PERM_TYPENAME              InternalPermissionTypename = "command"
	ENV_PERM_TYPENAME              InternalPermissionTypename = "env"
	SYSGRAPH_PERM_TYPENAME         InternalPermissionTypename = "system-graph"
	VALUE_VISIBILITY_PERM_TYPENAME InternalPermissionTypename = "value-visibility"
)
