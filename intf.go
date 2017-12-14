package tcpnetwork

// IEventQueue queues all connection's events
type IEventQueue interface {
	Push(*ConnEvent)
	Pop() *ConnEvent
}

// IStreamProtocol implement the protocol of the binary data stream for unpacking packet
type IStreamProtocol interface {
	// Init
	Init()
	// Get the header length of the stream
	GetHeaderLength() uint32
	// Read the header length of the stream
	UnserializeHeader([]byte) uint32
	// Format header
	SerializeHeader([]byte) []byte
}

// IEventHandler is callback interface to process connection's event
type IEventHandler interface {
	OnConnected(evt *ConnEvent)
	OnDisconnected(evt *ConnEvent)
	OnRecv(evt *ConnEvent)
}

// IUnpacker unpack the binary stream to replace the internal unpack process
type IUnpacker interface {
	Unpack(*Connection, []byte) ([]byte, error)
}
