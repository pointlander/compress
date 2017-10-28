// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compress

const (
	filterScale = 4096
	filterShift = 5
)

func (coder Coder16) AdaptiveCoder() Model {
	out := make(chan []Symbol, BUFFER_CHAN_SIZE)

	go func() {
		table, scale, buffer := make([]uint16, coder.Alphabit), uint16(0), [BUFFER_POOL_SIZE]Symbol{}
		for i, _ := range table {
			table[i] = 1
			scale += 1
		}

		current, offset, index := buffer[0:BUFFER_SIZE], BUFFER_SIZE, 0
		for input := range coder.Input {
			for _, s := range input {
				low := uint16(0)
				for _, count := range table[:s] {
					low += count
				}

				current[index], index = Symbol{Scale: scale, Low: low, High: low + table[s]}, index+1
				if index == BUFFER_SIZE {
					out <- current
					next := offset + BUFFER_SIZE
					current, offset, index = buffer[offset:next], next&BUFFER_POOL_SIZE_MASK, 0
				}

				scale++
				table[s]++
				if scale > MAX_SCALE16 {
					scale = 0
					for i, count := range table {
						if count >>= 1; count == 0 {
							table[i], scale = 1, scale+1
						} else {
							table[i], scale = count, scale+count
						}
					}
				}
			}
		}

		out <- current[:index]
		close(out)
	}()

	return Model{Input: out}
}

func (coder Coder16) AdaptivePredictiveCoder() Model {
	out := make(chan []Symbol, BUFFER_CHAN_SIZE)

	go func() {
		table, scale, context, buffer := make([][]uint16, coder.Alphabit), make([]uint16, coder.Alphabit), uint16(0), [BUFFER_POOL_SIZE]Symbol{}
		for i, _ := range table {
			table[i] = make([]uint16, coder.Alphabit)
			scale[i] = coder.Alphabit
			for j, _ := range table[i] {
				table[i][j] = 1
			}
		}

		current, offset, index := buffer[0:BUFFER_SIZE], BUFFER_SIZE, 0
		for input := range coder.Input {
			for _, s := range input {
				low := uint16(0)
				for _, count := range table[context][:s] {
					low += count
				}

				current[index], index = Symbol{Scale: scale[context], Low: low, High: low + table[context][s]}, index+1
				if index == BUFFER_SIZE {
					out <- current
					next := offset + BUFFER_SIZE
					current, offset, index = buffer[offset:next], next&BUFFER_POOL_SIZE_MASK, 0
				}

				scale[context]++
				table[context][s]++
				if scale[context] > MAX_SCALE16 {
					scale[context] = 0
					for i, count := range table[context] {
						if count >>= 1; count == 0 {
							table[context][i], scale[context] = 1, scale[context]+1
						} else {
							table[context][i], scale[context] = count, scale[context]+count
						}
					}
				}

				context = s
			}
		}

		out <- current[:index]
		close(out)
	}()

	return Model{Input: out}
}

func (coder Coder16) AdaptiveBitCoder() Model {
	out := make(chan []Symbol, BUFFER_CHAN_SIZE)

	go func() {
		table, buffer := [2]uint16{}, [BUFFER_POOL_SIZE]Symbol{}
		table[0] = 1
		table[1] = 1

		highest := uint32(0)
		for a := coder.Alphabit - 1; a > 0; a >>= 1 {
			highest++
		}

		current, offset, index, mask := buffer[0:BUFFER_SIZE], BUFFER_SIZE, 0, uint16(1)<<(highest-1)
		for input := range coder.Input {
			for _, s := range input {
				for bit := mask; bit > 0; bit >>= 1 {
					b, low, high, scale := uint16(0), uint16(0), table[0], table[0]+table[1]
					if bit&s != 0 {
						b, low, high = 1, high, scale
					}

					current[index], index = Symbol{Scale: scale, Low: low, High: high}, index+1
					if index == BUFFER_SIZE {
						out <- current
						next := offset + BUFFER_SIZE
						current, offset, index = buffer[offset:next], next&BUFFER_POOL_SIZE_MASK, 0
					}

					table[b]++
					if scale >= MAX_SCALE16 {
						table[0] >>= 1
						table[1] >>= 1
						if table[0] == 0 {
							table[0] = 1
						}
						if table[1] == 0 {
							table[1] = 1
						}
					}
				}
			}
		}

		out <- current[:index]
		close(out)
	}()

	return Model{Input: out}
}

func (coder Coder16) AdaptivePredictiveBitCoder() Model {
	out := make(chan []Symbol, BUFFER_CHAN_SIZE)

	go func() {
		table, context, buffer := make([][2]uint16, 65536), uint16(0), [BUFFER_POOL_SIZE]Symbol{}
		for i, _ := range table {
			table[i][0] = 1
			table[i][1] = 1
		}

		highest := uint32(0)
		for a := coder.Alphabit - 1; a > 0; a >>= 1 {
			highest++
		}

		current, offset, index, mask := buffer[0:BUFFER_SIZE], BUFFER_SIZE, 0, uint16(1)<<(highest-1)
		for input := range coder.Input {
			for _, s := range input {
				for bit := mask; bit > 0; bit >>= 1 {
					b, low, high, scale := uint16(0), uint16(0), table[context][0], table[context][0]+table[context][1]
					if bit&s != 0 {
						b, low, high = 1, high, scale
					}

					current[index], index = Symbol{Scale: scale, Low: low, High: high}, index+1
					if index == BUFFER_SIZE {
						out <- current
						next := offset + BUFFER_SIZE
						current, offset, index = buffer[offset:next], next&BUFFER_POOL_SIZE_MASK, 0
					}

					table[context][b]++
					if scale >= MAX_SCALE16 {
						table[context][0] >>= 1
						table[context][1] >>= 1
						if table[context][0] == 0 {
							table[context][0] = 1
						}
						if table[context][1] == 0 {
							table[context][1] = 1
						}
					}

					context = b | (context << 1)
				}
			}
		}

		out <- current[:index]
		close(out)
	}()

	return Model{Input: out}
}

// https://fgiesen.wordpress.com/2015/05/26/models-for-adaptive-arithmetic-coding/
func (coder Coder16) FilteredAdaptiveBitCoder() Model {
	out := make(chan []Symbol, BUFFER_CHAN_SIZE)

	go func() {
		const scale = uint16(filterScale)
		p1 := scale / 2

		buffer := [BUFFER_POOL_SIZE]Symbol{}

		highest := uint32(0)
		for a := coder.Alphabit - 1; a > 0; a >>= 1 {
			highest++
		}

		current, offset, index, mask := buffer[0:BUFFER_SIZE], BUFFER_SIZE, 0, uint16(1)<<(highest-1)
		for input := range coder.Input {
			for _, s := range input {
				for bit := mask; bit > 0; bit >>= 1 {
					b, low, high := uint16(0), uint16(0), scale-p1
					if bit&s != 0 {
						b, low, high = 1, high, scale
					}

					current[index], index = Symbol{Scale: scale, Low: low, High: high}, index+1
					if index == BUFFER_SIZE {
						out <- current
						next := offset + BUFFER_SIZE
						current, offset, index = buffer[offset:next], next&BUFFER_POOL_SIZE_MASK, 0
					}

					if b == 0 {
						p1 -= p1 >> filterShift
					} else {
						p1 += (scale - p1) >> filterShift
					}
				}
			}
		}

		out <- current[:index]
		close(out)
	}()

	return Model{Input: out}
}

func (coder Coder16) FilteredAdaptivePredictiveBitCoder() Model {
	out := make(chan []Symbol, BUFFER_CHAN_SIZE)

	go func() {
		const scale = uint16(filterScale)
		table, context, buffer := make([]uint16, 65536), uint16(0), [BUFFER_POOL_SIZE]Symbol{}
		for i, _ := range table {
			table[i] = scale / 2
		}

		highest := uint32(0)
		for a := coder.Alphabit - 1; a > 0; a >>= 1 {
			highest++
		}

		current, offset, index, mask := buffer[0:BUFFER_SIZE], BUFFER_SIZE, 0, uint16(1)<<(highest-1)
		for input := range coder.Input {
			for _, s := range input {
				for bit := mask; bit > 0; bit >>= 1 {
					b, low, high := uint16(0), uint16(0), scale-table[context]
					if bit&s != 0 {
						b, low, high = 1, high, scale
					}

					current[index], index = Symbol{Scale: scale, Low: low, High: high}, index+1
					if index == BUFFER_SIZE {
						out <- current
						next := offset + BUFFER_SIZE
						current, offset, index = buffer[offset:next], next&BUFFER_POOL_SIZE_MASK, 0
					}

					if b == 0 {
						table[context] -= table[context] >> filterShift
					} else {
						table[context] += (scale - table[context]) >> filterShift
					}

					context = b | (context << 1)
				}
			}
		}

		out <- current[:index]
		close(out)
	}()

	return Model{Input: out}
}

func (coder Coder16) FilteredAdaptiveCoder(newCDF func(size int) *CDF16) Model {
	out := make(chan []Symbol, BUFFER_CHAN_SIZE)

	go func() {
		cdf := newCDF(int(coder.Alphabit))
		buffer := [BUFFER_POOL_SIZE]Symbol{}

		current, offset, index := buffer[0:BUFFER_SIZE], BUFFER_SIZE, 0
		for input := range coder.Input {
			for _, s := range input {
				current[index], index = Symbol{Low: cdf.CDF[s], High: cdf.CDF[s+1]}, index+1
				if index == BUFFER_SIZE {
					out <- current
					next := offset + BUFFER_SIZE
					current, offset, index = buffer[offset:next], next&BUFFER_POOL_SIZE_MASK, 0
				}

				cdf.Update(int(s))
			}
		}

		out <- current[:index]
		close(out)
	}()

	return Model{Input: out, Fixed: CDF16Fixed}
}

func (coder Coder16) FilteredAdaptivePredictiveCoder(newCDF func(size int) *ContextCDF16) Model {
	out := make(chan []Symbol, BUFFER_CHAN_SIZE)

	go func() {
		cdf := newCDF(int(coder.Alphabit))
		buffer := [BUFFER_POOL_SIZE]Symbol{}

		current, offset, index := buffer[0:BUFFER_SIZE], BUFFER_SIZE, 0
		for input := range coder.Input {
			for _, s := range input {
				model := cdf.Model()
				current[index], index = Symbol{Low: model[s], High: model[s+1]}, index+1
				if index == BUFFER_SIZE {
					out <- current
					next := offset + BUFFER_SIZE
					current, offset, index = buffer[offset:next], next&BUFFER_POOL_SIZE_MASK, 0
				}

				cdf.Update(s)
			}
		}

		out <- current[:index]
		close(out)
	}()

	return Model{Fixed: CDF16Fixed, Input: out}
}

func (decoder Coder16) AdaptiveDecoder() Model {
	table, scale := make([]uint16, decoder.Alphabit), uint16(0)
	for i, _ := range table {
		table[i] = 1
		scale += 1
	}

	lookup := func(code uint16) Symbol {
		low, high, done := uint16(0), uint16(0), false
		for s, count := range table {
			if high += count; code < high {
				low, done, scale, table[s] = high-count, decoder.Output(uint16(s)), scale+1, table[s]+1
				break
			}
		}

		if done {
			return Symbol{}
		} else {
			if scale > MAX_SCALE16 {
				scale = 0
				for i, count := range table {
					if count >>= 1; count == 0 {
						table[i], scale = 1, scale+1
					} else {
						table[i], scale = count, scale+count
					}
				}
			}

			return Symbol{Scale: scale, Low: low, High: high}
		}
	}

	return Model{Scale: uint32(scale), Output: lookup}
}

func (decoder Coder16) AdaptivePredictiveDecoder() Model {
	table, scale, context := make([][]uint16, decoder.Alphabit), make([]uint16, decoder.Alphabit), uint16(0)
	for i, _ := range table {
		table[i] = make([]uint16, decoder.Alphabit)
		scale[i] = decoder.Alphabit
		for j, _ := range table[i] {
			table[i][j] = 1
		}
	}

	lookup := func(code uint16) Symbol {
		low, high, next, done := uint16(0), uint16(0), uint16(0), false
		for s, count := range table[context] {
			if high += count; code < high {
				next = uint16(s)
				low, done, scale[context], table[context][s] = high-count, decoder.Output(next), scale[context]+1, table[context][s]+1
				break
			}
		}

		if done {
			return Symbol{}
		} else {
			if scale[context] > MAX_SCALE16 {
				scale[context] = 0
				for i, count := range table[context] {
					if count >>= 1; count == 0 {
						table[context][i], scale[context] = 1, scale[context]+1
					} else {
						table[context][i], scale[context] = count, scale[context]+count
					}
				}
			}

			context = next
			return Symbol{Scale: scale[context], Low: low, High: high}
		}
	}

	return Model{Scale: uint32(decoder.Alphabit), Output: lookup}
}

func (decoder Coder16) AdaptiveBitDecoder() Model {
	table := [2]uint16{}
	table[0] = 1
	table[1] = 1

	highest := uint32(0)
	for a := decoder.Alphabit - 1; a > 0; a >>= 1 {
		highest++
	}
	mask := uint16(1) << (highest - 1)
	bit, bits := mask, uint16(0)

	lookup := func(code uint16) Symbol {
		scale, low, high, b := table[0]+table[1], uint16(0), uint16(0), uint16(0)
		if code < table[0] {
			high, bit = table[0], bit>>1
		} else {
			low, high, bits, bit, b = table[0], scale, bits|bit, bit>>1, 1
		}

		table[b]++
		if scale >= MAX_SCALE16 {
			table[0] >>= 1
			table[1] >>= 1
			if table[0] == 0 {
				table[0] = 1
			}
			if table[1] == 0 {
				table[1] = 1
			}
		}

		if bit == 0 {
			if decoder.Output(bits) {
				return Symbol{}
			}
			bits, bit = 0, mask
		}

		return Symbol{Scale: table[0] + table[1], Low: low, High: high}
	}

	return Model{Scale: uint32(2), Output: lookup}
}

func (decoder Coder16) AdaptivePredictiveBitDecoder() Model {
	table, context := make([][2]uint16, 65536), uint16(0)
	for i, _ := range table {
		table[i][0] = 1
		table[i][1] = 1
	}

	highest := uint32(0)
	for a := decoder.Alphabit - 1; a > 0; a >>= 1 {
		highest++
	}
	mask := uint16(1) << (highest - 1)
	bit, bits := mask, uint16(0)

	lookup := func(code uint16) Symbol {
		scale, low, high, b := table[context][0]+table[context][1], uint16(0), uint16(0), uint16(0)
		if code < table[context][0] {
			high, bit = table[context][0], bit>>1
		} else {
			low, high, bits, bit, b = table[context][0], scale, bits|bit, bit>>1, 1
		}

		table[context][b]++
		if scale >= MAX_SCALE16 {
			table[context][0] >>= 1
			table[context][1] >>= 1
			if table[context][0] == 0 {
				table[context][0] = 1
			}
			if table[context][1] == 0 {
				table[context][1] = 1
			}
		}
		context = b | (context << 1)

		if bit == 0 {
			if decoder.Output(bits) {
				return Symbol{}
			}
			bits, bit = 0, mask
		}

		return Symbol{Scale: table[context][0] + table[context][1], Low: low, High: high}
	}

	return Model{Scale: uint32(2), Output: lookup}
}

func (decoder Coder16) FilteredAdaptiveBitDecoder() Model {
	const scale = uint16(filterScale)
	p1 := scale / 2

	highest := uint32(0)
	for a := decoder.Alphabit - 1; a > 0; a >>= 1 {
		highest++
	}
	mask := uint16(1) << (highest - 1)
	bit, bits := mask, uint16(0)

	lookup := func(code uint16) Symbol {
		low, high, b := uint16(0), uint16(0), uint16(0)
		if p0 := scale - p1; code < p0 {
			high, bit = p0, bit>>1
		} else {
			low, high, bits, bit, b = p0, scale, bits|bit, bit>>1, 1
		}

		if b == 0 {
			p1 -= p1 >> filterShift
		} else {
			p1 += (scale - p1) >> filterShift
		}

		if bit == 0 {
			if decoder.Output(bits) {
				return Symbol{}
			}
			bits, bit = 0, mask
		}

		return Symbol{Scale: scale, Low: low, High: high}
	}

	return Model{Scale: uint32(scale), Output: lookup}
}

func (decoder Coder16) FilteredAdaptivePredictiveBitDecoder() Model {
	const scale = uint16(filterScale)
	table, context := make([]uint16, 65536), uint16(0)
	for i, _ := range table {
		table[i] = scale / 2
	}

	highest := uint32(0)
	for a := decoder.Alphabit - 1; a > 0; a >>= 1 {
		highest++
	}
	mask := uint16(1) << (highest - 1)
	bit, bits := mask, uint16(0)

	lookup := func(code uint16) Symbol {
		low, high, b := uint16(0), uint16(0), uint16(0)
		if p0 := scale - table[context]; code < p0 {
			high, bit = p0, bit>>1
		} else {
			low, high, bits, bit, b = p0, scale, bits|bit, bit>>1, 1
		}

		if b == 0 {
			table[context] -= table[context] >> filterShift
		} else {
			table[context] += (scale - table[context]) >> filterShift
		}
		context = b | (context << 1)

		if bit == 0 {
			if decoder.Output(bits) {
				return Symbol{}
			}
			bits, bit = 0, mask
		}

		return Symbol{Scale: scale, Low: low, High: high}
	}

	return Model{Scale: uint32(scale), Output: lookup}
}

func (decoder Coder16) FilteredAdaptiveDecoder(newCDF func(size int) *CDF16) Model {
	cdf := newCDF(int(decoder.Alphabit))

	lookup := func(code uint16) Symbol {
		low, high, done := uint16(0), uint16(0), false
		for s := 1; s < len(cdf.CDF); s++ {
			if code < cdf.CDF[s] {
				symbol := s - 1
				low, high, done = cdf.CDF[s-1], cdf.CDF[s], decoder.Output(uint16(symbol))

				cdf.Update(symbol)
				break
			}
		}

		if done {
			return Symbol{}
		}

		return Symbol{Low: low, High: high}
	}

	return Model{Fixed: CDF16Fixed, Output: lookup}
}

func (decoder Coder16) FilteredAdaptivePredictiveDecoder(newCDF func(size int) *ContextCDF16) Model {
	cdf := newCDF(int(decoder.Alphabit))

	lookup := func(code uint16) Symbol {
		low, high, done := uint16(0), uint16(0), false
		model := cdf.Model()
		for s := 1; s < len(model); s++ {
			if code < model[s] {
				symbol := uint16(s - 1)
				low, high, done = model[s-1], model[s], decoder.Output(symbol)

				cdf.Update(symbol)
				break
			}
		}

		if done {
			return Symbol{}
		}

		return Symbol{Low: low, High: high}
	}

	return Model{Fixed: CDF16Fixed, Output: lookup}
}

func (coder Coder16) AdaptiveCoder32() Model32 {
	out := make(chan []Symbol32, BUFFER_CHAN_SIZE)

	go func() {
		table, scale, buffer := make([]uint32, coder.Alphabit), uint32(coder.Alphabit), [BUFFER_POOL_SIZE]Symbol32{}
		for i, _ := range table {
			table[i] = 1
		}

		current, offset, index := buffer[0:BUFFER_SIZE], BUFFER_SIZE, 0
		for input := range coder.Input {
			for _, s := range input {
				low := uint32(0)
				for _, count := range table[:s] {
					low += count
				}

				current[index], index = Symbol32{Scale: scale, Low: low, High: low + table[s]}, index+1
				if index == BUFFER_SIZE {
					out <- current
					next := offset + BUFFER_SIZE
					current, offset, index = buffer[offset:next], next&BUFFER_POOL_SIZE_MASK, 0
				}

				scale++
				table[s]++
				if scale > MAX_SCALE32 {
					scale = 0
					for i, count := range table {
						if count >>= 1; count == 0 {
							table[i], scale = 1, scale+1
						} else {
							table[i], scale = count, scale+count
						}
					}
				}
			}
		}

		out <- current[:index]
		close(out)
	}()

	return Model32{Input: out}
}

func (coder Coder16) AdaptivePredictiveCoder32() Model32 {
	out := make(chan []Symbol32, BUFFER_CHAN_SIZE)

	go func() {
		table, scale, context, buffer := make([][]uint32, coder.Alphabit), make([]uint32, coder.Alphabit), uint16(0), [BUFFER_POOL_SIZE]Symbol32{}
		for i, _ := range table {
			table[i] = make([]uint32, coder.Alphabit)
			scale[i] = uint32(coder.Alphabit)
			for j, _ := range table[i] {
				table[i][j] = 1
			}
		}

		current, offset, index := buffer[0:BUFFER_SIZE], BUFFER_SIZE, 0
		for input := range coder.Input {
			for _, s := range input {
				low := uint32(0)
				for _, count := range table[context][:s] {
					low += count
				}

				current[index], index = Symbol32{Scale: scale[context], Low: low, High: low + table[context][s]}, index+1
				if index == BUFFER_SIZE {
					out <- current
					next := offset + BUFFER_SIZE
					current, offset, index = buffer[offset:next], next&BUFFER_POOL_SIZE_MASK, 0
				}

				scale[context]++
				table[context][s]++
				if scale[context] > MAX_SCALE32 {
					scale[context] = 0
					for i, count := range table[context] {
						if count >>= 1; count == 0 {
							table[context][i], scale[context] = 1, scale[context]+1
						} else {
							table[context][i], scale[context] = count, scale[context]+count
						}
					}
				}
				context = s
			}
		}

		out <- current[:index]
		close(out)
	}()

	return Model32{Input: out}
}

func (decoder Coder16) AdaptiveDecoder32() Model32 {
	table, scale := make([]uint32, decoder.Alphabit), uint32(decoder.Alphabit)
	for i, _ := range table {
		table[i] = 1
	}

	lookup := func(code uint32) Symbol32 {
		low, high, done := uint32(0), uint32(0), false
		for s, count := range table {
			if high += count; code < high {
				low, done, scale, table[s] = high-count, decoder.Output(uint16(s)), scale+1, table[s]+1
				break
			}
		}

		if done {
			return Symbol32{}
		} else {
			if scale > MAX_SCALE32 {
				scale = 0
				for i, count := range table {
					if count >>= 1; count == 0 {
						table[i], scale = 1, scale+1
					} else {
						table[i], scale = count, scale+count
					}
				}
			}

			return Symbol32{Scale: scale, Low: low, High: high}
		}
	}

	return Model32{Scale: uint64(decoder.Alphabit), Output: lookup}
}

func (decoder Coder16) AdaptivePredictiveDecoder32() Model32 {
	table, scale, context := make([][]uint32, decoder.Alphabit), make([]uint32, decoder.Alphabit), uint16(0)
	for i, _ := range table {
		table[i] = make([]uint32, decoder.Alphabit)
		scale[i] = uint32(decoder.Alphabit)
		for j, _ := range table[i] {
			table[i][j] = 1
		}
	}

	lookup := func(code uint32) Symbol32 {
		low, high, next, done := uint32(0), uint32(0), uint16(0), false
		for s, count := range table[context] {
			if high += count; code < high {
				next = uint16(s)
				low, done, scale[context], table[context][s] = high-count, decoder.Output(next), scale[context]+1, table[context][s]+1
				break
			}
		}

		if done {
			return Symbol32{}
		} else {
			if scale[context] > MAX_SCALE32 {
				scale[context] = 0
				for i, count := range table[context] {
					if count >>= 1; count == 0 {
						table[context][i], scale[context] = 1, scale[context]+1
					} else {
						table[context][i], scale[context] = count, scale[context]+count
					}
				}
			}

			context = next
			return Symbol32{Scale: scale[context], Low: low, High: high}
		}

	}

	return Model32{Scale: uint64(decoder.Alphabit), Output: lookup}
}
