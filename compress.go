// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compress

const (
	BUFFER_COUNT          = 1 << 3
	BUFFER_SIZE           = 1 << 10
	BUFFER_CHAN_SIZE      = BUFFER_COUNT - 2
	BUFFER_POOL_SIZE      = BUFFER_COUNT * BUFFER_SIZE
	BUFFER_POOL_SIZE_MASK = BUFFER_POOL_SIZE - 1
)

type Coder8 struct {
	Alphabit uint16
	Input    <-chan []uint8
	Output   func(symbol uint8) bool
}

type Coder16 struct {
	Alphabit uint16
	Input    <-chan []uint16
	Output   func(symbol uint16) bool
}

const (
	MAX_SCALE16 = (1 << (16 - 2)) - 1
	MAX_SCALE32 = (1 << (32 - 2)) - 1
)

type Symbol struct {
	Scale, Low, High uint16
}

type Symbol32 struct {
	Scale, Low, High uint32
}

type Model struct {
	Scale  uint32
	Fixed  uint32
	Input  <-chan []Symbol
	Output func(code uint16) Symbol
}

type Model32 struct {
	Scale  uint64
	Fixed  uint64
	Input  <-chan []Symbol32
	Output func(code uint32) Symbol32
}
