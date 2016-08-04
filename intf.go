package tcpnetwork

import (
	"net"
)

type IEventQueue interface {
	Push(*ConnEvent)
	Pop() *ConnEvent
}

type IStreamProtocol interface {
	//	Init
	Init()
	//	get the header length of the stream
	GetHeaderLength() int
	//	read the header length of the stream
	UnserializeHeader([]byte) int
	//	format header
	SerializeHeader([]byte) []byte
}

type IEventHandler interface {
	OnConnected(evt *ConnEvent)
	OnDisconnected(evt *ConnEvent)
	OnRecv(evt *ConnEvent)
}

type IUnpacker interface {
	Unpack(net.Conn, []byte) ([]byte, error)
}
