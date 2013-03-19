package fractal

import (
	"bytes"
	"code.google.com/p/lzma"
	"compress/zlib"
	"fmt"
	"github.com/nfnt/resize"
	"github.com/pointlander/compress"
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
)

func TestFractal(t *testing.T) {
	runtime.GOMAXPROCS(64)

	fcpBuffer := &bytes.Buffer{}
	file, err := os.Open("images/lenna.png")
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

	width, height, scale := input.Bounds().Max.X, input.Bounds().Max.Y, 4
	width, height = width/scale, height/scale
	input = resize.Resize(uint(width), uint(height), input, resize.NearestNeighbor)
	FractalCoder(input, 2, fcpBuffer)
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

	decoded = FractalDecoder(fcpBufferCopy, 4)

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
		fmt.Printf("%.3f%% %7vb\n", 100 * float64(output.Len())/float64(buffer.Len()), output.Len())
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
					R:uint8(r >> 8),
					G:uint8(g >> 8),
					B:uint8(b >> 8),
					A:0xFF})
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
		return buffer.Len(), "code.google.com/p/lzma"
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
		jpeg.Encode(buffer, input, nil)
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
