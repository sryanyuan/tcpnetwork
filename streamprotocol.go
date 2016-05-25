package tcpnetwork

import (
	"bytes"
	"encoding/binary"
)

const (
	kStreamProtocol4HeaderLength = 4
)

//	implement default stream protocol
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
	var ln int32 = int32(len(body) + kStreamProtocol4HeaderLength)
	this.serializeBuf.Reset()
	binary.Write(this.serializeBuf, binary.BigEndian, &ln)
	return this.serializeBuf.Bytes()
}
