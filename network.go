package tcpnetwork

import (
	"log"
	"net"
)

const (
	kServerConf_SendBufferSize = 1024
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
	shutdown        bool
	readTimeoutSec  int
}

func NewTCPNetwork(eventQueueSize int, sp IStreamProtocol) *TCPNetwork {
	s := &TCPNetwork{}
	s.eventQueue = make(chan *ConnEvent, eventQueueSize)
	s.streamProtocol = sp
	s.connsForServer = make(map[int]*Connection)
	s.connsForClient = make(map[int]*Connection)
	s.shutdown = false
	//	default config
	s.Conf.SendBufferSize = kServerConf_SendBufferSize
	return s
}

func (this *TCPNetwork) Push(evt *ConnEvent) {
	if nil == this.eventQueue {
		return
	}
	this.eventQueue <- evt
}

func (this *TCPNetwork) Pop() *ConnEvent {
	evt, ok := <-this.eventQueue
	if !ok {
		//	event queue already closed
		this.eventQueue = nil
		return nil
	}

	return evt
}

func (this *TCPNetwork) Listen(addr string) error {
	ls, err := net.Listen("tcp", addr)
	if nil != err {
		return err
	}

	//	accept
	this.listener = ls
	go this.acceptRoutine()
	return nil
}

func (this *TCPNetwork) Connect(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if nil != err {
		return err
	}

	connection := this.createConn(conn)
	connection.from = 1
	connection.run()

	return nil
}

func (this *TCPNetwork) GetStreamProtocol() IStreamProtocol {
	return this.streamProtocol
}

func (this *TCPNetwork) SetStreamProtocol(sp IStreamProtocol) {
	this.streamProtocol = sp
}

func (this *TCPNetwork) GetReadTimeoutSec() int {
	return this.readTimeoutSec
}

func (this *TCPNetwork) SetReadTimeoutSec(sec int) {
	this.readTimeoutSec = sec
}

func (this *TCPNetwork) DisconnectAllConnectionsServer() {
	for k, c := range this.connsForServer {
		c.Close()
		delete(this.connsForServer, k)
	}
}

func (this *TCPNetwork) DisconnectAllConnectionsClient() {
	for k, c := range this.connsForClient {
		c.Close()
		delete(this.connsForClient, k)
	}
}

func (this *TCPNetwork) Shutdown() {
	if this.shutdown {
		return
	}
	this.shutdown = true

	//	stop accept routine
	if nil != this.listener {
		this.listener.Close()
		this.listener = nil
	}

	//	close all connections
	this.DisconnectAllConnectionsClient()
	this.DisconnectAllConnectionsServer()
}

func (this *TCPNetwork) createConn(c net.Conn) *Connection {
	conn := newConnection(c, this.Conf.SendBufferSize, this)
	conn.setStreamProtocol(this.streamProtocol)
	return conn
}

func (this *TCPNetwork) ServeWithHandler(handler IEventHandler) {
	for {
		select {
		case evt, ok := <-this.eventQueue:
			{
				if !ok {
					//	channel closed??
					break
				}

				this.handleEvent(evt, handler)

				//	terminate?
				if this.shutdown {
					//	shutdown?
					if len(this.connsForClient) == 0 &&
						len(this.connsForServer) == 0 {
						//	terminate
						this.shutdown = false
						return
					}
				}
			}
		}
	}
}

func (this *TCPNetwork) acceptRoutine() {
	for {
		conn, err := this.listener.Accept()
		if err != nil {
			log.Println("accept routine quit.error:", err)
			return
		}

		//	process conn event
		connection := this.createConn(conn)
		connection.SetReadTimeoutSec(this.readTimeoutSec)
		connection.from = 0
		connection.run()
	}
}

func (this *TCPNetwork) handleEvent(evt *ConnEvent, handler IEventHandler) {
	switch evt.EventType {
	case KConnEvent_Connected:
		{
			//	add to connection map
			connId := 0
			if evt.Conn.from == 0 {
				connId = this.connIdForServer + 1
				this.connIdForServer = connId
				this.connsForServer[connId] = evt.Conn
			} else {
				connId = this.connIdForClient + 1
				this.connIdForClient = connId
				this.connsForClient[connId] = evt.Conn
			}
			evt.Conn.connId = connId

			handler.OnConnected(evt)
		}
	case KConnEvent_Disconnected:
		{
			handler.OnDisconnected(evt)

			//	remove from connection map
			if evt.Conn.from == 0 {
				delete(this.connsForServer, evt.Conn.connId)
			} else {
				delete(this.connsForClient, evt.Conn.connId)
			}
		}
	case KConnEvent_Data:
		{
			handler.OnRecv(evt)
		}
	}
}
