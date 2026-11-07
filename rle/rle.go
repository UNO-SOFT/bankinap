// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package rle contains a hybrid RLE encoder:
// - RLE: Store the initial boolean and the lengths of alternating runs, using unsigned varints.
// - Bitset fallback: If RLE would take more space, pack 8 booleans per byte.
package rle

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Format:
// [mode: 0=bitset, 1=RLE][count: uvarint][payload]

func Encode(values []bool) []byte {
	n := len(values)
	header := binary.AppendUvarint(nil, uint64(n))

	// Bitset encoding
	packed := make([]byte, (n+7)/8)
	for i, v := range values {
		if v {
			packed[i/8] |= 1 << uint(i%8)
		}
	}

	if n == 0 {
		return append([]byte{0}, header...)
	}

	// RLE encoding: initial value + alternating run lengths
	rle := []byte{0}
	if values[0] {
		rle[0] = 1
	}

	run := uint64(1)
	for i := 1; i < n; i++ {
		if values[i] == values[i-1] {
			run++
		} else {
			rle = binary.AppendUvarint(rle, run)
			run = 1
		}
	}
	rle = binary.AppendUvarint(rle, run)

	// Choose the smaller encoding
	if len(rle) < len(packed) {
		return append(append([]byte{1}, header...), rle...)
	}
	return append(append([]byte{0}, header...), packed...)
}

func Decode(encoded []byte) ([]bool, error) {
	if len(encoded) < 2 {
		return nil, errors.New("missing header")
	}

	mode := encoded[0]
	count, read := binary.Uvarint(encoded[1:])
	if read <= 0 || count > 1_000_000 {
		return nil, errors.New("invalid length")
	}

	payload := encoded[1+read:]
	n := int(count)
	out := make([]bool, n)

	switch mode {
	case 0: // Bitset
		if len(payload) != (n+7)/8 {
			return nil, errors.New("invalid bitset size")
		}
		for i := range out {
			out[i] = payload[i/8]&(1<<uint(i%8)) != 0
		}

	case 1: // RLE
		if n == 0 || len(payload) < 2 || payload[0] > 1 {
			return nil, errors.New("invalid RLE header")
		}

		value := payload[0] == 1
		payload = payload[1:]
		pos := 0

		for len(payload) > 0 {
			run, used := binary.Uvarint(payload)
			if used <= 0 || run == 0 || run > uint64(n-pos) {
				return nil, errors.New("invalid run length")
			}

			for j := 0; j < int(run); j++ {
				out[pos+j] = value
			}

			pos += int(run)
			payload = payload[used:]
			value = !value
		}

		if pos != n {
			return nil, errors.New("RLE length mismatch")
		}

	default:
		return nil, fmt.Errorf("unknown encoding mode: %d", mode)
	}

	return out, nil
}
