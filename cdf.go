package compress

import "fmt"

const (
	CDF16Fixed = 16 - 3
	CDF16Scale = 1 << CDF16Fixed
	CDF16Rate  = 5
)

type CDF16 struct {
	CDF    []uint16
	Mixin  [][]uint16
	Verify bool
}

func NewCDF16(size int) *CDF16 {
	if size != 256 {
		panic("size is not 256")
	}
	cdf, mixin := make([]uint16, size+1), make([][]uint16, size)

	sum := 0
	for i := range cdf {
		cdf[i] = uint16(sum)
		sum += 32
	}

	for i := range mixin {
		sum, m := 0, make([]uint16, size+1)
		for j := range m {
			m[j] = uint16(sum)
			sum++
			if j == i {
				sum += CDF16Scale - size
			}
		}
		mixin[i] = m
	}

	return &CDF16{
		CDF:    cdf,
		Mixin:  mixin,
		Verify: true,
	}
}

func (c *CDF16) Update(s int) {
	cdf, mixin := c.CDF, c.Mixin[s]
	size := len(cdf) - 1

	if c.Verify {
		for i := 1; i < size; i++ {
			a, b := int(cdf[i]), int(mixin[i])
			if a < 0 {
				panic("a is less than zero")
			}
			if b < 0 {
				panic("b is less than zero")
			}
			/*c := (b - a)
			if c >= 0 {
				c >>= cdfRate
				c = a + c
			} else {
				c = -c
				c >>= cdfRate
				c = a - c
			}
			if c < 0 {
				panic("c is less than zero")
			}*/
			cdf[i] = uint16(a + ((b - a) >> CDF16Rate))
		}
		if cdf[size] != CDF16Scale {
			panic("cdf scale is incorrect")
		}
		for i := 1; i < len(cdf); i++ {
			if a, b := cdf[i], cdf[i-1]; a < b {
				panic(fmt.Sprintf("invalid cdf %v,%v < %v,%v", i, a, i-1, b))
			} else if a == b {
				panic(fmt.Sprintf("invalid cdf %v,%v = %v,%v", i, a, i-1, b))
			}
		}
		return
	}

	for i := 1; i < size; i++ {
		a, b := int(cdf[i]), int(mixin[i])
		/*c := (b - a)
		if c >= 0 {
			c >>= cdfRate
			c = a + c
		} else {
			c = -c
			c >>= cdfRate
			c = a - c
		}*/
		cdf[i] = uint16(a + ((b - a) >> CDF16Rate))
	}
}
