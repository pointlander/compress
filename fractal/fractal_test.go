// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fractal

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/kjk/lzma"
	"github.com/nfnt/resize"
	"github.com/pointlander/compress"
)

const testImage = "../bench/310px-Tesla_colorado_adjusted.jpg"

func TestDCT(t *testing.T) {
	in := [8][8]uint8{
		{255, 0, 255, 0, 255, 0, 255, 0},
		{0, 255, 0, 255, 0, 255, 0, 255},
		{255, 0, 255, 0, 255, 0, 255, 0},
		{0, 255, 0, 255, 0, 255, 0, 255},
		{255, 0, 255, 0, 255, 0, 255, 0},
		{0, 255, 0, 255, 0, 255, 0, 255},
		{255, 0, 255, 0, 255, 0, 255, 0},
		{0, 255, 0, 255, 0, 255, 0, 255},
	}
	dct, idct := [8][8]int{}, [8][8]int{}
	ForwardDCT(&in, &dct)
	InverseDCT(&dct, &idct)
	for i := 0; i < N; i++ {
		for j := 0; j < N; j++ {
			if int(in[i][j]) != idct[i][j] {
				t.Errorf("input does not equal idct(dct(input))")
			}
		}
	}
}

func TestImageDCT(t *testing.T) {
	file, err := os.Open(testImage)
	if err != nil {
		log.Fatal(err)
	}

	input, _, err := image.Decode(file)
	if err != nil {
		log.Fatal(err)
	}
	file.Close()

	input = Gray(input)
	output := DCTCoder(input)
	//output = DCTMap(output)
	//output = Paeth8(output)

	file, err = os.Create("tesla_dct.png")
	if err != nil {
		log.Fatal(err)
	}

	err = png.Encode(file, output)
	if err != nil {
		log.Fatal(err)
	}
	file.Close()

	data := make([]byte, len(output.Pix))
	copy(data, output.Pix)
	in, compressed := make(chan []byte, 1), &bytes.Buffer{}
	in <- data
	close(in)
	compress.BijectiveBurrowsWheelerCoder(in).MoveToFrontRunLengthCoder().AdaptiveCoder().Code(compressed)
	fmt.Printf("%.3f%% %7vb\n", 100*float64(compressed.Len())/float64(len(output.Pix)), compressed.Len())

	//output = IPaeth8(output)
	//idct := DCTIMap(output)
	idct := DCTDecoder(output)

	file, err = os.Create("tesla_idct.png")
	if err != nil {
		log.Fatal(err)
	}

	err = png.Encode(file, idct)
	if err != nil {
		log.Fatal(err)
	}
	file.Close()
}

func TestFractal(t *testing.T) {
	runtime.GOMAXPROCS(64)

	fcpBuffer := &bytes.Buffer{}
	file, err := os.Open(testImage)
	if err != nil {
		log.Fatal(err)
	}

	info, err := file.Stat()
	if err != nil {
		log.Fatal(err)
	}
	name := info.Name()
	name = name[:strings.Index(name, ".")]

	input, _, err := image.Decode(file)
	if err != nil {
		log.Fatal(err)
	}
	file.Close()

	width, height, scale := input.Bounds().Max.X, input.Bounds().Max.Y, 1
	width, height = width/scale, height/scale
	if width < height {
		height = width
	} else if height < width {
		width = height
	}
	input = resize.Resize(uint(width), uint(height), input, resize.NearestNeighbor)

	gray := Gray(input)
	FractalCoder(gray, 4, fcpBuffer)
	fcp := fcpBuffer.Bytes()
	fcpBufferCopy := bytes.NewBuffer(fcp)

	file, err = os.Create(name + ".png")
	if err != nil {
		log.Fatal(err)
	}

	err = png.Encode(file, input)
	if err != nil {
		log.Fatal(err)
	}
	file.Close()

	file, err = os.Create(name + ".fcp")
	if err != nil {
		log.Fatal(err)
	}

	file.Write(fcp)
	file.Close()

	decoded := FractalDecoder(fcpBuffer, 2)

	file, err = os.Create(name + "_decoded.png")
	if err != nil {
		log.Fatal(err)
	}

	err = png.Encode(file, decoded)
	if err != nil {
		log.Fatal(err)
	}
	file.Close()

	decoded = FractalDecoder(fcpBufferCopy, 8)

	file, err = os.Create(name + "_decodedx2.png")
	if err != nil {
		log.Fatal(err)
	}

	err = png.Encode(file, decoded)
	if err != nil {
		log.Fatal(err)
	}
	file.Close()

	{
		tileSize := 2
		r, g, b := splitImage(input)
		buffer := &bytes.Buffer{}
		FractalCoder(r, tileSize, buffer)
		FractalCoder(g, tileSize, buffer)
		FractalCoder(b, tileSize, buffer)
		data := make([]byte, len(buffer.Bytes()))
		copy(data, buffer.Bytes())
		in, output := make(chan []byte, 1), &bytes.Buffer{}
		in <- data
		close(in)
		compress.BijectiveBurrowsWheelerCoder(in).MoveToFrontRunLengthCoder().AdaptiveCoder().Code(output)
		fmt.Printf("%.3f%% %7vb\n", 100*float64(output.Len())/float64(buffer.Len()), output.Len())
		red := FractalDecoder(buffer, tileSize)
		green := FractalDecoder(buffer, tileSize)
		blue := FractalDecoder(buffer, tileSize)
		decoded := image.NewRGBA(input.Bounds())
		width, height := input.Bounds().Max.X, input.Bounds().Max.Y
		for x := 0; x < width; x++ {
			for y := 0; y < height; y++ {
				r, _, _, _ := red.At(x, y).RGBA()
				g, _, _, _ := green.At(x, y).RGBA()
				b, _, _, _ := blue.At(x, y).RGBA()
				decoded.Set(x, y, color.RGBA{
					R: uint8(r >> 8),
					G: uint8(g >> 8),
					B: uint8(b >> 8),
					A: 0xFF})
			}
		}

		file, err = os.Create(name + "_color.png")
		if err != nil {
			log.Fatal(err)
		}

		err = png.Encode(file, decoded)
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
	}

	fmt.Printf("%vb fcp file size\n", len(fcp))

	zpaq_test := func() (int, string) {
		zpaqFile := "fifo.zpaq"
		syscall.Mkfifo(zpaqFile, 0600)
		cmd := exec.Command("zp", "c3", zpaqFile, name+".fcp")
		cmd.Start()
		buffer, err := ioutil.ReadFile(zpaqFile)
		if err != nil {
			log.Fatal(err)
		}
		cmd.Wait()
		return len(buffer), "zp (command)"
	}

	compress_test := func() (int, string) {
		data := make([]byte, len(fcp))
		copy(data, fcp)
		in, buffer := make(chan []byte, 1), &bytes.Buffer{}
		in <- data
		close(in)
		compress.BijectiveBurrowsWheelerCoder(in).MoveToFrontRunLengthCoder().AdaptiveCoder().Code(buffer)
		return buffer.Len(), "github.com/pointlander/compress"
	}

	bzip2_test := func() (int, string) {
		cmd, buffer := exec.Command("bzip2", "--best"), &bytes.Buffer{}
		stdout, _ := cmd.StdoutPipe()
		stdin, _ := cmd.StdinPipe()
		cmd.Start()
		go func() {
			io.Copy(buffer, stdout)
		}()
		stdin.Write(fcp)
		stdin.Close()
		cmd.Wait()
		return buffer.Len(), "bzip2 (command)"
	}

	lzma_test := func() (int, string) {
		buffer := &bytes.Buffer{}
		writer := lzma.NewWriterLevel(buffer, lzma.BestCompression)
		writer.Write(fcp)
		writer.Close()
		return buffer.Len(), "github.com/kjk/lzma"
	}

	zlib_test := func() (int, string) {
		buffer := &bytes.Buffer{}
		writer, _ := zlib.NewWriterLevel(buffer, zlib.BestCompression)
		writer.Write(fcp)
		writer.Close()
		return buffer.Len(), "compress/zlib"
	}

	jpg_test := func() (int, string) {
		buffer := &bytes.Buffer{}
		jpeg.Encode(buffer, gray, nil)
		return buffer.Len(), "image/jpeg"
	}

	tests := []func() (int, string){zpaq_test, compress_test, bzip2_test, lzma_test, zlib_test, jpg_test}
	results := make([]struct {
		size int
		name string
	}, len(tests))
	for i, test := range tests {
		results[i].size, results[i].name = test()
		if i < 5 {
			fmt.Printf("%.3f%% %7vb %v\n", 100*float64(results[i].size)/float64(len(fcp)), results[i].size, results[i].name)
		}
	}

	fmt.Println("\n image compression comparisons")
	for _, result := range results {
		fmt.Printf("%.3f%% %7vb %v\n", 100*float64(result.size)/float64(width*height), result.size, result.name)
	}
}
