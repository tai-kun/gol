package surreal

import (
	"errors"
	"gol/serial"
	"log"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/gorilla/websocket"
)

func At[T any](q *[]QueryRes, i int) (*T, error) {
	if i < 0 || i > len(*q)-1 {
		return nil, errors.New("out of range")
	}

	v := (*q)[i]
	if v.Status != "OK" {
		var r string
		if err := cbor.Unmarshal(*v.Result, &r); err != nil {
			return nil, err
		}

		return nil, errors.New(r)
	}

	var t T
	if err := cbor.Unmarshal(*v.Result, &t); err != nil {
		return nil, err
	}

	return &t, nil
}

type Surreal struct {
	id        *serial.Serial
	ws        *websocket.Conn
	ns        string
	host      string
	CloseErr  error
	CloseChan chan struct{}
	respChans map[uint32]chan RpcResp
	wsLock    sync.Mutex
	respLock  sync.RWMutex
	// useLock   sync.Mutex
}

func New() *Surreal {
	return &Surreal{}
}

func (s *Surreal) Connect(host string) error {
	if s.ws != nil {
		if s.host == host {
			return nil
		}

		return errors.New("connection conflict between " + s.host + " and " + host)
	}

	url := url.URL{Scheme: "ws", Host: host, Path: "/rpc"}
	dialer := websocket.DefaultDialer
	dialer.EnableCompression = true
	dialer.Subprotocols = append(dialer.Subprotocols, "cbor")
	ws, _, err := dialer.Dial(url.String(), nil)
	if err != nil {
		return err
	}

	s.id = serial.New()
	s.ws = ws
	s.host = host
	s.CloseErr = nil
	s.CloseChan = make(chan struct{})
	s.respChans = make(map[uint32]chan RpcResp)
	go s.listen()

	return nil
}

func (s *Surreal) Close() error {
	s.wsLock.Lock()

	if s.ws == nil {
		s.wsLock.Unlock()
		return nil
	}

	defer func() {
		s.id.Reset()
		s.ws = nil
		s.ns = ""
		s.host = ""
		s.CloseChan = nil
		s.respChans = nil
		s.wsLock.Unlock()
	}()
	close(s.CloseChan)
	errs := make([]error, 0)
	err := s.ws.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	if err != nil {
		errs = append(errs, err)
	}

	err = s.ws.Close()
	if err != nil {
		if websocket.IsCloseError(
			err,
			// 正常系
			websocket.CloseNormalClosure, // 1000
			// 早期切断に由来するエラー
			websocket.CloseGoingAway,       // 1001
			websocket.CloseAbnormalClosure, // 1006
		) {
			err = nil
		}

		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (s *Surreal) UseNs(ns string) error {
	if ns == "" {
		return errors.New("no namespace")
	}

	_, err := s.rpc("use", [2]*string{&ns, nil})
	if err != nil {
		return err
	}

	s.ns = ns

	return nil
}

func (s *Surreal) UseDb(db string) error {
	if s.ns == "" {
		return errors.New("no namespace")
	}

	_, err := s.rpc("use", [2]string{s.ns, db})

	return err
}

func (s *Surreal) Signin(user, pass string) error {
	if s.ns == "" {
		return errors.New("no namespace")
	}

	_, err := s.rpc("signin", [1]NsUserAuth{{
		Ns:   s.ns,
		User: user,
		Pass: pass,
	}})

	return err
}

func (s *Surreal) Query(query string, vars any) (*[]QueryRes, error) {
	msg, err := s.rpc("query", [2]any{query, vars})
	if err != nil {
		return nil, err
	}

	var res []QueryRes
	if err := cbor.Unmarshal(*msg, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

func (s *Surreal) listen() {
	for {
		select {
		case <-s.CloseChan:
			return
		default:
			_, data, err := s.ws.ReadMessage()
			if err != nil {
				switch {
				case errors.Is(err, net.ErrClosed):
					s.CloseErr = err
					return
				default:
					s.CloseErr = err
					<-s.CloseChan
					return
				}
			}

			var resp RpcResp
			err = cbor.Unmarshal(data, &resp)
			if err != nil {
				log.Println("decode cbor failed:", err)
				continue
			}

			// fmt.Println(resp)

			if respChan, exists := s.getChan(resp.Id); exists {
				respChan <- resp
			}
		}
	}
}

func (s *Surreal) rpc(method string, params any) (*cbor.RawMessage, error) {
	select {
	case <-s.CloseChan:
		return nil, s.CloseErr
	default:
	}

	id := s.id.Next()
	respChan, err := s.setChan(id)
	if err != nil {
		return nil, err
	}
	defer s.delChan(id)

	req := RpcReq{
		Id:     id,
		Method: method,
		Params: params,
	}
	if err := s.write(req); err != nil {
		return nil, err
	}

	select {
	case <-time.After(5 * time.Second):
		return nil, errors.New("'" + method + "' rpc timed out after 5 secconds")
	case resp, open := <-respChan:
		if !open {
			return nil, errors.New(
				"'" + method + "' rpc channel(" +
					strconv.FormatUint(uint64(id), 10) + ") is closed",
			)
		}
		if resp.Error != nil {
			return nil, errors.New(
				"'" + method + "' rpc (" +
					strconv.FormatUint(uint64(id), 10) +
					") is failed (" +
					strconv.FormatInt(int64(resp.Error.Code), 10) +
					"): " + resp.Error.Message,
			)
		}

		return resp.Result, nil
	}
}

func (s *Surreal) write(req RpcReq) error {
	v, err := cbor.Marshal(req)
	if err != nil {
		return err
	}

	s.wsLock.Lock()
	defer s.wsLock.Unlock()
	return s.ws.WriteMessage(websocket.BinaryMessage, v)
}

func (s *Surreal) setChan(id uint32) (chan RpcResp, error) {
	s.respLock.Lock()
	defer s.respLock.Unlock()
	if _, exists := s.respChans[id]; exists {
		return nil, errors.New(
			"rpc request id " + strconv.FormatUint(uint64(id), 10) + " is in use",
		)
	}

	respChan := make(chan RpcResp)
	s.respChans[id] = respChan

	return respChan, nil
}

func (s *Surreal) getChan(id uint32) (chan RpcResp, bool) {
	s.respLock.RLock()
	defer s.respLock.RUnlock()
	respChan, exists := s.respChans[id]

	return respChan, exists
}

func (s *Surreal) delChan(id uint32) {
	s.respLock.Lock()
	defer s.respLock.Unlock()
	delete(s.respChans, id)
}
