// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"image"
	/*"image/color"*/
	_ "image/jpeg"
	"io/ioutil"
	"log"
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pointlander/compress"
)

var failed []string

type Symbols8 [256]struct {
	uint8
	uint64
}

func (s *Symbols8) Count(data []byte) {
	for _, d := range data {
		s[d].uint64++
	}
}

func (s *Symbols8) Sort() {
	for i, _ := range s {
		s[i].uint8 = uint8(i)
	}
	sort.Sort(s)
}

func (s *Symbols8) Len() int {
	return len(s)
}

func (s *Symbols8) Less(i, j int) bool {
	a, b := s[i].uint64, s[j].uint64
	if a > b {
		return true
	} else if a == b {
		return s[i].uint8 < s[j].uint8
	}
	return false
}

func (s *Symbols8) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func Entropy(input []byte) float64 {
	var histogram [256]uint64

	for _, v := range input {
		histogram[v]++
	}

	entropy, length := float64(0), float64(len(input))
	for _, v := range histogram {
		if v != 0 {
			entropy += float64(v) * math.Log2(float64(v)/length) / length
		}
	}
	return -entropy
}

type Coder16Configuration struct {
	Name       string
	Compress   func(coder *compress.Coder16, buffer *bytes.Buffer)
	Uncompress func(coder *compress.Coder16, buffer *bytes.Buffer)
}

var coder16Configurations = [...]Coder16Configuration{
	{
		Name: "adaptive coder 16",
		Compress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.AdaptiveCoder().Code(buffer)
		},
		Uncompress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.AdaptiveDecoder().Decode(buffer)
		},
	},
	{
		Name: "adaptive predictive coder 16",
		Compress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.AdaptivePredictiveCoder().Code(buffer)
		},
		Uncompress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.AdaptivePredictiveDecoder().Decode(buffer)
		},
	},
	{
		Name: "adaptive bit coder 16",
		Compress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.AdaptiveBitCoder().Code(buffer)
		},
		Uncompress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.AdaptiveBitDecoder().Decode(buffer)
		},
	},
	{
		Name: "adaptive predictive bit coder 16",
		Compress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.AdaptivePredictiveBitCoder().Code(buffer)
		},
		Uncompress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.AdaptivePredictiveBitDecoder().Decode(buffer)
		},
	},
	{
		Name: "filtered adaptive bit coder 16",
		Compress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.FilteredAdaptiveBitCoder().Code(buffer)
		},
		Uncompress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.FilteredAdaptiveBitDecoder().Decode(buffer)
		},
	},
	{
		Name: "filtered adaptive predictive bit coder 16",
		Compress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.FilteredAdaptivePredictiveBitCoder().Code(buffer)
		},
		Uncompress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.FilteredAdaptivePredictiveBitDecoder().Decode(buffer)
		},
	},
	{
		Name: "filtered adaptive coder 16",
		Compress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.FilteredAdaptiveCoder(compress.NewCDF16(0, true)).Code(buffer)
		},
		Uncompress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.FilteredAdaptiveDecoder(compress.NewCDF16(0, true)).Decode(buffer)
		},
	},
	{
		Name: "filtered adaptive predictive coder 16",
		Compress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.FilteredAdaptiveCoder(compress.NewCDF16(2, true)).Code(buffer)
		},
		Uncompress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.FilteredAdaptiveDecoder(compress.NewCDF16(2, true)).Decode(buffer)
		},
	},
	{
		Name: "filtered meta adaptive predictive coder 16",
		Compress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.FilteredAdaptiveCoder(compress.NewMeta16(2, true)).Code(buffer)
		},
		Uncompress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.FilteredAdaptiveDecoder(compress.NewMeta16(2, true)).Decode(buffer)
		},
	},
	{
		Name: "filtered adaptive predictive coder 32",
		Compress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.FilteredAdaptiveCoder32(compress.NewCDF32(2, true)).Code(buffer)
		},
		Uncompress: func(coder *compress.Coder16, buffer *bytes.Buffer) {
			coder.FilteredAdaptiveDecoder32(compress.NewCDF32(2, true)).Decode(buffer)
		},
	},
}

type Configuration struct {
	Name       string
	Compress   func(in chan []byte, buffer *bytes.Buffer)
	Uncompress func(in chan []byte, buffer *bytes.Buffer)
}

var configurations = [...]Configuration{
	{
		Name: "burrows-wheeler adaptive coder 16",
		Compress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerCoder(in).MoveToFrontRunLengthCoder().AdaptiveCoder().Code(buffer)
		},
		Uncompress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerDecoder(in).MoveToFrontRunLengthDecoder().AdaptiveDecoder().Decode(buffer)
		},
	},
	{
		Name: "burrows-wheeler adaptive predictive coder 16",
		Compress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerCoder(in).MoveToFrontRunLengthCoder().AdaptivePredictiveCoder().Code(buffer)
		},
		Uncompress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerDecoder(in).MoveToFrontRunLengthDecoder().AdaptivePredictiveDecoder().Decode(buffer)
		},
	},
	{
		Name: "burrows-wheeler adaptive bit coder 16",
		Compress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerCoder(in).MoveToFrontCoder().AdaptiveBitCoder().Code(buffer)
		},
		Uncompress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerDecoder(in).MoveToFrontDecoder().AdaptiveBitDecoder().Decode(buffer)
		},
	},
	{
		Name: "burrows-wheeler adaptive predictive bit coder 16",
		Compress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerCoder(in).MoveToFrontCoder().AdaptivePredictiveBitCoder().Code(buffer)
		},
		Uncompress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerDecoder(in).MoveToFrontDecoder().AdaptivePredictiveBitDecoder().Decode(buffer)
		},
	},
	{
		Name: "burrows-wheeler filtered adaptive bit coder 16",
		Compress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerCoder(in).MoveToFrontCoder().FilteredAdaptiveBitCoder().Code(buffer)
		},
		Uncompress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerDecoder(in).MoveToFrontDecoder().FilteredAdaptiveBitDecoder().Decode(buffer)
		},
	},
	{
		Name: "burrows-wheeler filtered adaptive predictive bit coder 16",
		Compress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerCoder(in).MoveToFrontCoder().FilteredAdaptivePredictiveBitCoder().Code(buffer)
		},
		Uncompress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerDecoder(in).MoveToFrontDecoder().FilteredAdaptivePredictiveBitDecoder().Decode(buffer)
		},
	},
	{
		Name: "burrows-wheeler filtered adaptive coder 16",
		Compress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerCoder(in).MoveToFrontCoder().
				FilteredAdaptiveCoder(compress.NewCDF16(0, true)).Code(buffer)
		},
		Uncompress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerDecoder(in).MoveToFrontDecoder().
				FilteredAdaptiveDecoder(compress.NewCDF16(0, true)).Decode(buffer)
		},
	},
	{
		Name: "burrows-wheeler filtered adaptive predictive coder 16",
		Compress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerCoder(in).MoveToFrontCoder().
				FilteredAdaptiveCoder(compress.NewCDF16(2, true)).Code(buffer)
		},
		Uncompress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerDecoder(in).MoveToFrontDecoder().
				FilteredAdaptiveDecoder(compress.NewCDF16(2, true)).Decode(buffer)
		},
	},
}

var configurations32 = [...]Configuration{
	{
		Name: "burrows-wheeler adaptive coder 32",
		Compress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerCoder(in).MoveToFrontRunLengthCoder().AdaptiveCoder32().Code(buffer)
		},
		Uncompress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerDecoder(in).MoveToFrontRunLengthDecoder().AdaptiveDecoder32().Decode(buffer)
		},
	},
	{
		Name: "burrows-wheeler adaptive predictive coder 32",
		Compress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerCoder(in).MoveToFrontRunLengthCoder().AdaptivePredictiveCoder32().Code(buffer)
		},
		Uncompress: func(in chan []byte, buffer *bytes.Buffer) {
			compress.BijectiveBurrowsWheelerDecoder(in).MoveToFrontRunLengthDecoder().AdaptivePredictiveDecoder32().Decode(buffer)
		},
	},
}

func Compress(input []byte) {
	for c := range coder16Configurations {
		/* compress */
		start, data := time.Now(), make([]uint16, len(input))
		for i := range input {
			data[i] = uint16(input[i])
		}
		symbols, buffer := make(chan []uint16, 1), &bytes.Buffer{}
		symbols <- data
		close(symbols)
		coder16Configurations[c].Compress(&compress.Coder16{Alphabit: 256, Input: symbols}, buffer)
		fmt.Println(coder16Configurations[c].Name)
		fmt.Printf("compressed=%v\n", buffer.Len())
		fmt.Printf("ratio=%v\n", float64(buffer.Len())/float64(len(input)))
		fmt.Println(time.Now().Sub(start).String())

		/* decompress */
		start = time.Now()
		out, i := make([]byte, len(input)), 0
		output := func(symbol uint16) bool {
			out[i] = byte(symbol)
			i++
			return i >= len(data)
		}
		coder16Configurations[c].Uncompress(&compress.Coder16{Alphabit: 256, Output: output}, buffer)
		if bytes.Compare(input, out) != 0 {
			fmt.Println("decompression didn't work")
			failed = append(failed, coder16Configurations[c].Name)
		} else {
			fmt.Println("decompression worked")
		}
		fmt.Println(time.Now().Sub(start).String())
		fmt.Println()
	}

	for c := range configurations {
		/* compress */
		start, in, data := time.Now(), make(chan []byte, 1), make([]byte, len(input))
		copy(data, input)
		buffer := &bytes.Buffer{}
		in <- data
		close(in)
		configurations[c].Compress(in, buffer)
		fmt.Println(configurations[c].Name)
		fmt.Printf("compressed=%v\n", buffer.Len())
		fmt.Printf("ratio=%v\n", float64(buffer.Len())/float64(len(input)))
		fmt.Println(time.Now().Sub(start).String())

		/* decompress */
		start, in = time.Now(), make(chan []byte, 1)
		uncompressed := make([]byte, len(input))
		in <- uncompressed
		close(in)
		configurations[c].Uncompress(in, buffer)
		if bytes.Compare(input, uncompressed) != 0 {
			fmt.Println("decompression didn't work")
			failed = append(failed, configurations[c].Name)
		} else {
			fmt.Println("decompression worked")
		}
		fmt.Println(time.Now().Sub(start).String())
		fmt.Println()
	}

	/* compress */
	start, in, data := time.Now(), make(chan []byte, 1), make([]byte, len(input))
	copy(data, input)
	buffer := &bytes.Buffer{}
	in <- data
	close(in)
	code, sentinels := compress.BurrowsWheelerCoder(in)
	code.MoveToFrontRunLengthCoder().AdaptiveCoder().Code(buffer)
	fmt.Println("suffix tree adaptive coder")
	fmt.Printf("compressed=%v\n", buffer.Len())
	fmt.Printf("ratio=%v\n", float64(buffer.Len())/float64(len(input)))
	fmt.Println(time.Now().Sub(start).String())

	/* decompress */
	start, in = time.Now(), make(chan []byte, 1)
	uncompressed := make([]byte, len(input))
	in <- uncompressed
	close(in)
	compress.BurrowsWheelerDecoder(in, sentinels).MoveToFrontRunLengthDecoder().AdaptiveDecoder().Decode(buffer)
	if bytes.Compare(input, uncompressed) != 0 {
		fmt.Println("decompression didn't work")
		failed = append(failed, "suffix tree adaptive coder")
	} else {
		fmt.Println("decompression worked")
	}
	fmt.Println(time.Now().Sub(start).String())
	fmt.Println()
}

func Compress32(input []byte) {
	for c := range configurations32 {
		/* compress */
		start, in, data := time.Now(), make(chan []byte, 1), make([]byte, len(input))
		copy(data, input)
		buffer := &bytes.Buffer{}
		in <- data
		close(in)
		configurations32[c].Compress(in, buffer)
		fmt.Println(configurations32[c].Name)
		fmt.Printf("compressed=%v\n", buffer.Len())
		fmt.Printf("ratio=%v\n", float64(buffer.Len())/float64(len(input)))
		fmt.Println(time.Now().Sub(start).String())

		/* decompress */
		start, in = time.Now(), make(chan []byte, 1)
		uncompressed := make([]byte, len(input))
		in <- uncompressed
		close(in)
		configurations32[c].Uncompress(in, buffer)
		if bytes.Compare(input, uncompressed) != 0 {
			fmt.Println("decompression didn't work")
			failed = append(failed, configurations32[c].Name)
		} else {
			fmt.Println("decompression worked")
		}
		fmt.Println(time.Now().Sub(start).String())
		fmt.Println()
	}
}

func Test(file string) {
	var data []byte
	if strings.HasSuffix(file, ".jpg") {
		d, err := os.Open(file)
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			d.Close()
		}()
		img, format, err := image.Decode(d)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(format)
		bounds := img.Bounds()
		fmt.Println(bounds)
		gray := image.NewGray(bounds)
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				gray.Set(x, y, img.At(x, y))
			}
		}

		data = gray.Pix
	} else {
		d, err := ioutil.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}

		data = d
	}

	fmt.Printf("entropy=%v\n\n", Entropy(data))

	/*symbols, table := &Symbols8{}, [256]uint8{}
	symbols.Count(data)
	symbols.Sort()
	for k, v := range symbols {
		table[v.uint8] = uint8(k)
	}
	for k, v := range data {
		data[k] = table[v]
	}*/

	start := time.Now()
	Compress(data)
	fmt.Println(time.Now().Sub(start).String())
	fmt.Printf("\n")

	start = time.Now()
	Compress32(data)
	fmt.Println(time.Now().Sub(start).String())
	fmt.Printf("\n")
}

func main() {
	fmt.Printf("cpus=%v\n", runtime.NumCPU())
	runtime.GOMAXPROCS(64)
	files := [...]string{"alice30.txt", "310px-Tesla_colorado_adjusted.jpg"}
	for _, file := range files {
		fmt.Printf("./%v\n", file)
		Test(file)
		fmt.Printf("\n")
	}

	if len(failed) > 0 {
		fmt.Println("decompression failed:")
		for _, fail := range failed {
			fmt.Println(fail)
		}
	} else {
		fmt.Println("decompression worked")
	}
}
