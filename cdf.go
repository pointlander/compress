package compress

import "fmt"

const (
	CDF16Fixed = 16 - 3
	CDF16Scale = 1 << CDF16Fixed
	CDF16Rate  = 5
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
	Size     int
	MaxDepth int
	Root     *Node16
	Context  []uint16
	First    int
	Mixin    [][]uint16
	Verify   bool
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
