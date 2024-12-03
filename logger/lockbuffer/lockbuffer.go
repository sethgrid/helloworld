// Package lockbuffer provides a thread-safe buffer that can be leveraged during testing to capture logs or other output.

package lockbuffer

import (
	"bytes"
	"io"
	"sync"
)

// LockBuffer is a thread-safe ReadWriter that wraps a bytes.Buffer
type LockBuffer struct {
	mu     sync.Mutex
	buffer *bytes.Buffer
}

// NewLockBuffer creates a new LockBuffer instance
func NewLockBuffer() *LockBuffer {
	return &LockBuffer{
		buffer: bytes.NewBuffer(nil),
	}
}

// Write writes to the buffer safely
func (lb *LockBuffer) Write(p []byte) (int, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buffer.Write(p)
}

// Read reads from the buffer safely
func (lb *LockBuffer) Read(p []byte) (int, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buffer.Read(p)
}

// WriteTo implements the io.WriterTo interface, writing the buffer's content to the given Writer safely
func (lb *LockBuffer) WriteTo(w io.Writer) (int64, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buffer.WriteTo(w)
}

// ReadFrom implements the io.ReaderFrom interface, reading content into the buffer from the given Reader safely
func (lb *LockBuffer) ReadFrom(r io.Reader) (int64, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buffer.ReadFrom(r)
}

// Bytes returns the buffer's content as a byte slice safely
func (lb *LockBuffer) Bytes() []byte {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buffer.Bytes()
}

// String returns the buffer's content as a string safely
func (lb *LockBuffer) String() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buffer.String()
}
