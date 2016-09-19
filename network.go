package tcpnetwork

import (
	"net"
	"sync/atomic"
	"time"
)

const (
	kServerConf_SendBufferSize = 1024
	kServerConn                = 0
	kClientConn                = 1
)

type TCPNetworkConf struct {
	SendBufferSize int
}

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

func (t *TCPNetwork) Pop() *ConnEvent {
	evt, ok := <-t.eventQueue
	if !ok {
		//	event queue already closed
		return nil
	}

	return evt
}

func (t *TCPNetwork) GetEventQueue() <-chan *ConnEvent {
	return t.eventQueue
}

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

func (t *TCPNetwork) GetStreamProtocol() IStreamProtocol {
	return t.streamProtocol
}

func (t *TCPNetwork) SetStreamProtocol(sp IStreamProtocol) {
	t.streamProtocol = sp
}

func (t *TCPNetwork) GetReadTimeoutSec() int {
	return t.readTimeoutSec
}

func (t *TCPNetwork) SetReadTimeoutSec(sec int) {
	t.readTimeoutSec = sec
}

func (t *TCPNetwork) DisconnectAllConnectionsServer() {
	for k, c := range t.connsForServer {
		c.Close()
		delete(t.connsForServer, k)
	}
}

func (t *TCPNetwork) DisconnectAllConnectionsClient() {
	for k, c := range t.connsForClient {
		c.Close()
		delete(t.connsForClient, k)
	}
}

func (t *TCPNetwork) Shutdown() {
	if !atomic.CompareAndSwapInt32(&t.shutdownFlag, 0, 1) {
		return
	}

	//	stop accept routine
	if nil != t.listener {
		t.listener.Close()
	}

	//	close all connections
	t.DisconnectAllConnectionsClient()
	t.DisconnectAllConnectionsServer()
}

func (t *TCPNetwork) createConn(c net.Conn) *Connection {
	conn := newConnection(c, t.Conf.SendBufferSize, t)
	conn.setStreamProtocol(t.streamProtocol)
	return conn
}

func (t *TCPNetwork) ServeWithHandler(handler IEventHandler) {
SERVE_LOOP:
	for {
		select {
		case evt, ok := <-t.eventQueue:
			{
				if !ok {
					//	channel closed or shutdown
					break SERVE_LOOP
				}

				t.handleEvent(evt, handler)
			}
		}
	}
}

func (t *TCPNetwork) acceptRoutine() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			logError("accept routine quit.error:", err)
			t.listener = nil
			return
		}

		//	process conn event
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
			//	add to connection map
			connId := 0
			if kServerConn == evt.Conn.from {
				connId = t.connIdForServer + 1
				t.connIdForServer = connId
				t.connsForServer[connId] = evt.Conn
			} else {
				connId = t.connIdForClient + 1
				t.connIdForClient = connId
				t.connsForClient[connId] = evt.Conn
			}
			evt.Conn.connId = connId

			handler.OnConnected(evt)
		}
	case KConnEvent_Disconnected:
		{
			handler.OnDisconnected(evt)

			//	remove from connection map
			if kServerConn == evt.Conn.from {
				delete(t.connsForServer, evt.Conn.connId)
			} else {
				delete(t.connsForClient, evt.Conn.connId)
			}
		}
	case KConnEvent_Data:
		{
			handler.OnRecv(evt)
		}
	}
}
