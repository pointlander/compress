// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fractal

import (
	"bytes"
	"github.com/nfnt/resize"
	"image"
	"image/color"
	"io"
	"math"
)

const (
	GAMMA = 0.75
	DECODE_ITERATIONS = 16
)

type imagePanel struct {
	x, y, mean int
	pixels     []uint16
}

type image8 struct {
	xPanels, yPanels, panelSize int
	bounds                      image.Rectangle
	pixels                      []uint8
	panels                      []imagePanel
}

func splitImage(input image.Image) (r, g, b image.Image) {
	width, height := input.Bounds().Max.X, input.Bounds().Max.Y
	size := width * height
	rpixels, gpixels, bpixels :=
		make([]uint8, size),
		make([]uint8, size),
		make([]uint8, size)
	for y := 0; y < height; y++ {
		offset := y * width
		for x := 0; x < width; x++ {
			r, g, b, _ := input.At(x, y).RGBA()
			i := x + offset
			rpixels[i], gpixels[i], bpixels[i] =
				uint8(r >> 8),
				uint8(g >> 8),
				uint8(b >> 8)
		}
	}
	r = &image8{
		bounds: image.Rectangle{
			Max: image.Point{
				X: width,
				Y: height}},
		pixels: rpixels}
	g = &image8{
		bounds: image.Rectangle{
			Max: image.Point{
				X: width,
				Y: height}},
		pixels: gpixels}
	b = &image8{
		bounds: image.Rectangle{
			Max: image.Point{
				X: width,
				Y: height}},
		pixels: bpixels}
	return
}

func newImage(input image.Image, scale, panelSize int, gamma float64) *image8 {
	width, height := input.Bounds().Max.X, input.Bounds().Max.Y

	if scale > 1 {
		width, height = width/scale, height/scale
		input = resize.Resize(uint(width), uint(height), input, resize.NearestNeighbor)
	}

	for (width%panelSize) != 0 || (width%2) != 0 {
		width--
	}
	for (height%panelSize) != 0 || (height%2) != 0 {
		height--
	}

	pixels := make([]uint8, width*height)
	for y := 0; y < height; y++ {
		offset := y * width
		for x := 0; x < width; x++ {
			r, g, b, _ := input.At(x, y).RGBA()
			pixels[offset+x] = uint8(uint32((float64(r+g+b)*gamma)/3) >> 8)
		}
	}

	xPanels, yPanels := width/panelSize, height/panelSize
	panels, size := make([]imagePanel, xPanels*yPanels), panelSize*panelSize
	for i := range panels {
		panels[i].pixels = make([]uint16, size)
	}

	paneled := &image8{
		xPanels:   xPanels,
		yPanels:   yPanels,
		panelSize: panelSize,
		bounds: image.Rectangle{
			Max: image.Point{
				X: width,
				Y: height}},
		pixels: pixels,
		panels: panels}
	paneled.updatePanels()
	return paneled
}

func (i *image8) updatePanels() {
	width, height := i.Bounds().Max.X, i.Bounds().Max.Y
	pixels, panels, panelSize := i.pixels, i.panels, i.panelSize
	p, size := 0, panelSize*panelSize

	for y := 0; y < height; y += panelSize {
		for x := 0; x < width; x += panelSize {
			pix, q, sum := panels[p].pixels, 0, 0
			for j := 0; j < panelSize; j++ {
				for i := 0; i < panelSize; i++ {
					pix[q] = uint16(pixels[(y+j)*width+x+i])
					sum, q = sum+int(pix[q]), q+1
				}
			}

			panels[p] = imagePanel{
				x:      x,
				y:      y,
				mean:   sum / size,
				pixels: pix}
			p++
		}
	}
}

func (i *image8) ColorModel() color.Model {
	return color.GrayModel
}

func (i *image8) Bounds() image.Rectangle {
	return i.bounds
}

func (i *image8) At(x, y int) color.Color {
	return color.Gray{Y: i.pixels[y*i.bounds.Max.X+x]}
}

func (i *image8) Set(x, y int, c color.Color) {
	gray, _, _, _ := c.RGBA()
	i.pixels[y*i.bounds.Max.X+x] = uint8(gray)
}

func FractalCoder(in image.Image, panelSize int, out io.Writer) {
	destination := newImage(in, 1, panelSize, 1)
	reference := newImage(in, 2, panelSize, GAMMA)
	maps := newPixelMap(panelSize)

	buffer, count := &bytes.Buffer{}, 0
	write16 := func(s uint16) {
		b := [...]byte{
			byte(s >> 8),
			byte(s & 0xFF)}
		buffer.Write(b[:])
	}

	for _, dPanel := range destination.panels {
		var best imagePanel
		bestError, bestForm, bestBeta := uint64(math.MaxUint64), 0, 0
		for _, rPanel := range reference.panels {
			beta := dPanel.mean - rPanel.mean
		search:
			for f, pmap := range maps {
				error := uint64(0)
				for i, j := range pmap {
					delta := int(dPanel.pixels[i]) -
						int(rPanel.pixels[j]) -
						beta
					error += uint64(delta * delta)
					if error >= bestError {
						continue search
					}
				}
				if error < bestError {
					best = rPanel
					bestBeta = beta
					bestForm = f
					bestError = error
				}
			}
			if bestError == 0 {
				break
			}
		}
		write16(uint16(bestForm))
		write16(uint16(best.x))
		write16(uint16(best.y))
		write16(uint16(bestBeta))
		count++
	}

	write32 := func(i uint32) {
		b := [...]byte{
			byte(i >> 24),
			byte((i >> 16) & 0xFF),
			byte((i >> 8) & 0xFF),
			byte(i & 0xFF)}
		out.Write(b[:])
	}
	write32(uint32(destination.xPanels))
	write32(uint32(destination.yPanels))
	write32(uint32(destination.panelSize))
	write32(uint32(count))
	out.Write(buffer.Bytes())
}

func FractalDecoder(in io.Reader, _panelSize int) image.Image {
	read32 := func() uint32 {
		var p [4]byte
		in.Read(p[:])
		return (uint32(p[0]) << 24) |
			(uint32(p[1]) << 16) |
			(uint32(p[2]) << 8) |
			uint32(p[3])
	}
	xPanels := read32()
	yPanels := read32()
	panelSize := read32()
	count := read32()

	read16 := func() uint16 {
		var p [2]byte
		in.Read(p[:])
		return (uint16(p[0]) << 8) |
			uint16(p[1])
	}
	codes := make([]struct{ form, x, y, beta uint16 }, count)
	for i := range codes {
		codes[i].form = read16()
		codes[i].x = read16()
		codes[i].y = read16()
		codes[i].beta = read16()
	}

	width, height := xPanels*uint32(_panelSize), yPanels*uint32(_panelSize)
	pixels := make([]uint8, width*height)
	for y := uint32(0); y < height; y++ {
		offset := y * width
		for x := uint32(0); x < width; x++ {
			pixels[offset+x] = uint8(0x80)
		}
	}

	panels, size := make([]imagePanel, xPanels*yPanels), _panelSize*_panelSize
	for i := range panels {
		panels[i].pixels = make([]uint16, size)
	}

	destination := &image8{
		xPanels:   int(xPanels),
		yPanels:   int(yPanels),
		panelSize: _panelSize,
		bounds: image.Rectangle{
			Max: image.Point{
				X: int(width),
				Y: int(height)}},
		pixels: pixels,
		panels: panels}
	destination.updatePanels()

	newReference := func() *image8 {
		width, height := destination.Bounds().Max.X, destination.Bounds().Max.Y
		width, height = width/2, height/2
		reference := resize.Resize(uint(width), uint(height), destination, resize.NearestNeighbor)

		pixels := make([]uint8, width*height)
		for y := 0; y < height; y++ {
			offset := y * width
			for x := 0; x < width; x++ {
				r, _, _, _ := reference.At(x, y).RGBA()
				pixels[offset+x] = uint8(uint32(float64(r)*GAMMA) >> 8)
			}
		}

		xPanels, yPanels := width/_panelSize, height/_panelSize
		panels, size := make([]imagePanel, xPanels*yPanels), _panelSize*_panelSize
		for i := range panels {
			panels[i].pixels = make([]uint16, size)
		}

		paneled := &image8{
			xPanels:   xPanels,
			yPanels:   yPanels,
			panelSize: _panelSize,
			bounds: image.Rectangle{
				Max: image.Point{
					X: width,
					Y: height}},
			pixels: pixels,
			panels: panels}
		paneled.updatePanels()
		return paneled
	}

	maps := newPixelMap(_panelSize)
	for i := 0; i < DECODE_ITERATIONS; i++ {
		reference := newReference()

		for j, d := range panels {
			code := codes[j]
			//x, y := int(uint64(_panelSize)*uint64(code.x)/(2*uint64(panelSize))),
			//	int(uint64(_panelSize)*uint64(code.y)/(2*uint64(panelSize)))
			x, y := int(uint32(code.x)/panelSize),
				int(uint32(code.y)/panelSize)
			if x >= reference.xPanels {
				x = reference.xPanels - 1
			}
			if y >= reference.yPanels {
				y = reference.yPanels - 1
			}
			r := reference.panels[x+y*reference.xPanels]
			pmap, f := maps[code.form], 0

			for y := 0; y < _panelSize; y++ {
				for x := 0; x < _panelSize; x++ {
					z, e := int(r.pixels[pmap[f]])+int(int16(code.beta)),
						d.x+x+int(width)*(d.y+y)
					if z < 0 {
						pixels[e] = uint8(0)
					} else if z > 255 {
						pixels[e] = uint8(0xFF)
					} else {
						pixels[e] = uint8(z)
					}
					f++
				}
			}
		}
	}

	return destination
}
