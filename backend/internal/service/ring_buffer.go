package service

import "sync"

// terminalRingBufferSize is the scrollback capacity kept per terminal
// session so a reattaching client can replay recent output (FEAT-015).
// Once a session has produced more output than this, the oldest bytes are
// silently dropped - this is scrollback, not a lossless transcript.
const terminalRingBufferSize = 256 * 1024 // 256KB

// ringBuffer is a fixed-capacity circular byte buffer: once more than its
// capacity has been written in total, the oldest bytes are overwritten by
// the newest ones. Safe for concurrent use.
type ringBuffer struct {
	mu    sync.Mutex
	buf   []byte
	start int // index of the oldest byte currently held
	count int // number of valid bytes currently held (<= len(buf))
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{buf: make([]byte, size)}
}

// Write appends p, evicting the oldest bytes first if p doesn't fit in the
// remaining capacity.
func (r *ringBuffer) Write(p []byte) {
	if len(p) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	size := len(r.buf)
	if size == 0 {
		return
	}

	// If the write alone is bigger than the whole buffer, only its tail
	// could possibly survive - keep just that.
	if len(p) >= size {
		copy(r.buf, p[len(p)-size:])
		r.start = 0
		r.count = size
		return
	}

	n := len(p)
	end := (r.start + r.count) % size
	first := min(n, size-end)
	copy(r.buf[end:end+first], p[:first])
	if first < n {
		copy(r.buf[0:n-first], p[first:])
	}

	if r.count+n <= size {
		r.count += n
	} else {
		overflow := r.count + n - size
		r.start = (r.start + overflow) % size
		r.count = size
	}
}

// Snapshot returns a copy of the bytes currently held, oldest first. The
// returned slice is safe to use after Snapshot returns - it does not alias
// the internal buffer.
func (r *ringBuffer) Snapshot() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == 0 {
		return nil
	}
	out := make([]byte, r.count)
	size := len(r.buf)
	if r.start+r.count <= size {
		copy(out, r.buf[r.start:r.start+r.count])
	} else {
		n := copy(out, r.buf[r.start:])
		copy(out[n:], r.buf[:r.count-n])
	}
	return out
}
