// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package zlib implements reading and writing of zlib format compressed data,
as specified in RFC 1950.

The implementation provides filters that uncompress during reading
and compress during writing.  For example, to write compressed data
to a buffer:

	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	w.Write([]byte("hello, world\n"))
	w.Close()

and to read that data back:

	r, err := zlib.NewReader(&b)
	io.Copy(os.Stdout, r)
	r.Close()
*/
package zlib

import (
	"bufio"
	"compress/flate"
	"errors"
	"hash"
	"hash/adler32"
	"io"
)

const zlibDeflate = 8

var (
	// ErrChecksum is returned when reading ZLIB data that has an invalid checksum.
	ErrChecksum = errors.New("zlib: invalid checksum")
	// ErrDictionary is returned when reading ZLIB data that has an invalid dictionary.
	ErrDictionary = errors.New("zlib: invalid dictionary")
	// ErrHeader is returned when reading ZLIB data that has an invalid header.
	ErrHeader = errors.New("zlib: invalid header")
)

type zlib struct {
	r            flate.Reader
	decompressor io.ReadCloser
	digest       hash.Hash32
	err          error
	scratch      [4]byte
}

// NewReader creates a new io.ReadCloser.
// Reads from the returned io.ReadCloser read and decompress data from r.
// The implementation buffers input and may read more data than necessary from r.
// It is the caller's responsibility to call Close on the ReadCloser when done.
func NewReader(r io.Reader) (io.ReadCloser, error) {
	return NewReaderDict(r, nil)
}

// NewReaderDict is like NewReader but uses a preset dictionary.
// NewReaderDict ignores the dictionary if the compressed data does not refer to it.
func NewReaderDict(r io.Reader, dict []byte) (io.ReadCloser, error) {
	z := new(zlib)
	if fr, ok := r.(flate.Reader); ok {
		z.r = fr
	} else {
		z.r = bufio.NewReader(r)
	}
	_, err := io.ReadFull(z.r, z.scratch[0:2])
	if err != nil {
		return nil, err
	}
	h := uint(z.scratch[0])<<8 | uint(z.scratch[1])
	if (z.scratch[0]&0x0f != zlibDeflate) || (h%31 != 0) {
		return nil, ErrHeader
	}
	if z.scratch[1]&0x20 != 0 {
		_, err = io.ReadFull(z.r, z.scratch[0:4])
		if err != nil {
			return nil, err
		}
		checksum := uint32(z.scratch[0])<<24 | uint32(z.scratch[1])<<16 | uint32(z.scratch[2])<<8 | uint32(z.scratch[3])
		if checksum != adler32.Checksum(dict) {
			return nil, ErrDictionary
		}
		z.decompressor = flate.NewReaderDict(z.r, dict)
	} else {
		z.decompressor = flate.NewReader(z.r)
	}
	z.digest = adler32.New()
	return z, nil
}

func (z *zlib) Read(p []byte) (n int, err error) {
	if z.err != nil {
		return 0, z.err
	}
	if len(p) == 0 {
		return 0, nil
	}

	n, err = z.decompressor.Read(p)
	z.digest.Write(p[0:n])
	if n != 0 || (err != io.EOF && err != io.ErrUnexpectedEOF) {
		z.err = err
		return
	}

	// Don't worry if we got an unexpected EOF, we've just finished the currently
	// available bytes.
	if err == io.ErrUnexpectedEOF {
		err = nil
		return
	}

	// Finished file; check checksum.
	if _, err := io.ReadFull(z.r, z.scratch[0:4]); err != nil {
		z.err = err
		return 0, err
	}
	// ZLIB (RFC 1950) is big-endian, unlike GZIP (RFC 1952).
	checksum := uint32(z.scratch[0])<<24 | uint32(z.scratch[1])<<16 | uint32(z.scratch[2])<<8 | uint32(z.scratch[3])
	if checksum != z.digest.Sum32() {
		z.err = ErrChecksum
		return 0, z.err
	}
	return
}

// Calling Close does not close the wrapped io.Reader originally passed to NewReader.
func (z *zlib) Close() error {
	if z.err != nil {
		return z.err
	}
	z.err = z.decompressor.Close()
	return z.err
}
