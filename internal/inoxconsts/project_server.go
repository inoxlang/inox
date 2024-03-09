package inoxconsts

const (
	DEFAULT_PROJECT_SERVER_PORT                             = "8305"
	DEFAULT_PROJECT_SERVER_PORT_INT                         = 8305
	DEFAULT_DENO_CONTROL_SERVER_PORT_FOR_PROJECT_SERVER     = "8306"
	DEFAULT_DENO_CONTROL_SERVER_PORT_INT_FOR_PROJECT_SERVER = 8306

	DEV_PORT_0 string = "8080"
	DEV_PORT_1 string = "8081"

	DEV_SESSION_KEY_HEADER = "X-Dev-Session-Key"

	DEV_CTX_DATA_ENTRY = "/dev"
)

func IsDevPort(s string) bool {
	return s == DEV_PORT_0 || s == DEV_PORT_1
}
