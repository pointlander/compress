// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compress

func (coder Coder16) AdaptiveCoder1() Model {
	out := make(chan []Symbol, BUFFER_CHAN_SIZE)

	go func() {
		table, scale, buffer, history, head := make([]uint16, coder.Alphabit), uint16(coder.Alphabit), [BUFFER_POOL_SIZE]Symbol{}, [MAX_SCALE16 + 1]uint16{}, 0
		for i, _ := range table {
			table[i] = 1
		}
		for i, _ := range history {
			history[i] = 0xFFFF
		}

		current, offset, index := buffer[0:BUFFER_SIZE], BUFFER_SIZE, 0
		for input := range coder.Input {
			for _, s := range input {
				low := uint16(0)
				for _, count := range table[:s] {
					low += count
				}

				current[index], index = Symbol{Scale:scale, Low:low, High:low + table[s]}, index + 1
				if index == BUFFER_SIZE {
					out <- current
					next := offset + BUFFER_SIZE
					current, offset, index = buffer[offset:next], next & BUFFER_POOL_SIZE_MASK, 0
				}

				if t := history[head]; t != 0xFFFF {
					if t != s {
						history[head], table[s], table[t] = s, table[s] + 1, table[t] - 1
					}
				} else if scale < MAX_SCALE16 {
					history[head], table[s], scale = s, table[s] + 1, scale + 1
				}
				head = (head + 1) & MAX_SCALE16
			}
		}

		out <- current[:index]
		close(out)
	}()

	return Model{Input: out}
}

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

				current[index], index = Symbol{Scale:scale, Low:low, High:low + table[s]}, index + 1
				if index == BUFFER_SIZE {
					out <- current
					next := offset + BUFFER_SIZE
					current, offset, index = buffer[offset:next], next & BUFFER_POOL_SIZE_MASK, 0
				}

				scale++
				table[s]++
				if scale > MAX_SCALE16 {
					scale = 0
					for i, count := range table {
						if count >>= 1; count == 0 {
							table[i], scale = 1, scale + 1
						} else {
							table[i], scale = count, scale + count
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

				current[index], index = Symbol{Scale:scale[context], Low:low, High:low + table[context][s]}, index + 1
				if index == BUFFER_SIZE {
					out <- current
					next := offset + BUFFER_SIZE
					current, offset, index = buffer[offset:next], next & BUFFER_POOL_SIZE_MASK, 0
				}

				scale[context]++
				table[context][s]++
				if scale[context] > MAX_SCALE16 {
					scale[context] = 0
					for i, count := range table[context] {
						if count >>= 1; count == 0 {
							table[context][i], scale[context] = 1, scale[context] + 1
						} else {
							table[context][i], scale[context] = count, scale[context] + count
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

func (decoder Coder16) AdaptiveDecoder1() Model {
	table, scale, history, head := make([]uint16, decoder.Alphabit), uint16(decoder.Alphabit), [MAX_SCALE16 + 1]uint16{}, 0
	for i, _ := range table {
		table[i] = 1
	}
	for i, _ := range history {
		history[i] = 0xFFFF
	}

	lookup := func(code uint16) Symbol {
		low, high, s := uint16(0), uint16(0), uint16(0)
		for symbol, count := range table {
			if high += count; code < high {
				low, s = high - count, uint16(symbol)
				break
			}
		}

		if !decoder.Output(s) {
			if t := history[head]; t != 0xFFFF {
				if t != s {
					history[head], table[s], table[t] = s, table[s] + 1, table[t] - 1
				}
			} else if scale < MAX_SCALE16 {
				history[head], table[s], scale = s, table[s] + 1, scale + 1
			}
			head = (head + 1) & MAX_SCALE16

			return Symbol{Scale:scale, Low:low, High:high}
		}
		return Symbol{}
	}

	return Model{Scale:uint32(decoder.Alphabit), Output:lookup}
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
				low, done, scale, table[s] = high - count, decoder.Output(uint16(s)), scale + 1, table[s] + 1
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
						table[i], scale = 1, scale + 1
					} else {
						table[i], scale = count, scale + count
					}
				}
			}

			return Symbol{Scale:scale, Low:low, High:high}
		}
		return Symbol{}
	}

	return Model{Scale:uint32(scale), Output:lookup}
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
				low, done, scale[context], table[context][s] = high - count, decoder.Output(next), scale[context] + 1, table[context][s] + 1
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
						table[context][i], scale[context] = 1, scale[context] + 1
					} else {
						table[context][i], scale[context] = count, scale[context] + count
					}
				}
			}

			context = next
			return Symbol{Scale:scale[context], Low:low, High:high}
		}
		return Symbol{}
	}

	return Model{Scale:uint32(decoder.Alphabit), Output:lookup}
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

				current[index], index = Symbol32{Scale:scale, Low:low, High:low + table[s]}, index + 1
				if index == BUFFER_SIZE {
					out <- current
					next := offset + BUFFER_SIZE
					current, offset, index = buffer[offset:next], next & BUFFER_POOL_SIZE_MASK, 0
				}

				scale++
				table[s]++
				if scale > MAX_SCALE32 {
					scale = 0
					for i, count := range table {
						if count >>= 1; count == 0 {
							table[i], scale = 1, scale + 1
						} else {
							table[i], scale = count, scale + count
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

				current[index], index = Symbol32{Scale:scale[context], Low:low, High:low + table[context][s]}, index + 1
				if index == BUFFER_SIZE {
					out <- current
					next := offset + BUFFER_SIZE
					current, offset, index = buffer[offset:next], next & BUFFER_POOL_SIZE_MASK, 0
				}

				scale[context]++
				table[context][s]++
				if scale[context] > MAX_SCALE32 {
					scale[context] = 0
					for i, count := range table[context] {
						if count >>= 1; count == 0 {
							table[context][i], scale[context] = 1, scale[context] + 1
						} else {
							table[context][i], scale[context] = count, scale[context] + count
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
				low, done, scale, table[s] = high - count, decoder.Output(uint16(s)), scale + 1, table[s] + 1
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
						table[i], scale = 1, scale + 1
					} else {
						table[i], scale = count, scale + count
					}
				}
			}

			return Symbol32{Scale:scale, Low:low, High:high}
		}
		return Symbol32{}
	}

	return Model32{Scale:uint64(decoder.Alphabit), Output:lookup}
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
				low, done, scale[context], table[context][s] = high - count, decoder.Output(next), scale[context] + 1, table[context][s] + 1
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
						table[context][i], scale[context] = 1, scale[context] + 1
					} else {
						table[context][i], scale[context] = count, scale[context] + count
					}
				}
			}

			context = next
			return Symbol32{Scale:scale[context], Low:low, High:high}
		}

		return Symbol32{}
	}

	return Model32{Scale:uint64(decoder.Alphabit), Output:lookup}
}
