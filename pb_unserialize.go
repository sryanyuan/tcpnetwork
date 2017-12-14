package tcpnetwork

import (
	"github.com/golang/protobuf/proto"
)

// FuncPbUnserializeHook is a function to unserialize binary data to protobuf message
type FuncPbUnserializeHook func([]byte) proto.Message

var (
	fnPbUnserializeHook FuncPbUnserializeHook
)

// SetPbUnserializeHook set the global protobuf unserialize function
func SetPbUnserializeHook(hook FuncPbUnserializeHook) {
	fnPbUnserializeHook = hook
}

func unserializePb(data []byte) proto.Message {
	if nil == fnPbUnserializeHook {
		return nil
	}

	return fnPbUnserializeHook(data)
}
