// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compress

import (
	"bytes"
	"io/ioutil"
	"log"
	/*"fmt"*/
	"strconv"
	"testing"
)

var TESTS = [...]string{
	"SIX.MIXED.PIXIES.SIFT.SIXTY.PIXIE.DUST.BOXES",
	"output[j], j, k, minor[k] = s[k], j - 1, major[s[k]] + minor[k], -1",
	"now is the time for the truly nice people to come to the party",
	"Wild Beasts and Their Ways, Reminiscences of Europe, Asia, Africa and America — Volume 1",
	"EEEIT..SXXIT.ESDIXOS..IISXMBSIYSIDXTXIUPF.P.",
	"MXIOTXD.SI.SSTEDXIUIX.X.I.XSPISSFYBPEETEI.I.",
}

func TestSuffixTree(t *testing.T) {
	test := func(input string) {
		tree := BuildSuffixTree([]uint8(input))
		edges, nodes := tree.edges, tree.nodes

		for _, edge := range edges {
			if edge.first_index > edge.last_index {
				t.Errorf("first_index is greater than last_index")
			}
			end := nodes[edge.end_node]
			if end != -1 {
				edge.last_index++
			}
			/*fmt.Printf("%v %v %v '%v'\n", edge.start_node, edge.end_node, end, input[edge.first_index:edge.last_index])*/
		}

		if index := tree.Index(input); index != 0 {
			t.Errorf("index of %v is %v; should be 0", input, index)
		}
	}
	test("banana")
	test("the frightened Mouse splashed his way through the")
}

func TestBurrowsWheeler(t *testing.T) {
	test := func(input string) {
		buffer := make([]byte, len(input))
		copy(buffer, input)
		tree := BuildSuffixTree(buffer)
		bw, sentinel := tree.BurrowsWheelerCoder()
		index, out_buffer := 0, make([]byte, len(buffer)+1)
		for b := range bw {
			out_buffer[index] = b
			index++
		}
		s := <-sentinel
		for b, c := out_buffer[s], s+1; c < len(out_buffer); c++ {
			out_buffer[c], b = b, out_buffer[c]
		}

		/*fmt.Println(strconv.QuoteToASCII(string(out_buffer)))*/
		original := burrowsWheelerDecoder(out_buffer, s)
		if bytes.Compare(buffer, original) != 0 {
			t.Errorf("should be '%v'; got '%v'", input, strconv.QuoteToASCII(string(original)))
		}
	}
	for _, v := range TESTS {
		test(v)
	}
}

const repeated = 10000

func TestBijectiveBurrowsWheeler(t *testing.T) {
	input, output := make(chan []byte), make(chan []byte, 2)
	coder, decoder := BijectiveBurrowsWheelerCoder(input), BijectiveBurrowsWheelerDecoder(output)
	test := func(buffer []byte) {
		for c := 0; c < repeated; c++ {
			input <- buffer
			<-coder.Input
		}

		in := make([]byte, len(buffer))
		for c := 0; c < repeated; c++ {
			output <- in
			output <- nil
			for _, i := range buffer {
				decoder.Output(i)
			}
			copy(buffer, in)
		}
	}
	for _, v := range TESTS {
		buffer := make([]byte, len(v))
		copy(buffer, []byte(v))
		test(buffer)
		if string(buffer) != v {
			t.Errorf("should be '%v'; got '%v'", v, strconv.QuoteToASCII(string(buffer)))
		}
	}
	close(input)
	close(output)
}

func TestMoveToFront(t *testing.T) {
	test := func(buffer []byte) {
		input, output := make(chan []byte), make(chan []byte, 1)
		coder := BijectiveBurrowsWheelerCoder(input).MoveToFrontCoder()
		decoder := BijectiveBurrowsWheelerDecoder(output).MoveToFrontDecoder()

		input <- buffer
		close(input)
		output <- buffer
		close(output)
		for out := range coder.Input {
			for _, symbol := range out {
				decoder.Output(symbol)
			}
		}
	}
	for _, v := range TESTS {
		buffer := make([]byte, len(v))
		copy(buffer, []byte(v))
		test(buffer)
		if string(buffer) != v {
			t.Errorf("inverse should be '%v'; got '%v'", v, strconv.QuoteToASCII(string(buffer)))
		}
	}
}

func TestCode16(t *testing.T) {
	test := []byte("GLIB BATES\x00")
	var table = [256]Symbol{'B': {11, 0, 1},
		'I':    {11, 1, 2},
		'L':    {11, 2, 4},
		' ':    {11, 4, 5},
		'G':    {11, 5, 6},
		'A':    {11, 6, 7},
		'T':    {11, 7, 8},
		'E':    {11, 8, 9},
		'S':    {11, 9, 10},
		'\x00': {11, 10, 11}}
	in, buffer := make(chan []Symbol), &bytes.Buffer{}
	go func() {
		input := make([]Symbol, len(test))
		for i, s := range test {
			input[i] = table[s]
		}
		in <- input
		close(in)
	}()
	Model{Input: in}.Code(buffer)
	if compressed := [...]byte{120, 253, 188, 155, 248}; bytes.Compare(compressed[:], buffer.Bytes()) != 0 {
		t.Errorf("arithmetic coding failed")
	}

	uncompressed, j := make([]byte, len(test)), 0
	lookup := func(code uint16) Symbol {
		for i, symbol := range table {
			if code >= symbol.Low && code < symbol.High {
				uncompressed[j], j = byte(i), j+1
				if i == 0 {
					return Symbol{}
				} else {
					return symbol
				}
			}
		}
		return Symbol{}
	}
	Model{Scale: 11, Output: lookup}.Decode(buffer)
	if bytes.Compare(test, uncompressed) != 0 {
		t.Errorf("arithmetic decoding failed")
	}
}

func TestCode32(t *testing.T) {
	test := []byte("GLIB BATES\x00")
	var table = [256]Symbol32{'B': {11, 0, 1},
		'I':    {11, 1, 2},
		'L':    {11, 2, 4},
		' ':    {11, 4, 5},
		'G':    {11, 5, 6},
		'A':    {11, 6, 7},
		'T':    {11, 7, 8},
		'E':    {11, 8, 9},
		'S':    {11, 9, 10},
		'\x00': {11, 10, 11}}

	in, buffer := make(chan []Symbol32), &bytes.Buffer{}
	go func() {
		input := make([]Symbol32, len(test))
		for i, s := range test {
			input[i] = table[s]
		}
		in <- input
		close(in)
	}()
	Model32{Input: in}.Code(buffer)
	if compressed := [...]byte{120, 254, 27, 129, 174}; bytes.Compare(compressed[:], buffer.Bytes()) != 0 {
		t.Errorf("arithmetic coding failed")
	}

	uncompressed, j := make([]byte, len(test)), 0
	lookup := func(code uint32) Symbol32 {
		for i, symbol := range table {
			if code >= symbol.Low && code < symbol.High {
				uncompressed[j], j = byte(i), j+1
				if i == 0 {
					return Symbol32{}
				} else {
					return symbol
				}
			}
		}
		return Symbol32{}
	}
	Model32{Scale: 11, Output: lookup}.Decode(buffer)
	if bytes.Compare(test, uncompressed) != 0 {
		t.Errorf("arithmetic decoding failed")
	}
}

func TestFiltered(t *testing.T) {
	testFiltered := func(test string) {
		t.Log(test, len(test))
		input := make([]uint16, len(test))
		testBytes := []byte(test)
		for i := range testBytes {
			input[i] = uint16(testBytes[i])
		}
		symbols, buffer := make(chan []uint16, 1), &bytes.Buffer{}
		symbols <- input
		close(symbols)
		Coder16{Alphabit: 256, Input: symbols}.FilteredAdaptiveCoder(NewCDF16).Code(buffer)
		t.Log(buffer.Len())

		out, i := make([]byte, len(test)), 0
		output := func(symbol uint16) bool {
			out[i] = byte(symbol)
			i++
			return i >= len(test)
		}
		Coder16{Alphabit: 256, Output: output}.FilteredAdaptiveDecoder(NewCDF16).Decode(buffer)
		t.Log(string(out))
		if string(out) != test {
			t.Errorf("%v != %v", string(out), test)
		}
	}

	d, err := ioutil.ReadFile("bench/alice30.txt")
	if err != nil {
		log.Fatal(err)
	}
	tests := append(TESTS[:], string(d))

	for _, test := range tests {
		testFiltered(test)
	}
}

func TestMark1(t *testing.T) {
	for _, v := range TESTS {
		output, buffer := make([]byte, len(v)), &bytes.Buffer{}
		Mark1Compress16([]byte(v), buffer)
		Mark1Decompress16(buffer, output)
		if string(output) != v {
			t.Errorf("should be '%v'; got '%v'", v, strconv.QuoteToASCII(string(output)))
		}
	}
}
