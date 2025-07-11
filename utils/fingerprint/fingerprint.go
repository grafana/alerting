package fingerprint

import (
	"encoding/binary"
	"fmt"
	"hash"
	"hash/fnv"
	"unsafe"
)

type Fingerprint uint64

type Hash struct {
	f   hash.Hash64
	tmp [8]byte
}

func NewHash() *Hash {
	return &Hash{
		f: fnv.New64a(),
	}
}

func (f Fingerprint) String() string {
	return fmt.Sprintf("%016x", uint64(f))
}

type WriteFn[T any] func(T) Fingerprint

func (h *Hash) write(p []byte) {
	h.f.Reset()
	_, err := h.f.Write(p)
	if err != nil {
		panic(err)
	}
}

func (h *Hash) Bytes(p []byte) Fingerprint {
	h.write(p)
	return Fingerprint(h.f.Sum64())
}

func (h *Hash) String(s string) Fingerprint {
	return h.Bytes(unsafe.Slice(unsafe.StringData(s), len(s)))
}

func (h *Hash) Int(i int64) Fingerprint {
	buf := h.tmp[0:8]
	binary.LittleEndian.PutUint64(buf, uint64(i))
	return h.Bytes(buf)
}

func (h *Hash) Uint(i uint64) Fingerprint {
	buf := h.tmp[0:8]
	binary.LittleEndian.PutUint64(buf, i)
	return h.Bytes(buf)
}

func (h *Hash) Bool(b bool) Fingerprint {
	if b {
		return h.Bytes([]byte{1})
	}
	return h.Bytes([]byte{0})
}

func Map[TKey comparable, TValue any](m map[TKey]TValue, writeKey WriteFn[TKey], writeValue WriteFn[TValue]) Fingerprint {
	var fpp uint64
	h := NewHash()
	b := make([]byte, 16)
	for k, v := range m {
		fk := writeKey(k)
		fv := writeValue(v)

		binary.LittleEndian.PutUint64(b[0:8], uint64(fk))
		binary.LittleEndian.PutUint64(b[8:16], uint64(fv))

		fpp ^= uint64(h.Bytes(b[0:16]))
	}
	return h.Uint(fpp)
}

func SliceUnordered[T any](s []T, writeElement WriteFn[T]) Fingerprint {
	var fpp uint64
	for _, v := range s {
		fp := writeElement(v)
		fpp ^= uint64(fp)
	}
	return Fingerprint(fpp)
}

func SliceOrdered[T any](s []T, writeElement WriteFn[T]) Fingerprint {
	b := Builder{}
	for _, v := range s {
		b.Append(writeElement(v))
	}
	return b.Fingerprint()
}

type Builder struct {
	f   Fingerprint
	buf [16]byte
	h   *Hash
}

func NewBuilder(h *Hash) *Builder {
	return &Builder{
		h: h,
	}
}

func (b *Builder) Append(p Fingerprint) {
	binary.LittleEndian.PutUint64(b.buf[0:8], uint64(b.f))
	binary.LittleEndian.PutUint64(b.buf[8:16], uint64(p))
	b.f = b.h.Bytes(b.buf[0:16])
}

func (b *Builder) Fingerprint() Fingerprint {
	return b.f
}
