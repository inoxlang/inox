package inoxd

import "log"

const DAEMON_SUBCMD = "daemon"

type DaemonConfig struct {
	InoxCloud bool                   `json:"inoxCloud"`
	Server    IndividualServerConfig `json:"serverConfig"`
}

type IndividualServerConfig struct {
	MaxWebSocketPerIp      int  `json:"maxWebsocketPerIp"`
	IgnoreInstalledBrowser bool `json:"ignoreInstalledBrowser,omitempty"`
}

func Inoxd(config DaemonConfig) {
	log.Printf("START INOXD: %#v", config)
}
