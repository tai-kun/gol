package serial_test

import (
	"sync"
	"testing"

	"gol/serial"

	"github.com/stretchr/testify/assert"
)

func TestSerialNext(t *testing.T) {
	s := serial.New()

	assert.Equal(t, uint32(1), s.Next())
	assert.Equal(t, uint32(2), s.Next())
	assert.Equal(t, uint32(3), s.Next())
}

func TestSerialReset(t *testing.T) {
	s := serial.New()

	s.Next() // 1
	s.Next() // 2
	s.Next() // 3

	s.Reset()

	assert.Equal(t, uint32(1), s.Next(), "リセット後は 1")
}

func TestSerialConcurrency(t *testing.T) {
	s := serial.New()
	var wg sync.WaitGroup

	const goroutines = 100
	const incrementsPerRoutine = 1000

	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerRoutine; j++ {
				s.Next()
			}
		}()
	}

	wg.Wait()

	expected := uint32(goroutines * incrementsPerRoutine)
	assert.Equal(t, expected+1, s.Next(), "すべてのゴルーチンでインクリメントされる")
}
