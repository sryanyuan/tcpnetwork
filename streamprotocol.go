package tcpnetwork

import "encoding/binary"

const (
	kStreamProtocol4HeaderLength = 4
	kStreamProtocol2HeaderLength = 2
)

func getStreamMaxLength(headerBytes uint32) uint64 {
	return 1<<(8*headerBytes) - 1
}

// StreamProtocol4
// Binary format : | 4 byte (total stream length) | data ... (total stream length - 4) |
//	implement default stream protocol
//	stream protocol interface for 4 bytes header
type StreamProtocol4 struct {
}

func NewStreamProtocol4() *StreamProtocol4 {
	return &StreamProtocol4{}
}

func (s *StreamProtocol4) Init() {

}

func (s *StreamProtocol4) GetHeaderLength() uint32 {
	return kStreamProtocol4HeaderLength
}

func (s *StreamProtocol4) UnserializeHeader(buf []byte) uint32 {
	if len(buf) < kStreamProtocol4HeaderLength {
		return 0
	}
	return binary.BigEndian.Uint32(buf)
}

func (s *StreamProtocol4) SerializeHeader(body []byte) []byte {
	if uint64(len(body)+kStreamProtocol4HeaderLength) > uint64(getStreamMaxLength(kStreamProtocol4HeaderLength)) {
		//	stream is too long
		return nil
	}

	var ln uint32 = uint32(len(body) + kStreamProtocol4HeaderLength)
	var buffer [kStreamProtocol4HeaderLength]byte
	binary.BigEndian.PutUint32(buffer[0:], ln)
	return buffer[0:]
}

// StreamProtocol2
// Binary format : | 2 byte (total stream length) | data ... (total stream length - 2) |
//	stream protocol interface for 2 bytes header
type StreamProtocol2 struct {
}

func NewStreamProtocol2() *StreamProtocol2 {
	return &StreamProtocol2{}
}

func (s *StreamProtocol2) Init() {

}

func (s *StreamProtocol2) GetHeaderLength() uint32 {
	return kStreamProtocol2HeaderLength
}

func (s *StreamProtocol2) UnserializeHeader(buf []byte) uint32 {
	if len(buf) < kStreamProtocol2HeaderLength {
		return 0
	}
	return uint32(binary.BigEndian.Uint16(buf))
}

func (s *StreamProtocol2) SerializeHeader(body []byte) []byte {
	if uint64(len(body)+kStreamProtocol2HeaderLength) > uint64(getStreamMaxLength(kStreamProtocol2HeaderLength)) {
		//	stream is too long
		return nil
	}

	var ln uint16 = uint16(len(body) + kStreamProtocol2HeaderLength)
	var buffer [kStreamProtocol2HeaderLength]byte
	binary.BigEndian.PutUint16(buffer[0:], ln)
	return buffer[0:]
}
