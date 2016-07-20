package tcpnetwork

import (
	"github.com/golang/protobuf/proto"
)

type FuncPbUnserializeHook func([]byte) proto.Message

var (
	fnPbUnserializeHook FuncPbUnserializeHook
)

func SetPbUnserializeHook(hook FuncPbUnserializeHook) {
	fnPbUnserializeHook = hook
}

func unserializePb(data []byte) proto.Message {
	if nil == fnPbUnserializeHook {
		return nil
	}

	return fnPbUnserializeHook(data)
}
