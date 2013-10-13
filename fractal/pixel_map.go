// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fractal

type pixelMap []int

func newPixelMap(panelSize int) []pixelMap {
	forms := [...]struct {
		a, b, c, d    int
		contractivity float64
	}{
		{a: 1, b: 0, c: 0, d: 1, contractivity: 0.5},
		{a: -1, b: 0, c: 0, d: 1, contractivity: 0.5},
		{a: 1, b: 0, c: 0, d: -1, contractivity: 0.5},
		{a: -1, b: 0, c: 0, d: -1, contractivity: 0.5},
		{a: 0, b: 1, c: 1, d: 0, contractivity: 0.5},
		{a: 0, b: -1, c: 1, d: 0, contractivity: 0.5},
		{a: 0, b: 1, c: -1, d: 0, contractivity: 0.5},
		{a: 0, b: -1, c: -1, d: 0, contractivity: 0.5}}
	maps, size := make([]pixelMap, len(forms)), panelSize*panelSize
	for form := range forms {
		pmap := make(pixelMap, size)
		for x := 0; x < panelSize; x++ {
			for y := 0; y < panelSize; y++ {
				index, i, j := x+y*panelSize, 0, 0

				switch true {
				case forms[form].a == 1:
					i = x
				case forms[form].a == -1:
					i = panelSize - 1 - x
				case forms[form].b == 1:
					i = y
				case forms[form].b == -1:
					i = panelSize - 1 - y
				}

				switch true {
				case forms[form].c == 1:
					j = x
				case forms[form].c == -1:
					j = panelSize - 1 - x
				case forms[form].d == 1:
					j = y
				case forms[form].d == -1:
					j = panelSize - 1 - y
				}

				pmap[index] = i + j*panelSize
			}
		}
		maps[form] = pmap
	}
	return maps
}
