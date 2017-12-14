package tcpnetwork

import (
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"runtime/debug"

	"github.com/golang/protobuf/proto"
)

const (
	kConnStatus_None = iota
	kConnStatus_Connected
	kConnStatus_Disconnected
)

// All connection event
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

// Send method flag
const (
	KConnFlag_CopySendBuffer = 1 << iota // Copy the send buffer
	KConnFlag_NoHeader                   // Do not append stream header
)

// send task
type sendTask struct {
	data []byte
	flag int64
}

// Connection is a wrap for net.Conn and process read and write task of the conn
// When event occurs, it will call the eventQueue to dispatch event
type Connection struct {
	conn                net.Conn
	status              int32
	connID              int
	sendMsgQueue        chan *sendTask
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
		connID:              0,
		sendMsgQueue:        make(chan *sendTask, sendBufferSize),
		sendTimeoutSec:      kConnConf_DefaultSendTimeoutSec,
		maxReadBufferLength: kConnConf_MaxReadBufferLength,
		eventQueue:          eq,
	}
}

// ConnEvent represents a event occurs on a connection, such as connected, disconnected or data arrived
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

// Directly close, packages in queue will not be sent
func (c *Connection) close() {
	// Set the disconnected status, use atomic operation to avoid close twice
	if atomic.CompareAndSwapInt32(&c.status, kConnStatus_Connected, kConnStatus_Disconnected) {
		c.conn.Close()
	}
}

// Close the connection, routine safe, send task in the queue will be sent before closing the connection
func (c *Connection) Close() {
	if atomic.LoadInt32(&c.status) != kConnStatus_Connected {
		return
	}

	select {
	case c.sendMsgQueue <- nil:
		{
			// Nothing
		}
	case <-time.After(time.Duration(c.sendTimeoutSec) * time.Second):
		{
			// Timeout, close the connection
			c.close()
		}
	}

	//	disable send
	atomic.StoreInt32(&c.disableSend, 1)
}

// Free the connection. When don't need conection to send any thing, free it, DO NOT call it on multi routines
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
	// This is for sync execute
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

// SetSyncExecuteFunc , you can set a callback that you can synchoronously process the event in every connection's event routine
// If the callback function return true, the event will not be dispatched
func (c *Connection) SetSyncExecuteFunc(fn FuncSyncExecute) FuncSyncExecute {
	prevFn := c.fnSyncExecute
	c.fnSyncExecute = fn
	return prevFn
}

// GetStatus get the connection's status
func (c *Connection) GetStatus() int32 {
	return c.status
}

func (c *Connection) setStatus(stat int) {
	c.status = int32(stat)
}

// GetConnId get the connection's id
func (c *Connection) GetConnId() int {
	return c.connID
}

// SetConnId set the connection's id
func (c *Connection) SetConnId(id int) {
	c.connID = id
}

// GetConn get the raw net.Conn interface
func (c *Connection) GetConn() net.Conn {
	return c.conn
}

// GetUserdata get the userdata you set
func (c *Connection) GetUserdata() interface{} {
	return c.userdata
}

// SetUserdata set the userdata you need
func (c *Connection) SetUserdata(ud interface{}) {
	c.userdata = ud
}

// SetReadTimeoutSec set the read deadline for the connection
func (c *Connection) SetReadTimeoutSec(sec int) {
	c.readTimeoutSec = sec
}

// GetReadTimeoutSec get the read deadline for the connection
func (c *Connection) GetReadTimeoutSec() int {
	return c.readTimeoutSec
}

// GetRemoteAddress return the remote address of the connection
func (c *Connection) GetRemoteAddress() string {
	return c.remoteAddr
}

// GetLocalAddress return the local address of the connection
func (c *Connection) GetLocalAddress() string {
	return c.localAddr
}

// SetUnpacker you can set a custom binary stream unpacker on the connection
func (c *Connection) SetUnpacker(unpacker IUnpacker) {
	c.unpacker = unpacker
}

// GetUnpacker you can get the unpacker you set
func (c *Connection) GetUnpacker() IUnpacker {
	return c.unpacker
}

func (c *Connection) setStreamProtocol(sp IStreamProtocol) {
	c.streamProtocol = sp
}

func (c *Connection) sendRaw(task *sendTask) error {
	if atomic.LoadInt32(&c.disableSend) != 0 {
		return ErrConnIsClosed
	}
	if atomic.LoadInt32(&c.status) != kConnStatus_Connected {
		return ErrConnIsClosed
	}

	select {
	case c.sendMsgQueue <- task:
		{
			// Nothing
		}
	case <-time.After(time.Duration(c.sendTimeoutSec) * time.Second):
		{
			// Timeout, close the connection
			logError("Send to peer %s timeout, close connection", c.GetRemoteAddress())
			c.close()
			return ErrConnSendTimeout
		}
	}

	return nil
}

// ApplyReadDeadline set the read deadline seconds
func (c *Connection) ApplyReadDeadline() {
	if 0 != c.readTimeoutSec {
		c.conn.SetReadDeadline(time.Now().Add(time.Duration(c.readTimeoutSec) * time.Second))
	}
}

// ResetReadDeadline reset the read deadline
func (c *Connection) ResetReadDeadline() {
	c.conn.SetReadDeadline(time.Time{})
}

// Send the buffer with KConnFlag flag
func (c *Connection) Send(msg []byte, f int64) error {
	task := &sendTask{
		data: msg,
		flag: f,
	}
	buf := msg

	// Copy send buffer is KConnFlag_CopySendBuffer flag is set
	if 0 != f&KConnFlag_CopySendBuffer {
		msgCopy := make([]byte, len(msg))
		copy(msgCopy, msg)
		buf = msgCopy
		task.data = buf
	}

	return c.sendRaw(task)
}

// SendPb send protocol buffer message
func (c *Connection) SendPb(pb proto.Message, f int64) error {
	var pbd []byte
	var err error

	if pbd, err = proto.Marshal(pb); nil != err {
		return err
	}

	task := sendTask{
		data: pbd,
		flag: f,
	}

	//	send protobuf
	return c.sendRaw(&task)
}

// Run a routine to process the connection
func (c *Connection) run() {
	go c.routineMain()
}

func (c *Connection) routineMain() {
	defer func() {
		// Routine end
		e := recover()
		if e != nil {
			logFatal("Read routine panic %v, stack:", e)
			stackInfo := debug.Stack()
			logFatal(string(stackInfo))
		}

		// Close the connection
		logWarn("Read routine %s closed", c.GetRemoteAddress())
		c.close()

		// Free channel
		// FIXED : consumers need free it, not producer

		// Post disconnected event
		c.pushEvent(KConnEvent_Disconnected, nil)
	}()

	if nil == c.streamProtocol {
		panic("Nil stream protocol")
		return
	}
	c.streamProtocol.Init()

	// Push connected event
	c.pushEvent(KConnEvent_Connected, nil)
	atomic.StoreInt32(&c.status, kConnStatus_Connected)

	go c.routineSend()
	err := c.routineRead()
	if nil != err {
		logError("Read routine quit with error %v", err)
	}
}

func (c *Connection) routineSend() error {
	var err error

	defer func() {
		if nil != err {
			logError("Send routine quit with error %v", err)
		}
		e := recover()
		if nil != e {
			//	panic
			logFatal("Send routine panic %v, stack:", e)
			stackInfo := debug.Stack()
			logFatal(string(stackInfo))
		}
	}()

	for {
		select {
		case evt, ok := <-c.sendMsgQueue:
			{
				if !ok {
					// Channel closed, quit
					return nil
				}

				if nil == evt {
					c.close()
					return nil
				}

				// Append header data only when KConnFlag_NoHeader is not set
				if 0 == evt.flag&KConnFlag_NoHeader {
					headerBytes := c.streamProtocol.SerializeHeader(evt.data)
					if nil != headerBytes {
						// Write header first
						if len(headerBytes) != 0 {
							_, err = c.conn.Write(headerBytes)
							if err != nil {
								return err
							}
						}
					} else {
						// Invalid packet
						panic("Failed to serialize header")
						break
					}
				}

				_, err = c.conn.Write(evt.data)
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
		if nil == msg {
			// Empty pstream
			continue
		}

		// Only push event when the connection is connected
		if atomic.LoadInt32(&c.status) == kConnStatus_Connected {
			// Try to unserialize to pb. do it in each routine to reduce the pressure of worker routine
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
	var err error
	// Read head
	c.ApplyReadDeadline()
	headerLength := int(c.streamProtocol.GetHeaderLength())
	if headerLength > len(buf) {
		return nil, fmt.Errorf("Header length %d > buffer length %d", headerLength, len(buf))
	}
	headBuf := buf[:headerLength]
	if _, err = io.ReadFull(c.conn, headBuf); nil != err {
		return nil, err
	}

	// Check length
	packetLength := c.streamProtocol.UnserializeHeader(headBuf)
	if packetLength > uint32(c.maxReadBufferLength) ||
		packetLength < c.streamProtocol.GetHeaderLength() {
		return nil, fmt.Errorf("Invalid stream length %d", packetLength)
	}
	if packetLength == c.streamProtocol.GetHeaderLength() {
		// Empty stream ?
		logFatal("Invalid stream length equal to header length")
		return nil, nil
	}

	// Read body
	c.ApplyReadDeadline()
	bodyLength := packetLength - c.streamProtocol.GetHeaderLength()
	if _, err = io.ReadFull(c.conn, buf[:bodyLength]); nil != err {
		return nil, err
	}

	// OK
	msg := make([]byte, bodyLength)
	copy(msg, buf[:bodyLength])
	c.ResetReadDeadline()

	return msg, nil
}
