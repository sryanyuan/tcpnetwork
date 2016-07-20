package tcpnetwork

import (
	"errors"
)

var (
	ErrConnIsClosed    = errors.New("Connection is closed")
	ErrConnSendTimeout = errors.New("Connection send timeout")
)
