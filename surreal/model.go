package surreal

import (
	"time"

	"github.com/fxamacker/cbor/v2"
)

const (
	// TAG_TABLE    = 7
	TAG_DATETIME = 12
)

// func Table(name string) *cbor.Tag {
// 	return &cbor.Tag{
// 		Number:  TAG_TABLE,
// 		Content: name,
// 	}
// }

func Datetime(t *time.Time) *cbor.Tag {
	if t == nil {
		return nil
	}

	return &cbor.Tag{
		Number:  TAG_DATETIME,
		Content: [2]int64{t.Unix(), int64(t.Nanosecond())},
	}
}

type RpcReq struct {
	Id     uint32 `cbor:"id"`
	Method string `cbor:"method"`
	Params any    `cbor:"params"`
}

type RpcErr struct {
	Code    int    `cbor:"code"`
	Message string `cbor:"message"`
}

type RpcResp struct {
	Id     uint32           `cbor:"id"`
	Error  *RpcErr          `cbor:"error"`
	Result *cbor.RawMessage `cbor:"result"`
}

type QueryRes struct {
	// Time   string           `cbor:"time"`
	Status string           `cbor:"status"` // "OK" | "ERR"
	Result *cbor.RawMessage `cbor:"result"`
}

type NsUserAuth struct {
	Ns   string `cbor:"ns"`
	User string `cbor:"user"`
	Pass string `cbor:"pass"`
}
