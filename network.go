package tcpnetwork

import (
	"net"
	"sync/atomic"
	"time"
)

// Constants
const (
	kServerConf_SendBufferSize = 1024
	kServerConn                = 0
	kClientConn                = 1
)

// TCPNetworkConf config the TCPNetwork
type TCPNetworkConf struct {
	SendBufferSize int
}

// TCPNetwork manages all server and client connections
type TCPNetwork struct {
	streamProtocol  IStreamProtocol
	eventQueue      chan *ConnEvent
	listener        net.Listener
	Conf            TCPNetworkConf
	connIdForServer int
	connIdForClient int
	connsForServer  map[int]*Connection
	connsForClient  map[int]*Connection
	shutdownFlag    int32
	readTimeoutSec  int
}

// NewTCPNetwork creates a TCPNetwork object
func NewTCPNetwork(eventQueueSize int, sp IStreamProtocol) *TCPNetwork {
	s := &TCPNetwork{}
	s.eventQueue = make(chan *ConnEvent, eventQueueSize)
	s.streamProtocol = sp
	s.connsForServer = make(map[int]*Connection)
	s.connsForClient = make(map[int]*Connection)
	s.shutdownFlag = 0
	//	default config
	s.Conf.SendBufferSize = kServerConf_SendBufferSize
	return s
}

// Push implements the IEventQueue interface
func (t *TCPNetwork) Push(evt *ConnEvent) {
	if nil == t.eventQueue {
		return
	}

	//	push timeout
	select {
	case t.eventQueue <- evt:
		{

		}
	case <-time.After(time.Second * 5):
		{
			evt.Conn.close()
		}
	}

}

// Pop the event in event queue
func (t *TCPNetwork) Pop() *ConnEvent {
	evt, ok := <-t.eventQueue
	if !ok {
		//	event queue already closed
		return nil
	}

	return evt
}

// GetEventQueue get the event queue channel
func (t *TCPNetwork) GetEventQueue() <-chan *ConnEvent {
	return t.eventQueue
}

// Listen an address to accept client connection
func (t *TCPNetwork) Listen(addr string) error {
	ls, err := net.Listen("tcp", addr)
	if nil != err {
		return err
	}

	//	accept
	t.listener = ls
	go t.acceptRoutine()
	return nil
}

// Connect the remote server
func (t *TCPNetwork) Connect(addr string) (*Connection, error) {
	conn, err := net.Dial("tcp", addr)
	if nil != err {
		return nil, err
	}

	connection := t.createConn(conn)
	connection.from = 1
	connection.run()
	connection.init()

	return connection, nil
}

// GetStreamProtocol returns the stream protocol of current TCPNetwork
func (t *TCPNetwork) GetStreamProtocol() IStreamProtocol {
	return t.streamProtocol
}

// SetStreamProtocol sets the stream protocol of current TCPNetwork
func (t *TCPNetwork) SetStreamProtocol(sp IStreamProtocol) {
	t.streamProtocol = sp
}

// GetReadTimeoutSec returns the read timeout seconds of current TCPNetwork
func (t *TCPNetwork) GetReadTimeoutSec() int {
	return t.readTimeoutSec
}

// SetReadTimeoutSec sets the read timeout seconds of current TCPNetwork
func (t *TCPNetwork) SetReadTimeoutSec(sec int) {
	t.readTimeoutSec = sec
}

// DisconnectAllConnectionsServer disconnect all connections on server side
func (t *TCPNetwork) DisconnectAllConnectionsServer() {
	for k, c := range t.connsForServer {
		c.Close()
		delete(t.connsForServer, k)
	}
}

// DisconnectAllConnectionsClient disconnect all connections on client side
func (t *TCPNetwork) DisconnectAllConnectionsClient() {
	for k, c := range t.connsForClient {
		c.Close()
		delete(t.connsForClient, k)
	}
}

// Shutdown frees all connection and stop the listener
func (t *TCPNetwork) Shutdown() {
	if !atomic.CompareAndSwapInt32(&t.shutdownFlag, 0, 1) {
		return
	}

	// Stop accept routine
	if nil != t.listener {
		t.listener.Close()
	}

	// Close all connections
	t.DisconnectAllConnectionsClient()
	t.DisconnectAllConnectionsServer()
}

func (t *TCPNetwork) createConn(c net.Conn) *Connection {
	conn := newConnection(c, t.Conf.SendBufferSize, t)
	conn.setStreamProtocol(t.streamProtocol)
	return conn
}

// ServeWithHandler process all events in the event queue and dispatch to the IEventHandler
func (t *TCPNetwork) ServeWithHandler(handler IEventHandler) {
SERVE_LOOP:
	for {
		select {
		case evt, ok := <-t.eventQueue:
			{
				if !ok {
					// Channel closed or shutdown
					break SERVE_LOOP
				}

				t.handleEvent(evt, handler)
			}
		}
	}
}

func (t *TCPNetwork) acceptRoutine() {
	// After accept temporary failure, enter sleep and try again
	var tempDelay time.Duration

	for {
		conn, err := t.listener.Accept()
		if err != nil {
			// Check if the error is an temporary error
			if acceptErr, ok := err.(net.Error); ok && acceptErr.Temporary() {
				if 0 == tempDelay {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}

				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}

				logWarn("Accept error %s , retry after %d ms", acceptErr.Error(), tempDelay)
				time.Sleep(tempDelay)
				continue
			}

			logError("accept routine quit.error:%s", err.Error())
			t.listener = nil
			return
		}

		// Process conn event
		connection := t.createConn(conn)
		connection.SetReadTimeoutSec(t.readTimeoutSec)
		connection.from = kServerConn
		connection.init()
		connection.run()
	}
}

func (t *TCPNetwork) handleEvent(evt *ConnEvent, handler IEventHandler) {
	switch evt.EventType {
	case KConnEvent_Connected:
		{
			// Add to connection map
			connID := 0
			if kServerConn == evt.Conn.from {
				connID = t.connIdForServer + 1
				t.connIdForServer = connID
				t.connsForServer[connID] = evt.Conn
			} else {
				connID = t.connIdForClient + 1
				t.connIdForClient = connID
				t.connsForClient[connID] = evt.Conn
			}
			evt.Conn.connID = connID

			handler.OnConnected(evt)
		}
	case KConnEvent_Disconnected:
		{
			handler.OnDisconnected(evt)

			// Remove from connection map
			if kServerConn == evt.Conn.from {
				delete(t.connsForServer, evt.Conn.connID)
			} else {
				delete(t.connsForClient, evt.Conn.connID)
			}
		}
	case KConnEvent_Data:
		{
			handler.OnRecv(evt)
		}
	}
}
