package grm

import (
	"strconv"
	"unsafe"
)

// A SQLBuilder is used to efficiently build a string using Write methods.
// It minimizes memory copying. The zero value is ready to use.
// Do not copy a non-zero SQLBuilder.
type SQLBuilder struct {
	addr *SQLBuilder // of receiver, to detect copies by value
	buf  []byte
}

// noescape hides a pointer from escape analysis. It is the identity function
// but escape analysis doesn't think the output depends on the input.
// noescape is inlined and currently compiles down to zero instructions.
// USE CAREFULLY!
// This was copied from the runtime; see issues 23382 and 7921.
//go:nosplit
//go:nocheckptr
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}

func (b *SQLBuilder) copyCheck() {
	if b.addr == nil {
		b.addr = (*SQLBuilder)(noescape(unsafe.Pointer(b)))
	} else if b.addr != b {
		panic("strings: illegal use of non-zero SQLBuilder copied by value")
	}
}

// String returns the accumulated string.
func (b *SQLBuilder) String() string {
	return *(*string)(unsafe.Pointer(&b.buf))
}

// Len returns the number of accumulated bytes; b.Len() == len(b.String()).
func (b *SQLBuilder) Len() int { return len(b.buf) }

// Cap returns the capacity of the builder's underlying byte slice. It is the
// total space allocated for the string being built and includes any bytes
// already written.
func (b *SQLBuilder) Cap() int { return cap(b.buf) }

// Reset resets the SQLBuilder to be empty.
func (b *SQLBuilder) Reset() {
	b.addr = nil
	b.buf = nil
}

// Write appends the contents of p to b's buffer.
// Write always returns len(p), nil.
func (b *SQLBuilder) Write(p []byte) {
	b.copyCheck()
	b.buf = append(b.buf, p...)
}

// WriteByte appends the byte c to b's buffer.
// The returned error is always nil.
func (b *SQLBuilder) WriteByte(c byte) {
	b.copyCheck()
	b.buf = append(b.buf, c)
}

// WriteString appends the contents of s to b's buffer.
// It returns the length of s and a nil error.
func (b *SQLBuilder) WriteString(s string) {
	b.copyCheck()
	b.buf = append(b.buf, s...)
}

func (b *SQLBuilder) WriteInt(i int) {
	b.copyCheck()
	b.buf = append(b.buf, strconv.FormatInt(int64(i), 10)...)
}

// RemoveEnd remove end of string
func (b *SQLBuilder) RemoveEnd(i int) {
	b.copyCheck()
	b.buf = b.buf[:len(b.buf)-i] // it will panic when index not exist
}
