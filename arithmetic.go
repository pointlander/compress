// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compress

import "io"

func (model Model) Code(out io.Writer) int {
	count := 0
	var bits [1]byte
	var mask uint8 = 0x80

	output := func(bit uint16) {
		count++
		if bit != 0 {
			bits[0] |= mask
		}
		if mask >>= 1; mask == 0 {
			out.Write(bits[:])
			bits[0], mask = 0, 0x80
		}
	}

	var low, high uint16 = 0, 0xffff
	var underflow uint32
	for current := range model.Input {
		for _, s := range current {
			hl, scale := uint32(high-low)+1, uint32(s.Scale)
			low, high = low+uint16((hl*uint32(s.Low))/scale), low+uint16((hl*uint32(s.High))/scale-1)

			for {
				if low&0x8000 == high&0x8000 {
					output(high & 0x8000)
					for underflow > 0 {
						output(^high & 0x8000)
						underflow--
					}
				} else if low&0x4000 != 0 && high&0x4000 == 0 {
					low, high, underflow = low&0x3fff, high|0x4000, underflow+1
				} else {
					break
				}
				low, high = low<<1, (high<<1)|1
			}
		}
	}

	output(low & 0x4000)
	underflow++
	for underflow > 0 {
		output(^low & 0x4000)
		underflow--
	}
	if mask != 0x80 {
		out.Write(bits[:])
	}

	return count
}

func (model Model) Decode(in io.Reader) {
	var bits [1]byte
	var mask uint8 = 0x80
	var code uint16

	input := func() {
		if code <<= 1; bits[0]&mask != 0 {
			code |= 1
		}

		if mask >>= 1; mask == 0 {
			if n, _ := in.Read(bits[:]); n > 0 {
				mask = 0x80
			}
		}
	}

	in.Read(bits[:])
	for i := 0; i < 16; i++ {
		input()
	}

	var low, high uint16 = 0, 0xffff
	hl := uint32(high-low) + 1

	s := model.Output(uint16(((uint32(code-low)+1)*model.Scale - 1) / hl))
	for s.High != 0 {
		low, high = low+uint16((hl*uint32(s.Low))/model.Scale), low+uint16((hl*uint32(s.High))/model.Scale-1)

		for {
			if low&0x8000 == high&0x8000 {

			} else if low&0x4000 != 0 && high&0x4000 == 0 {
				low, high, code = low&0x3fff, high|0x4000, code^0x4000
			} else {
				hl, model.Scale = uint32(high-low)+1, uint32(s.Scale)
				break
			}
			input()
			low, high = low<<1, (high<<1)|1
		}

		s = model.Output(uint16(((uint32(code-low)+1)*model.Scale - 1) / hl))
	}
}

func (model Model32) Code(out io.Writer) int {
	count := 0
	var bits [1]byte
	var mask uint8 = 0x80

	output := func(bit uint32) {
		count++
		if bit != 0 {
			bits[0] |= mask
		}
		if mask >>= 1; mask == 0 {
			out.Write(bits[:])
			bits[0], mask = 0, 0x80
		}
	}

	var low, high uint32 = 0, 0xffffffff
	var underflow uint32

	for current := range model.Input {
		for _, s := range current {
			hl, scale := uint64(high-low)+1, uint64(s.Scale)
			low, high = low+uint32((hl*uint64(s.Low))/scale), low+uint32((hl*uint64(s.High))/scale-1)

			for {
				if low&0x80000000 == high&0x80000000 {
					output(high & 0x80000000)
					for underflow > 0 {
						output(^high & 0x80000000)
						underflow--
					}
				} else if low&0x40000000 != 0 && high&0x40000000 == 0 {
					low, high, underflow = low&0x3fffffff, high|0x40000000, underflow+1
				} else {
					break
				}
				low, high = low<<1, (high<<1)|1
			}
		}
	}

	output(low & 0x40000000)
	underflow++
	for underflow > 0 {
		output(^low & 0x40000000)
		underflow--
	}
	if mask != 0x80 {
		out.Write(bits[:])
	}

	return count
}

func (model Model32) Decode(in io.Reader) {
	var bits [1]byte
	var mask uint8 = 0x80
	var code uint32

	input := func() {
		if code <<= 1; bits[0]&mask != 0 {
			code |= 1
		}

		if mask >>= 1; mask == 0 {
			if n, _ := in.Read(bits[:]); n > 0 {
				mask = 0x80
			}
		}
	}

	in.Read(bits[:])
	for i := 0; i < 32; i++ {
		input()
	}

	var low, high uint32 = 0, 0xffffffff
	hl := uint64(high-low) + 1

	s := model.Output(uint32(((uint64(code-low)+1)*model.Scale - 1) / hl))
	for s.High != 0 {
		low, high = low+uint32((hl*uint64(s.Low))/model.Scale), low+uint32((hl*uint64(s.High))/model.Scale-1)

		for {
			if low&0x80000000 == high&0x80000000 {

			} else if low&0x40000000 != 0 && high&0x40000000 == 0 {
				low, high, code = low&0x3fffffff, high|0x40000000, code^0x40000000
			} else {
				hl, model.Scale = uint64(high-low)+1, uint64(s.Scale)
				break
			}
			input()
			low, high = low<<1, (high<<1)|1
		}

		s = model.Output(uint32(((uint64(code-low)+1)*model.Scale - 1) / hl))
	}
}
