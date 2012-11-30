// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compress

import "sort"

type rotation struct {
        int
        s []uint8
}

type Rotations []rotation

func (r Rotations) Len() int {
        return len(r)
}

func less(a, b rotation) bool {
	la, lb, ia, ib := len(a.s), len(b.s), a.int, b.int
	for {
		if x, y := a.s[ia], b.s[ib]; x != y {
			return x < y
		}
		ia, ib = ia + 1, ib + 1
		if ia == la {
			ia = 0
		}
		if ib == lb {
			ib = 0
		}
		if ia == a.int && ib == b.int {
			break
		}
	}
	return false
}

func (r Rotations) Less(i, j int) bool {
	return less(r[i], r[j])
}

func (r Rotations) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func merge(left, right, out Rotations) {
	for len(left) > 0 && len(right) > 0 {
		if less(left[0], right[0]) {
			out[0], left = left[0], left[1:]
		} else {
			out[0], right = right[0], right[1:]
		}
		out = out[1:]
	}
	copy(out, left)
	copy(out, right)
}

func psort(in Rotations, s chan<- bool) {
	if len(in) < 1024 {
		sort.Sort(in)
		s <- true
		return
	}

	l, r, split := make(chan bool), make(chan bool), len(in) / 2
	left, right := in[:split], in[split:]
	go psort(left, l)
	go psort(right, r)
	_, _ = <-l, <-r
	out := make(Rotations, len(in))
	merge(left, right, out)
	copy(in, out)
	s <- true
}

func BijectiveBurrowsWheelerCoder(input <-chan []byte) Coder8 {
	output := make(chan []byte)

	go func() {
		var lyndon Lyndon
		var rotations Rotations
		wait := make(chan bool)
		var buffer []uint8

		for block := range input {
			if cap(buffer) < len(block) {
				buffer = make([]uint8, len(block))
			} else {
				buffer = buffer[:len(block)]
			}
			copy(buffer, block)
			lyndon.Factor(buffer)

			/* rotate */
			if length := len(block); cap(rotations) < length {
				rotations = make(Rotations, length)
			} else {
				rotations = rotations[:length]
			}
			r := 0
			for _, word := range lyndon.Words {
				for i, _ := range word {
					rotations[r], r = rotation{i, word}, r + 1
				}
			}

			go psort(rotations, wait)
			<-wait

			/* output the last character of each rotation */
			for i, j := range rotations {
				if j.int == 0 {
					j.int = len(j.s)
				}
				block[i] = j.s[j.int - 1]
			}

			output <- block
		}

		close(output)
	}()

	return Coder8{Alphabit:256, Input:output}
}

func BijectiveBurrowsWheelerDecoder(input <-chan []byte) Coder8 {
	inverse := func(buffer []byte) {
		length := len(buffer)
		input, major, minor := make([]byte, length), [256]int {}, make([]int, length)
		for k, v := range buffer {
			input[k], minor[k], major[v] = v, major[v], major[v] + 1
		}

		sum := 0
		for k, v := range major {
			major[k], sum = sum, sum + v
		}

		j := length - 1
		for k, _ := range input {
			for minor[k] != -1 {
				buffer[j], j, k, minor[k] = input[k], j - 1, major[input[k]] + minor[k], -1
			}
		}
	}

	buffer, i := []byte(nil), 0
	add := func(symbol uint8) bool {
		if len(buffer) == 0 {
			next, ok := <-input
			if !ok {
				return true
			}
			buffer = next
		}

		buffer[i], i = symbol, i + 1
		if i == len(buffer) {
			inverse(buffer)
			next, ok := <-input
			if !ok {
				return true
			}
			buffer, i = next, 0
		}
		return false
	}

	return Coder8{Alphabit:256, Output:add}
}
