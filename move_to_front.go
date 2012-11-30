// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compress

func (coder Coder8) MoveToFrontCoder() Coder16 {
	symbols := make(chan []uint16, BUFFER_CHAN_SIZE)

	go func() {
		nodes, buffer := [256]byte{}, [BUFFER_POOL_SIZE]uint16{}
		var first byte

		for node, _ := range nodes {
			nodes[node] = uint8(node) + 1
		}

		current, offset, index := buffer[0:BUFFER_SIZE], BUFFER_SIZE, 0
		for block := range coder.Input {
			for _, v := range block {
				var node, next byte
				var symbol uint16
				for next = first; next != v; node, next = next, nodes[next] {
					symbol++
				}

				current[index], index = symbol, index + 1
				if symbol != 0 {
					first, nodes[node], nodes[next] = next, nodes[next], first
				}

				if index == BUFFER_SIZE {
					symbols <- current
					next := offset + BUFFER_SIZE
					current, offset, index = buffer[offset:next], next & BUFFER_POOL_SIZE_MASK, 0
				}
 			}
		}

		symbols <- current[:index]
		close(symbols)
	}()

	return Coder16{Alphabit:256, Input:symbols}
}

func (coder Coder8) MoveToFrontDecoder() Coder16 {
	nodes := [256]byte{}
	var first byte

	for node, _ := range nodes {
		nodes[node] = uint8(node) + 1
	}

	output := func(symbol uint16) bool {
		var node, next byte
		moveToFront := symbol != 0
		for next = first; symbol > 0; node, next = next, nodes[next] {
			symbol--
		}

		if moveToFront {
			first, nodes[node], nodes[next] = next, nodes[next], first
		}

		return coder.Output(next)
	}

	return Coder16{Alphabit:256, Output:output}
}

func (coder Coder8) MoveToFrontRunLengthCoder() Coder16 {
	symbols := make(chan []uint16, BUFFER_CHAN_SIZE)

	go func() {
		var buffer [BUFFER_POOL_SIZE]uint16
		current, offset, index, length := buffer[0:BUFFER_SIZE], BUFFER_SIZE, 0, uint64(0)
		outputSymbol := func(symbol uint16) {
			current[index], index = symbol, index + 1
			if index == BUFFER_SIZE {
				symbols <- current
				next := offset + BUFFER_SIZE
				current, offset, index = buffer[offset:next], next & BUFFER_POOL_SIZE_MASK, 0
			}
		}
		outputLength := func() {
			if length > 0 {
				length--
				outputSymbol(uint16(length & 1))
				for length > 1 {
					length = (length - 2) >> 1
					outputSymbol(uint16(length & 1))
				}
				length = 0
			}
		}

		var nodes [256]byte
		var first byte
		for node, _ := range nodes {
			nodes[node] = byte(node) + 1
		}

		for block := range coder.Input {
			for _, v := range block {
				var node, next byte
				var symbol uint16
				for next = first; next != v; node, next = next, nodes[next] {
					symbol++
				}

				if symbol == 0 {
					length++
					continue
				}

				first, nodes[node], nodes[next] = next, nodes[next], first

				outputLength()
				outputSymbol(symbol + 1)
 			}
		}

		outputLength()
		symbols <- current[:index]
		close(symbols)
	}()

	return Coder16{Alphabit:257, Input:symbols}
}

func (coder Coder8) MoveToFrontRunLengthDecoder() Coder16 {
	var nodes [256]byte
	var first byte

	for node, _ := range nodes {
		nodes[node] = uint8(node) + 1
	}

	length := uint64(1)
	output := func(symbol uint16) bool {
		if symbol > 1 {
			var node, next byte
			symbol, length = symbol - 1, 1
			for next = first; symbol > 0; node, next = next, nodes[next] {
				symbol--
			}
			first, nodes[node], nodes[next] = next, nodes[next], first
			return coder.Output(next)
		}

		for c := length << symbol; c > 0; c-- {
			if coder.Output(first) {
				return true
			}
		}
		length <<= 1

		return false
	}

	return Coder16{Alphabit:257, Output:output}
}
