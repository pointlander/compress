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

type ContextCDF16 struct {
	Size    int
	Root    []uint16
	CDF     [][]uint16
	Context []uint16
	First   int
	Mixin   [][]uint16
	Verify  bool
}

func NewContextCDF16(size int) *ContextCDF16 {
	if size != 256 {
		panic("size is not 256")
	}
	root, cdf, mixin := make([]uint16, size+1), make([][]uint16, 65536), make([][]uint16, size)

	sum := 0
	for i := range root {
		root[i] = uint16(sum)
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

	return &ContextCDF16{
		Size:    size,
		Root:    root,
		CDF:     cdf,
		Context: make([]uint16, 2),
		Mixin:   mixin,
		Verify:  true,
	}
}

func (c *ContextCDF16) Model() []uint16 {
	ctx, first := c.Context, c.First
	length, context := len(ctx), uint16(ctx[first])
	for i := (first + 1) % length; i != first; i = (i + 1) % length {
		context = context<<8 | ctx[i]
	}

	model := c.CDF[context]
	if model == nil {
		model = c.Root
	}

	return model
}

func (c *ContextCDF16) Update(s uint16) {
	ctx, first := c.Context, c.First
	length, context := len(ctx), uint16(ctx[first])
	for i := (first + 1) % length; i != first; i = (i + 1) % length {
		context = context<<8 | ctx[i]
	}

	size, root, cdf, mixin := c.Size, c.Root, c.CDF[context], c.Mixin[s]
	if cdf == nil {
		cdf = make([]uint16, size+1)
		sum := 0
		for i := range cdf {
			cdf[i] = uint16(sum)
			sum += 32
		}
		c.CDF[context] = cdf
	}

	ctx[first], c.First = s, (first+1)%length

	if c.Verify {
		for i := 1; i < size; i++ {
			a, b := int(root[i]), int(mixin[i])
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
			root[i] = uint16(a + ((b - a) >> CDF16Rate))
		}
		if root[size] != CDF16Scale {
			panic("cdf scale is incorrect")
		}
		for i := 1; i < len(root); i++ {
			if a, b := root[i], root[i-1]; a < b {
				panic(fmt.Sprintf("invalid cdf %v,%v < %v,%v", i, a, i-1, b))
			} else if a == b {
				panic(fmt.Sprintf("invalid cdf %v,%v = %v,%v", i, a, i-1, b))
			}
		}

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
		a, b := int(root[i]), int(mixin[i])
		/*c := (b - a)
		if c >= 0 {
			c >>= cdfRate
			c = a + c
		} else {
			c = -c
			c >>= cdfRate
			c = a - c
		}*/
		root[i] = uint16(a + ((b - a) >> CDF16Rate))
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
