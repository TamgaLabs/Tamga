package service

import (
	"bytes"
	"fmt"
	"testing"
)

func TestRingBufferUnderCapacity(t *testing.T) {
	r := newRingBuffer(16)
	r.Write([]byte("hello"))
	r.Write([]byte(" world"))
	if got := r.Snapshot(); !bytes.Equal(got, []byte("hello world")) {
		t.Fatalf("got %q, want %q", got, "hello world")
	}
}

func TestRingBufferEmpty(t *testing.T) {
	r := newRingBuffer(16)
	if got := r.Snapshot(); got != nil {
		t.Fatalf("expected nil snapshot for empty buffer, got %q", got)
	}
}

func TestRingBufferOverwritesOldest(t *testing.T) {
	r := newRingBuffer(8)
	r.Write([]byte("abcdefgh")) // exactly fills the buffer
	r.Write([]byte("ij"))       // evicts "ab"
	if got := r.Snapshot(); !bytes.Equal(got, []byte("cdefghij")) {
		t.Fatalf("got %q, want %q", got, "cdefghij")
	}
}

func TestRingBufferSingleWriteBiggerThanCapacity(t *testing.T) {
	r := newRingBuffer(4)
	r.Write([]byte("abcdefgh"))
	if got := r.Snapshot(); !bytes.Equal(got, []byte("efgh")) {
		t.Fatalf("got %q, want %q (only the tail should survive)", got, "efgh")
	}
}

// TestRingBufferManySmallWrites exercises the wraparound path with many
// small writes (as terminal output arrives in real usage) and checks the
// final content is exactly the last N bytes ever written, in order.
func TestRingBufferManySmallWrites(t *testing.T) {
	const size = 100
	r := newRingBuffer(size)

	var all bytes.Buffer
	for i := 0; i < 500; i++ {
		chunk := []byte(fmt.Sprintf("%03d-", i))
		all.Write(chunk)
		r.Write(chunk)
	}

	want := all.Bytes()
	want = want[len(want)-size:]
	if got := r.Snapshot(); !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRingBufferZeroCapacity(t *testing.T) {
	r := newRingBuffer(0)
	r.Write([]byte("anything"))
	if got := r.Snapshot(); got != nil {
		t.Fatalf("expected nil snapshot for zero-capacity buffer, got %q", got)
	}
}
