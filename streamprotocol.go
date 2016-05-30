package tcpnetwork

import (
	"bytes"
	"encoding/binary"
)

const (
	kStreamProtocol4HeaderLength = 4
	kStreamProtocol2HeaderLength = 2
)

func getStreamMaxLength(headerBytes uint32) int {
	return 1<<(8*headerBytes) - 1
}

//	implement default stream protocol
//	stream protocol interface for 4 bytes header
type StreamProtocol4 struct {
	serializeBuf   *bytes.Buffer
	unserializeBuf *bytes.Buffer
}

func NewStreamProtocol4() *StreamProtocol4 {
	return &StreamProtocol4{}
}

func (this *StreamProtocol4) Init() {
	this.serializeBuf = new(bytes.Buffer)
	this.unserializeBuf = new(bytes.Buffer)
}

func (this *StreamProtocol4) GetHeaderLength() int {
	return kStreamProtocol4HeaderLength
}

func (this *StreamProtocol4) UnserializeHeader(buf []byte) int {
	var ln int32 = 0
	this.unserializeBuf.Reset()
	this.unserializeBuf.Write(buf)
	binary.Read(this.unserializeBuf, binary.BigEndian, &ln)
	return int(ln)
}

func (this *StreamProtocol4) SerializeHeader(body []byte) []byte {
	if len(body)+kStreamProtocol4HeaderLength > getStreamMaxLength(kStreamProtocol4HeaderLength) {
		//	stream is too long
		return nil
	}
	var ln int32 = int32(len(body) + kStreamProtocol4HeaderLength)
	this.serializeBuf.Reset()
	binary.Write(this.serializeBuf, binary.BigEndian, &ln)
	return this.serializeBuf.Bytes()
}

//	stream protocol interface for 2 bytes header
type StreamProtocol2 struct {
	serializeBuf   *bytes.Buffer
	unserializeBuf *bytes.Buffer
}

func NewStreamProtocol2() *StreamProtocol2 {
	return &StreamProtocol2{}
}

func (this *StreamProtocol2) Init() {
	this.serializeBuf = new(bytes.Buffer)
	this.unserializeBuf = new(bytes.Buffer)
}

func (this *StreamProtocol2) GetHeaderLength() int {
	return kStreamProtocol2HeaderLength
}

func (this *StreamProtocol2) UnserializeHeader(buf []byte) int {
	var ln int16 = 0
	this.unserializeBuf.Reset()
	this.unserializeBuf.Write(buf)
	binary.Read(this.unserializeBuf, binary.BigEndian, &ln)
	return int(ln)
}

func (this *StreamProtocol2) SerializeHeader(body []byte) []byte {
	if len(body)+kStreamProtocol2HeaderLength > getStreamMaxLength(kStreamProtocol2HeaderLength) {
		//	stream is too long
		return nil
	}

	var ln int16 = int16(len(body) + kStreamProtocol2HeaderLength)
	this.serializeBuf.Reset()
	binary.Write(this.serializeBuf, binary.BigEndian, &ln)
	return this.serializeBuf.Bytes()
}
