// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fractal

import (
	"image"
	"image/color"
	"math"
)

const (
	N int = 8
	QUALITY int = 0
)

var (
	zigZag8 = [64]struct {row, col int} {
		{0, 0},
		{0, 1}, {1, 0},
		{2, 0}, {1, 1}, {0, 2},
		{0, 3}, {1, 2}, {2, 1}, {3, 0},
		{4, 0}, {3, 1}, {2, 2}, {1, 3}, {0, 4},
		{0, 5}, {1, 4}, {2, 3}, {3, 2}, {4, 1}, {5, 0},
		{6, 0}, {5, 1}, {4, 2}, {3, 3}, {2, 4}, {1, 5}, {0, 6},
		{0, 7}, {1, 6}, {2, 5}, {3, 4}, {4, 3}, {5, 2}, {6, 1}, {7, 0},
		{7, 1}, {6, 2}, {5, 3}, {4, 4}, {3, 5}, {2, 6}, {1, 7},
		{2, 7}, {3, 6}, {4, 5}, {5, 4}, {6, 3}, {7, 2},
		{7, 3}, {6, 4}, {5, 5}, {4, 6}, {3, 7},
		{4, 7}, {5, 6}, {6, 5}, {7, 4},
		{7, 5}, {6, 6}, {5, 7},
		{6, 7}, {7, 6},
		{7, 7},
	}
	quantum [N][N]int
	c, cT [N][N]float64
)

func init() {
	for i := 0; i < N; i++ {
		for j := 0; j < N; j++ {
			quantum[i][j] = (1 + ((1 + i + j) * QUALITY))
		}
	}

	for j := 0; j < N; j++ {
		c[0][j] = 1.0 / math.Sqrt(float64(N))
		cT[j][0] = c[0][j]
	}

	for i := 1; i < N; i++ {
		for j := 0; j < N; j++ {
			jj, ii := float64(j), float64(i)
			c[i][j] = math.Sqrt(2.0 / 8.0) * math.Cos(((2.0 * jj + 1.0) * ii * math.Pi) / (2.0 * 8.0))
			cT[j][i] = c[i][j]
		}
	}
}

func round(x float64) float64 {
	if x < 0 {
		return math.Ceil(x - 0.5)
	}
	return math.Floor(x + 0.5)
}

func ForwardDCT(in *[N][N]uint8, out *[N][N]int) {
	var x [N][N]float64
	for i := 0; i < N; i++ {
		for j := 0; j < N; j++ {
			for k := 0; k < N; k++ {
				x[i][j] += float64(int(in[i][k]) - 128) * cT[k][j]
			}
		}
	}

	for i := 0; i < N; i++ {
		for j := 0; j < N; j++ {
			y := 0.0
			for k := 0; k < N; k++ {
				y += c[i][k] * x[k][j]
			}
			out[i][j] = int(round(y))
		}
	}
}

func InverseDCT(in, out *[N][N]int) {
	var x [N][N]float64
	for i := 0; i < N; i++ {
		for j := 0; j < N; j++ {
			for k := 0; k < N; k++ {
				x[i][j] += float64(in[i][k]) * c[k][j]
			}
		}
	}

	for i := 0; i < N; i++ {
		for j := 0; j < N; j++ {
			y := 0.0
			for k := 0; k < N; k++ {
				y += cT[i][k] * x[k][j]
			}
			y += 128
			if y < 0 {
				out[i][j] = 0
			} else if y > 255 {
				out[i][j] = 255
			} else {
				out[i][j] = int(round(y))
			}
		}
	}
}

func DCTCoder(input image.Image) *image.Gray {
	in, dct, output := [8][8]uint8 {}, [8][8]int {}, image.NewGray(input.Bounds())
	width, height := input.Bounds().Max.X, input.Bounds().Max.Y
	for x := 0; x < width; x += 8 {
		for y := 0; y < height; y += 8 {
			for i := 0; i < 8; i++ {
				for j := 0; j < 8; j++ {
					c, _, _, _ := input.At(x+i, y+j).RGBA()
					in[i][j], dct[i][j] = uint8(c >> 8), 0
				}
			}
			ForwardDCT(&in, &dct)
			for i := 0; i < 8; i++ {
				for j := 0; j < 8; j++ {
					output.SetGray(x+i, y+j, color.Gray {uint8((dct[i][j]/16)+128)})
				}
			}
		}
	}
	return output
}

func DCTDecoder(input image.Image) *image.Gray {
	in, idct, output := [8][8]int {}, [8][8]int {}, image.NewGray(input.Bounds())
	width, height := input.Bounds().Max.X, input.Bounds().Max.Y
	for x := 0; x < width; x += 8 {
		for y := 0; y < height; y += 8 {
			for i := 0; i < 8; i++ {
				for j := 0; j < 8; j++ {
					r, _, _, _ := input.At(x+i, y+j).RGBA()
					in[i][j], idct[i][j] = 16 * (int(r >> 8) - 128), 0
				}
			}
			InverseDCT(&in, &idct)
			for i := 0; i < 8; i++ {
				for j := 0; j < 8; j++ {
					output.SetGray(x+i, y+j, color.Gray {uint8(idct[i][j])})
				}
			}
		}
	}
	return output
}

func DCTMap(input image.Image) *image.Gray {
	bounds := input.Bounds()
	output := image.NewGray(bounds)
	width, height := bounds.Max.X, bounds.Max.Y
	sx, sy := width/8, height/8
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			for i := 0; i < sx; i++ {
				for j := 0; j < sy; j++ {
					r, _, _, _ := input.At(x+i*8, y+j*8).RGBA()
					output.SetGray(i+x*sx, j+y*sx, color.Gray {uint8(r >> 8)})
				}
			}
		}
	}
	return output
}

func DCTIMap(input image.Image) *image.Gray {
	bounds := input.Bounds()
	output := image.NewGray(bounds)
	width, height := bounds.Max.X, bounds.Max.Y
	sx, sy := width/8, height/8
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			for i := 0; i < sx; i++ {
				for j := 0; j < sy; j++ {
					r, _, _, _ := input.At(i+x*sx, j+y*sx).RGBA()
					output.SetGray(x+i*8, y+j*8, color.Gray {uint8(r >> 8)})
				}
			}
		}
	}
	return output
}
