package core

import (
	"encoding/gob"
	"sync/atomic"
)

// Registration of some core types by the gob package in order to support encoding/decoding them.

var (
	permTypesGobRegistered        atomic.Bool
	simpleValueTypesGobRegistered atomic.Bool
)

func RegisterSimpleValueTypesInGob() {
	if !simpleValueTypesGobRegistered.CompareAndSwap(false, true) {
		return
	}

	gob.Register(Str(""))
	gob.Register(Rune('a'))
	gob.Register(Byte(0))
	gob.Register(Port{})

	gob.Register(Int(0))
	gob.Register(Float(0))
	gob.Register(Bool(true))

	gob.Register(Path(""))
	gob.Register(PathPattern(""))

	gob.Register(Scheme(""))
	gob.Register((""))
	gob.Register(Host(""))
	gob.Register(HostPattern(""))
	gob.Register(URL(""))
	gob.Register(URLPattern(""))
}

func RegisterPermissionTypesInGob() {
	if !permTypesGobRegistered.CompareAndSwap(false, true) {
		return
	}

	gob.Register([]Permission{})

	gob.Register(FilesystemPermission{})
	gob.Register(GlobalVarPermission{})
	gob.Register(HttpPermission{})
	gob.Register(DNSPermission{})
	gob.Register(WebsocketPermission{})
	gob.Register(LThreadPermission{})
	gob.Register(DatabasePermission{})
	gob.Register(EnvVarPermission{})
	gob.Register(RawTcpPermission{})
	gob.Register(SystemGraphAccessPermission{})
	gob.Register(ValueVisibilityPermission{})
}
