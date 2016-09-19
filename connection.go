package tcpnetwork

import (
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
)

const (
	kConnStatus_None = iota
	kConnStatus_Connected
	kConnStatus_Disconnected
)

const (
	KConnEvent_None = iota
	KConnEvent_Connected
	KConnEvent_Disconnected
	KConnEvent_Data
	KConnEvent_Pb
	KConnEvent_Close
	KConnEvent_Total
)

const (
	kConnConf_DefaultSendTimeoutSec = 5
	kConnConf_MaxReadBufferLength   = 0xffff // 0xffff
)

const (
	KConnFlag_CopySendBuffer = 1 << iota
)

type Connection struct {
	conn                net.Conn
	status              int32
	connId              int
	sendMsgQueue        chan []byte
	sendTimeoutSec      int
	eventQueue          IEventQueue
	streamProtocol      IStreamProtocol
	maxReadBufferLength int
	userdata            interface{}
	from                int
	readTimeoutSec      int
	fnSyncExecute       FuncSyncExecute
	unpacker            IUnpacker
	disableSend         int32
	localAddr           string
	remoteAddr          string
}

func newConnection(c net.Conn, sendBufferSize int, eq IEventQueue) *Connection {
	return &Connection{
		conn:                c,
		status:              kConnStatus_None,
		connId:              0,
		sendMsgQueue:        make(chan []byte, sendBufferSize),
		sendTimeoutSec:      kConnConf_DefaultSendTimeoutSec,
		maxReadBufferLength: kConnConf_MaxReadBufferLength,
		eventQueue:          eq,
	}
}

type ConnEvent struct {
	EventType int
	Conn      *Connection
	Data      []byte
	Userdata  interface{}
	PbM       proto.Message
}

func newConnEvent(et int, c *Connection, d []byte) *ConnEvent {
	return &ConnEvent{
		EventType: et,
		Conn:      c,
		Data:      d,
	}
}

func (c *Connection) init() {
	c.localAddr = c.conn.LocalAddr().String()
	c.remoteAddr = c.conn.RemoteAddr().String()
}

//	directly close, packages in queue will not be sent
func (c *Connection) close() {
	//	set the disconnected status, use atomic operation to avoid close twice
	if atomic.CompareAndSwapInt32(&c.status, kConnStatus_Connected, kConnStatus_Disconnected) {
		c.conn.Close()
	}
}

func (c *Connection) Close() {
	if c.status != kConnStatus_Connected {
		return
	}

	select {
	case c.sendMsgQueue <- nil:
		{
			//	nothing
		}
	case <-time.After(time.Duration(c.sendTimeoutSec)):
		{
			//	timeout, close the connection
			c.close()
		}
	}

	//	disable send
	atomic.StoreInt32(&c.disableSend, 1)
}

//	When don't need conection to send any thing, free it, DO NOT call it on multi routines
func (c *Connection) Free() {
	if nil != c.sendMsgQueue {
		close(c.sendMsgQueue)
		c.sendMsgQueue = nil
	}
}

func (c *Connection) syncExecuteEvent(evt *ConnEvent) bool {
	if nil == c.fnSyncExecute {
		return false
	}

	return c.fnSyncExecute(evt)
}

func (c *Connection) pushEvent(et int, d []byte) {
	//	this is for sync execute
	evt := newConnEvent(et, c, d)
	if c.syncExecuteEvent(evt) {
		return
	}

	if nil == c.eventQueue {
		panic("Nil event queue")
		return
	}
	c.eventQueue.Push(evt)
}

func (c *Connection) pushPbEvent(pb proto.Message) {
	if nil == c.eventQueue {
		panic("Nil event queue")
		return
	}
	c.eventQueue.Push(&ConnEvent{
		EventType: KConnEvent_Data,
		Conn:      c,
		PbM:       pb,
	})
}

func (c *Connection) SetSyncExecuteFunc(fn FuncSyncExecute) FuncSyncExecute {
	prevFn := c.fnSyncExecute
	c.fnSyncExecute = fn
	return prevFn
}

func (c *Connection) GetStatus() int32 {
	return c.status
}

func (c *Connection) setStatus(stat int) {
	c.status = int32(stat)
}

func (c *Connection) GetConnId() int {
	return c.connId
}

func (c *Connection) SetConnId(id int) {
	c.connId = id
}

func (c *Connection) GetConn() net.Conn {
	return c.conn
}

func (c *Connection) GetUserdata() interface{} {
	return c.userdata
}

func (c *Connection) SetUserdata(ud interface{}) {
	c.userdata = ud
}

func (c *Connection) SetReadTimeoutSec(sec int) {
	c.readTimeoutSec = sec
}

func (c *Connection) GetReadTimeoutSec() int {
	return c.readTimeoutSec
}

func (c *Connection) GetRemoteAddress() string {
	return c.remoteAddr
}

func (c *Connection) GetLocalAddress() string {
	return c.localAddr
}

func (c *Connection) SetUnpacker(unpacker IUnpacker) {
	c.unpacker = unpacker
}

func (c *Connection) GetUnpacker() IUnpacker {
	return c.unpacker
}

func (c *Connection) setStreamProtocol(sp IStreamProtocol) {
	c.streamProtocol = sp
}

func (c *Connection) sendRaw(msg []byte) error {
	if atomic.LoadInt32(&c.disableSend) != 0 {
		return ErrConnIsClosed
	}
	if atomic.LoadInt32(&c.status) != kConnStatus_Connected {
		return ErrConnIsClosed
	}

	select {
	case c.sendMsgQueue <- msg:
		{
			//	nothing
		}
	case <-time.After(time.Duration(c.sendTimeoutSec)):
		{
			//	timeout, close the connection
			c.close()
			return ErrConnSendTimeout
		}
	}

	return nil
}

func (c *Connection) ApplyReadDeadline() {
	if 0 != c.readTimeoutSec {
		c.conn.SetReadDeadline(time.Now().Add(time.Duration(c.readTimeoutSec) * time.Second))
	}
}

func (c *Connection) ResetReadDeadline() {
	c.conn.SetReadDeadline(time.Time{})
}

//	send bytes
func (c *Connection) Send(msg []byte, flag int64) error {
	buf := msg

	//	copy send buffer
	if 0 != flag&KConnFlag_CopySendBuffer {
		msgCopy := make([]byte, len(msg))
		copy(msgCopy, msg)
		buf = msgCopy
	}

	return c.sendRaw(buf)
}

//	send protocol buffer message
func (c *Connection) SendPb(pb proto.Message) error {
	var data []byte
	var err error

	if data, err = proto.Marshal(pb); nil != err {
		return err
	}

	//	send protobuf
	return c.sendRaw(data)
}

//	run a routine to process the connection
func (c *Connection) run() {
	go c.routineMain()
}

func (c *Connection) routineMain() {
	defer func() {
		//	routine end
		e := recover()
		if e != nil {
			logError("Panic", e)
		}

		//	close the connection
		c.close()

		//	free channel
		//	FIXED : consumers need free it, not producer

		//	post event
		c.pushEvent(KConnEvent_Disconnected, nil)
	}()

	if nil == c.streamProtocol {
		panic("Nil stream protocol")
		return
	}
	c.streamProtocol.Init()

	//	connected
	c.pushEvent(KConnEvent_Connected, nil)
	atomic.StoreInt32(&c.status, kConnStatus_Connected)

	go c.routineSend()
	c.routineRead()
}

func (c *Connection) routineSend() error {
	defer func() {
		e := recover()
		if nil != e {
			//	panic
			logError("Panic", e)
		}
	}()

	for {
		select {
		case evt, ok := <-c.sendMsgQueue:
			{
				if !ok {
					//	channel closed, quit
					return nil
				}

				if nil == evt {
					c.close()
					return nil
				}

				var err error

				headerBytes := c.streamProtocol.SerializeHeader(evt)
				if nil != headerBytes {
					//	write header first
					_, err = c.conn.Write(headerBytes)
					if err != nil {
						return err
					}
				} else {
					//	invalid packet
					panic("Failed to serialize header")
					break
				}

				_, err = c.conn.Write(evt)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (c *Connection) routineRead() error {
	//	default buffer
	buf := make([]byte, c.maxReadBufferLength)
	var msg []byte
	var err error

	for {
		if nil == c.unpacker {
			msg, err = c.unpack(buf)
		} else {
			msg, err = c.unpacker.Unpack(c, buf)
		}
		if err != nil {
			return err
		}

		//	only push event when the connection is connected
		if c.status == kConnStatus_Connected {
			//	try to unserialize to pb. do it in each routine to reduce the pressure of worker routine
			pb := unserializePb(msg)
			if nil != pb {
				c.pushPbEvent(pb)
			} else {
				c.pushEvent(KConnEvent_Data, msg)
			}
		}
	}

	return nil
}

func (c *Connection) unpack(buf []byte) ([]byte, error) {
	//	read head
	c.ApplyReadDeadline()
	headBuf := buf[:c.streamProtocol.GetHeaderLength()]
	_, err := c.conn.Read(headBuf)
	if err != nil {
		return nil, err
	}

	//	check length
	packetLength := c.streamProtocol.UnserializeHeader(headBuf)
	if packetLength > c.maxReadBufferLength ||
		0 == packetLength {
		return nil, errors.New("The stream data is too long")
	}

	//	read body
	c.ApplyReadDeadline()
	bodyLength := packetLength - c.streamProtocol.GetHeaderLength()
	_, err = c.conn.Read(buf[:bodyLength])
	if err != nil {
		return nil, err
	}

	//	ok
	msg := make([]byte, bodyLength)
	copy(msg, buf[:bodyLength])
	c.ResetReadDeadline()

	return msg, nil
}
