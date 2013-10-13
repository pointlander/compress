// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fractal

import (
	"image"
	"image/color"
)

func Gray(input image.Image) *image.Gray {
	bounds := input.Bounds()
	output := image.NewGray(bounds)
	width, height := bounds.Max.X, bounds.Max.Y
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			r, g, b, _ := input.At(x, y).RGBA()
			output.SetGray(x, y, color.Gray {uint8((r+g+b)/768)})
		}
	}
	return output
}
