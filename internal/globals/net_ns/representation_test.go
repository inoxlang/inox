package net_ns

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebsocketRepresentation(t *testing.T) {
	assert.False(t, (&WebsocketConnection{}).HasRepresentation(nil, nil))
}

func TestTcpConnRepresentation(t *testing.T) {
	assert.False(t, (&TcpConn{}).HasRepresentation(nil, nil))
}
