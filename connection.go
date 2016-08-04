package tcpnetwork

import (
	"errors"
	"log"
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

//	directly close, packages in queue will not be sent
func (this *Connection) close() {
	if kConnStatus_Connected != this.status {
		return
	}

	//	set the disconnected status, use atomic operation
	//this.status = kConnStatus_Disconnected
	atomic.StoreInt32(&this.status, kConnStatus_Disconnected)
	this.conn.Close()
}

func (this *Connection) Close() {
	if this.status != kConnStatus_Connected {
		return
	}

	select {
	case this.sendMsgQueue <- nil:
		{
			//	nothing
		}
	case <-time.After(time.Duration(this.sendTimeoutSec)):
		{
			//	timeout, close the connection
			this.close()
		}
	}

	//	set the disconnected status, use atomic operation
	//this.status = kConnStatus_Disconnected
	atomic.StoreInt32(&this.status, kConnStatus_Disconnected)
}

//	When don't need conection to send any thing, free it, DO NOT call it on multi routines
func (this *Connection) Free() {
	if nil != this.sendMsgQueue {
		close(this.sendMsgQueue)
		this.sendMsgQueue = nil
	}
}

func (this *Connection) syncExecuteEvent(evt *ConnEvent) bool {
	if nil == this.fnSyncExecute {
		return false
	}

	return this.fnSyncExecute(evt)
}

func (this *Connection) pushEvent(et int, d []byte) {
	//	this is for sync execute
	evt := newConnEvent(et, this, d)
	if this.syncExecuteEvent(evt) {
		return
	}

	if nil == this.eventQueue {
		panic("Nil event queue")
		return
	}
	this.eventQueue.Push(evt)
}

func (this *Connection) pushPbEvent(pb proto.Message) {
	if nil == this.eventQueue {
		panic("Nil event queue")
		return
	}
	this.eventQueue.Push(&ConnEvent{
		EventType: KConnEvent_Data,
		Conn:      this,
		PbM:       pb,
	})
}

func (this *Connection) SetSyncExecuteFunc(fn FuncSyncExecute) FuncSyncExecute {
	prevFn := this.fnSyncExecute
	this.fnSyncExecute = fn
	return prevFn
}

func (this *Connection) GetStatus() int32 {
	return this.status
}

func (this *Connection) setStatus(stat int) {
	this.status = int32(stat)
}

func (this *Connection) GetConnId() int {
	return this.connId
}

func (this *Connection) SetConnId(id int) {
	this.connId = id
}

func (this *Connection) GetUserdata() interface{} {
	return this.userdata
}

func (this *Connection) SetUserdata(ud interface{}) {
	this.userdata = ud
}

func (this *Connection) SetReadTimeoutSec(sec int) {
	this.readTimeoutSec = sec
}

func (this *Connection) GetReadTimeoutSec() int {
	return this.readTimeoutSec
}

func (this *Connection) GetRemoteAddress() string {
	return this.conn.RemoteAddr().String()
}

func (this *Connection) GetLocalAddress() string {
	return this.conn.LocalAddr().String()
}

func (this *Connection) SetUnpacker(unpacker IUnpacker) {
	this.unpacker = unpacker
}

func (this *Connection) GetUnpacker() IUnpacker {
	return this.unpacker
}

func (this *Connection) setStreamProtocol(sp IStreamProtocol) {
	this.streamProtocol = sp
}

func (this *Connection) sendRaw(msg []byte) error {
	if this.status != kConnStatus_Connected {
		return ErrConnIsClosed
	}

	select {
	case this.sendMsgQueue <- msg:
		{
			//	nothing
		}
	case <-time.After(time.Duration(this.sendTimeoutSec)):
		{
			//	timeout, close the connection
			this.close()
			return ErrConnSendTimeout
		}
	}

	return nil
}

//	send bytes
func (this *Connection) Send(msg []byte, flag int64) error {
	if this.status != kConnStatus_Connected {
		return ErrConnIsClosed
	}

	buf := msg

	//	copy send buffer
	if 0 != flag&KConnFlag_CopySendBuffer {
		msgCopy := make([]byte, len(msg))
		copy(msgCopy, msg)
		buf = msgCopy
	}

	return this.sendRaw(buf)
}

//	send protocol buffer message
func (this *Connection) SendPb(pb proto.Message) error {
	if this.status != kConnStatus_Connected {
		return ErrConnIsClosed
	}

	var data []byte
	var err error

	if data, err = proto.Marshal(pb); nil != err {
		return err
	}

	//	send protobuf
	return this.sendRaw(data)
}

//	run a routine to process the connection
func (this *Connection) run() {
	go this.routineMain()
}

func (this *Connection) routineMain() {
	defer func() {
		//	routine end
		e := recover()
		if e != nil {
			log.Println("Panic:", e)
		}

		//	close the connection
		this.close()

		//	free channel
		//	FIXED : consumers need free it, not producer
		//close(this.sendMsgQueue)
		//this.sendMsgQueue = nil

		//	post event
		this.pushEvent(KConnEvent_Disconnected, nil)
	}()

	if nil == this.streamProtocol {
		panic("Nil stream protocol")
		return
	}
	this.streamProtocol.Init()

	//	connected
	this.pushEvent(KConnEvent_Connected, nil)
	this.status = kConnStatus_Connected

	go this.routineSend()
	this.routineRead()
}

func (this *Connection) routineSend() error {
	defer func() {
		e := recover()
		if nil != e {
			//	panic, close the connection
			log.Println("Panic:", e)
			this.close()
		}
	}()

	for {
		select {
		case evt, ok := <-this.sendMsgQueue:
			{
				if !ok {
					//	channel closed, quit
					return nil
				}

				if nil == evt {
					this.close()
					return nil
				}

				var err error

				headerBytes := this.streamProtocol.SerializeHeader(evt)
				if nil != headerBytes {
					//	write header first
					_, err = this.conn.Write(headerBytes)
					if err != nil {
						return err
					}
				} else {
					//	invalid packet
					panic("Failed to serialize header")
					break
				}

				_, err = this.conn.Write(evt)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (this *Connection) routineRead() error {
	//	default buffer
	buf := make([]byte, this.maxReadBufferLength)
	var msg []byte
	var err error

	for {
		if nil == this.unpacker {
			msg, err = this.unpack(buf)
		} else {
			msg, err = this.unpacker.Unpack(this.conn, buf)
		}
		if err != nil {
			return err
		}

		//	only push event when the connection is connected
		if this.status == kConnStatus_Connected {
			//	try to unserialize to pb. do it in each routine to reduce the pressure of worker routine
			pb := unserializePb(msg)
			if nil != pb {
				this.pushPbEvent(pb)
			} else {
				this.pushEvent(KConnEvent_Data, msg)
			}
		}
	}

	return nil
}

func (this *Connection) unpack(buf []byte) ([]byte, error) {
	//	read head
	if 0 != this.readTimeoutSec {
		this.conn.SetReadDeadline(time.Now().Add(time.Duration(this.readTimeoutSec) * time.Second))
	}
	headBuf := buf[:this.streamProtocol.GetHeaderLength()]
	_, err := this.conn.Read(headBuf)
	if err != nil {
		return nil, err
	}

	//	check length
	packetLength := this.streamProtocol.UnserializeHeader(headBuf)
	if packetLength > this.maxReadBufferLength ||
		0 == packetLength {
		return nil, errors.New("The stream data is too long")
	}

	//	read body
	if 0 != this.readTimeoutSec {
		this.conn.SetReadDeadline(time.Now().Add(time.Duration(this.readTimeoutSec) * time.Second))
	}
	bodyLength := packetLength - this.streamProtocol.GetHeaderLength()
	_, err = this.conn.Read(buf[:bodyLength])
	if err != nil {
		return nil, err
	}

	//	ok
	msg := make([]byte, bodyLength)
	copy(msg, buf[:bodyLength])
	if 0 != this.readTimeoutSec {
		this.conn.SetReadDeadline(time.Time{})
	}

	return msg, nil
}
