// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compress

type Lyndon struct {
        Words [][]uint8
}

func (l *Lyndon) Factor(s []uint8) {
        k, m, n, words, max := 0, 1, len(s), l.Words[:0], len(s)
	if max > 256 {
		max = 256 + (max - 256) / 2
	}
	if cap(words) < max {
		words = make([][]uint8, 0, max)
	}

        for {
		switch sk, sm := s[k], s[m]; true {
		case sk < sm:
			k, m = 0, m + 1
			if m < n {
				continue
			}
		case sk == sm:
			k, m = k + 1, m + 1
			if m < n {
				continue
			}
			fallthrough
		case sk > sm:
			split := m - k
			k, m, s, words = 0, 1, s[split:], append(words, s[:split])
			n = len(s)
			if n > 1 {
				continue
			}
                }
		break
        }
	l.Words = append(words, s)
}
