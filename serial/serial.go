package serial

import "sync"

type Serial struct {
	cntr uint32
	lock sync.Mutex
}

func New() *Serial {
	return &Serial{}
}

func (s *Serial) Next() uint32 {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.cntr++
	return s.cntr
}

func (s *Serial) Reset() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.cntr = 0
}
