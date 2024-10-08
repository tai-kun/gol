package out

import (
	"bytes"
	"gol/surreal"
	"time"

	"github.com/fxamacker/cbor/v2"
)

var (
	LF   = []byte("\n")
	CR   = []byte("\r")
	CRLF = []byte("\r\n")
)

var (
	GROUP     = []byte("::group::")
	GROUP_END = []byte("::endgroup::")
)

type OutData struct {
	Message string    `cbor:"msg"`
	Time    *cbor.Tag `cbor:"time"`
}

type Out struct {
	Ch   chan *OutData
	msg  []byte
	time *time.Time
}

func New() *Out {
	return &Out{
		Ch: make(chan *OutData),
	}
}

func (o *Out) Write(b []byte) (int, error) {
	l := len(b)

	if l == 0 {
		return 0, nil
	}

	if o.time == nil {
		t := time.Now()
		o.time = &t
	}

	b = bytes.ReplaceAll(b, CRLF, LF)
	b = bytes.ReplaceAll(b, CR, LF)

	if b[l-1] == 10 { // 10 == \n
		if b, has := bytes.CutPrefix(b[:l-1], GROUP); has {
			o.msg = append(o.msg, b...)
			o.Ch <- &OutData{
				Message: "BEGIN",
				Time:    nil,
			}
			o.Ch <- o.Consume()
		} else if bytes.Equal(b, GROUP_END) {
			o.Ch <- o.Consume()
			o.Ch <- &OutData{
				Message: "END",
				Time:    nil,
			}
		} else {
			o.msg = append(o.msg, b...)
			o.Ch <- o.Consume()
		}
	} else {
		if b, has := bytes.CutPrefix(b, GROUP); has {
			o.msg = append(o.msg, b...)
			o.Ch <- &OutData{
				Message: "BEGIN",
				Time:    nil,
			}
			o.Ch <- o.Consume()
		} else if bytes.Equal(b, GROUP_END) {
			o.Ch <- o.Consume()
			o.Ch <- &OutData{
				Message: "END",
				Time:    nil,
			}
		} else {
			o.msg = append(o.msg, b...)
		}
	}

	return l, nil
}

func (o *Out) Consume() *OutData {
	data := &OutData{
		Message: string(o.msg),
		Time:    surreal.Datetime(o.time),
	}
	o.reset()

	return data
}

func (o *Out) reset() {
	o.msg = []byte{}
	o.time = nil
}
