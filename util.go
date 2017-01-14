// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compress

import "io"

func Mark1Compress16(input []byte, output io.Writer) {
	data, channel := make([]byte, len(input)), make(chan []byte, 1)
	copy(data, input)
	channel <- data
	close(channel)
	BijectiveBurrowsWheelerCoder(channel).MoveToFrontRunLengthCoder().AdaptiveCoder().Code(output)
}

func Mark1Decompress16(input io.Reader, output []byte) {
	channel := make(chan []byte, 1)
	channel <- output
	close(channel)
	BijectiveBurrowsWheelerDecoder(channel).MoveToFrontRunLengthDecoder().AdaptiveDecoder().Decode(input)
}
