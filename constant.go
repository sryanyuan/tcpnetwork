package tcpnetwork

import (
	"errors"
)

//	Sync event callback
//	If return true, this event will not be sent to event channel
//	If return false, this event will be sent to event channel again
type FuncSyncExecute func(*ConnEvent) bool

var (
	ErrConnIsClosed    = errors.New("Connection is closed")
	ErrConnSendTimeout = errors.New("Connection send timeout")
)
