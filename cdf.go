package compress

import "fmt"

const (
	// CDF16Fixed is the shift for 16 bit coders
	CDF16Fixed = 16 - 3
	// CDF16Scale is the scale for 16 bit coder
	CDF16Scale = 1 << CDF16Fixed
	// CDF16Rate is the damping factor for 16 bit coder
	CDF16Rate = 5
	// CDF32Fixed is the shift for 32 bit coders
	CDF32Fixed = 32 - 3
	// CDF32Scale is the scale for 32 bit coder
	CDF32Scale = 1 << CDF32Fixed
	// CDF32Rate is the damping factor for 32 bit coder
	CDF32Rate = 5
)

type Node16 struct {
	Model    []uint16
	Children map[uint16]*Node16
}

func NewNode16(size int) *Node16 {
	model, children, sum := make([]uint16, size+1), make(map[uint16]*Node16), 0
	for i := range model {
		model[i] = uint16(sum)
		sum += 32
	}
	return &Node16{
		Model:    model,
		Children: children,
	}
}

type CDF16 struct {
	Size    int
	Root    *Node16
	Context []uint16
	First   int
	Mixin   [][]uint16
	Verify  bool
}

type CDF16Maker func(size int) *CDF16

func NewCDF16(depth int, verify bool) CDF16Maker {
	return func(size int) *CDF16 {
		if size != 256 {
			panic("size is not 256")
		}
		root, mixin := NewNode16(size), make([][]uint16, size)

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
			Size:    size,
			Root:    root,
			Context: make([]uint16, depth),
			Mixin:   mixin,
			Verify:  verify,
		}
	}
}

func (c *CDF16) Model() []uint16 {
	context := c.Context
	length := len(context)
	var lookUp func(n *Node16, current, depth int) *Node16
	lookUp = func(n *Node16, current, depth int) *Node16 {
		if depth >= length {
			return n
		}

		node := n.Children[context[current]]
		if node == nil {
			return n
		}
		child := lookUp(node, (current+1)%length, depth+1)
		if child == nil {
			return n
		}
		return child
	}

	return lookUp(c.Root, c.First, 0).Model
}

func (c *CDF16) Update(s uint16) {
	context, first, mixin := c.Context, c.First, c.Mixin[s]
	length := len(context)
	var update func(n *Node16, current, depth int)
	update = func(n *Node16, current, depth int) {
		model := n.Model
		size := len(model) - 1

		if c.Verify {
			for i := 1; i < size; i++ {
				a, b := int(model[i]), int(mixin[i])
				if a < 0 {
					panic("a is less than zero")
				}
				if b < 0 {
					panic("b is less than zero")
				}
				model[i] = uint16(a + ((b - a) >> CDF16Rate))
			}
			if model[size] != CDF16Scale {
				panic("cdf scale is incorrect")
			}
			for i := 1; i < len(model); i++ {
				if a, b := model[i], model[i-1]; a < b {
					panic(fmt.Sprintf("invalid cdf %v,%v < %v,%v", i, a, i-1, b))
				} else if a == b {
					panic(fmt.Sprintf("invalid cdf %v,%v = %v,%v", i, a, i-1, b))
				}
			}
		} else {
			for i := 1; i < size; i++ {
				a, b := int(model[i]), int(mixin[i])
				model[i] = uint16(a + ((b - a) >> CDF16Rate))
			}
		}

		if depth >= length {
			return
		}

		node := n.Children[context[current]]
		if node == nil {
			node = NewNode16(size)
			n.Children[context[current]] = node
		}
		update(node, (current+1)%length, depth+1)
	}

	update(c.Root, first, 0)
	if length > 0 {
		context[first], c.First = s, (first+1)%length
	}
}

type Node32 struct {
	Model    []uint32
	Children map[uint16]*Node32
}

func NewNode32(size int) *Node32 {
	model, children, sum := make([]uint32, size+1), make(map[uint16]*Node32), 0
	for i := range model {
		model[i] = uint32(sum)
		sum += 2097152
	}
	return &Node32{
		Model:    model,
		Children: children,
	}
}

type CDF32 struct {
	Size    int
	Root    *Node32
	Context []uint16
	First   int
	Mixin   [][]uint32
	Verify  bool
}

type CDF32Maker func(size int) *CDF32

func NewCDF32(depth int, verify bool) CDF32Maker {
	return func(size int) *CDF32 {
		if size != 256 {
			panic("size is not 256")
		}
		root, mixin := NewNode32(size), make([][]uint32, size)

		for i := range mixin {
			sum, m := 0, make([]uint32, size+1)
			for j := range m {
				m[j] = uint32(sum)
				sum++
				if j == i {
					sum += CDF32Scale - size
				}
			}
			mixin[i] = m
		}

		return &CDF32{
			Size:    size,
			Root:    root,
			Context: make([]uint16, depth),
			Mixin:   mixin,
			Verify:  verify,
		}
	}
}

func (c *CDF32) Model() []uint32 {
	context := c.Context
	length := len(context)
	var lookUp func(n *Node32, current, depth int) *Node32
	lookUp = func(n *Node32, current, depth int) *Node32 {
		if depth >= length {
			return n
		}

		node := n.Children[context[current]]
		if node == nil {
			return n
		}
		child := lookUp(node, (current+1)%length, depth+1)
		if child == nil {
			return n
		}
		return child
	}

	return lookUp(c.Root, c.First, 0).Model
}

func (c *CDF32) Update(s uint16) {
	context, first, mixin := c.Context, c.First, c.Mixin[s]
	length := len(context)
	var update func(n *Node32, current, depth int)
	update = func(n *Node32, current, depth int) {
		model := n.Model
		size := len(model) - 1

		if c.Verify {
			for i := 1; i < size; i++ {
				a, b := int64(model[i]), int64(mixin[i])
				if a < 0 {
					panic("a is less than zero")
				}
				if b < 0 {
					panic("b is less than zero")
				}
				model[i] = uint32(a + ((b - a) >> CDF32Rate))
			}
			if model[size] != CDF32Scale {
				panic("cdf scale is incorrect")
			}
			for i := 1; i < len(model); i++ {
				if a, b := model[i], model[i-1]; a < b {
					panic(fmt.Sprintf("invalid cdf %v,%v < %v,%v", i, a, i-1, b))
				} else if a == b {
					panic(fmt.Sprintf("invalid cdf %v,%v = %v,%v", i, a, i-1, b))
				}
			}
		} else {
			for i := 1; i < size; i++ {
				a, b := int64(model[i]), int64(mixin[i])
				model[i] = uint32(a + ((b - a) >> CDF32Rate))
			}
		}

		if depth >= length {
			return
		}

		node := n.Children[context[current]]
		if node == nil {
			node = NewNode32(size)
			n.Children[context[current]] = node
		}
		update(node, (current+1)%length, depth+1)
	}

	update(c.Root, first, 0)
	if length > 0 {
		context[first], c.First = s, (first+1)%length
	}
}
