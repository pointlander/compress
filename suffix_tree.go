// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compress

//import "fmt"
//import "time"

type Edge struct {
	first_index, last_index, start_node, end_node int
}

type SuffixTree struct {
	edges  map[uint]Edge
	nodes  []int
	buffer []byte
}

const SYMBOL_SIZE = 9

func BuildSuffixTree(input []byte) *SuffixTree {
	length := len(input)
	size := 2 * length
	edges, nodes := make(map[uint]Edge, size), make([]int, size)
	for i := range nodes {
		nodes[i] = -1
	}

	putEdge := func(edge Edge) {
		symbol := uint(input[edge.first_index])
		edges[(uint(edge.start_node)<<SYMBOL_SIZE)|symbol] = edge
	}

	getEdge := func(node, index int) Edge {
		symbol := uint(input[index])
		return edges[(uint(node)<<SYMBOL_SIZE)|symbol]
	}

	hasEdge := func(node, index int) bool {
		symbol := uint(input[index])
		_, has := edges[(uint(node)<<SYMBOL_SIZE)|symbol]
		return has
	}

	putNode := func(node, parent int) {
		nodes[node] = parent
	}

	getNode := func(node int) int {
		return nodes[node]
	}

	node_count, parent_node, origin, first_index, last_index := 1, 0, 0, 0, -1

	findEdge := func(i int, v byte) bool {
		if first_index > last_index {
			if hasEdge(origin, i) {
				return true
			}
		} else {
			edge, last_edge := getEdge(origin, first_index), last_index-first_index
			last_edge += edge.first_index
			next_edge := last_edge + 1
			if v == input[next_edge] {
				return true
			}
			putEdge(Edge{first_index: edge.first_index, last_index: last_edge, start_node: origin, end_node: node_count})
			putNode(node_count, origin)
			edge.first_index, edge.start_node = next_edge, node_count
			putEdge(edge)
			parent_node = node_count
			node_count++
		}
		return false
	}

	canonize := func() {
		if first_index > last_index {
			return
		}
		edge := getEdge(origin, first_index)
		span := edge.last_index - edge.first_index
		for span <= (last_index - first_index) {
			first_index += span + 1
			origin = edge.end_node
			if first_index <= last_index {
				edge = getEdge(edge.end_node, first_index)
				span = edge.last_index - edge.first_index
			}
		}
	}

	addEdge := func(i int) {
		putEdge(Edge{first_index: i, last_index: length, start_node: parent_node, end_node: node_count})
		node_count++
		if origin == 0 {
			first_index++
		} else {
			origin = getNode(origin)
		}
		canonize()
	}

	for i, v := range input {
		parent_node = origin
		if findEdge(i, v) {
			last_index++
			canonize()
			continue
		}
		addEdge(i)
		last_parent_node := parent_node
		parent_node = origin
		for !findEdge(i, v) {
			addEdge(i)
			putNode(last_parent_node, parent_node)
			last_parent_node, parent_node = parent_node, origin
		}
		putNode(last_parent_node, parent_node)
		last_index++
		canonize()
	}

	/*add the last end nodes*/
	putEdge = func(edge Edge) {
		symbol := uint(256)
		if int(edge.first_index) < length {
			symbol = uint(input[edge.first_index])
		}

		edges[(uint(edge.start_node)<<SYMBOL_SIZE)|symbol] = edge
	}

	getEdge = func(node, index int) Edge {
		symbol := uint(256)
		if index < length {
			symbol = uint(input[index])
		}

		return edges[(uint(node)<<SYMBOL_SIZE)|symbol]
	}

	hasEdge = func(node, index int) bool {
		symbol := uint(256)
		if index < length {
			symbol = uint(input[index])
		}

		_, has := edges[(uint(node)<<SYMBOL_SIZE)|symbol]
		return has
	}

	findEdge = func(i int, v byte) bool {
		if first_index > last_index {
			if hasEdge(origin, i) {
				return true
			}
		} else {
			edge, last_edge := getEdge(origin, first_index), last_index-first_index
			last_edge += edge.first_index
			next_edge := last_edge + 1
			if next_edge == length {
				return true
			}
			putEdge(Edge{first_index: edge.first_index, last_index: last_edge, start_node: origin, end_node: node_count})
			putNode(node_count, origin)
			edge.first_index, edge.start_node = next_edge, node_count
			putEdge(edge)
			parent_node = node_count
			node_count++
		}
		return false
	}

	tree := &SuffixTree{edges: edges, nodes:  nodes, buffer: input}
	parent_node = origin
	if findEdge(length, 0) {
		return tree
	}
	addEdge(length)
	last_parent_node := parent_node
	parent_node = origin
	for !findEdge(length, 0) {
		addEdge(length)
		putNode(last_parent_node, parent_node)
		last_parent_node, parent_node = parent_node, origin
	}
	putNode(last_parent_node, parent_node)
	return tree
}

func (tree *SuffixTree) Index(sep string) int {
	i, node, last_i := 0, 0, 0
	var last_edge Edge
search:
	for i < len(sep) {
		edge, has := tree.edges[(uint(node)<<SYMBOL_SIZE)+uint(sep[i])]
		if !has {
			return -1
		}
		/*fmt.Printf("at node %v %v %v %v\n", edge.first_index, edge.last_index, edge.start_node, edge.end_node)
		  fmt.Printf("found '%c'\n", sep[i])*/
		node, last_edge, last_i, i = int(edge.end_node), edge, i, i+1
		if edge.first_index >= edge.last_index {
			continue search
		}
		for index := edge.first_index + 1; index <= edge.last_index && i < len(sep); index++ {
			if sep[i] != tree.buffer[index] {
				return -1
			}
			/*fmt.Printf("found '%c'\n", sep[i])*/
			i++
		}
	}
	/*fmt.Printf("%v\n", string(tree.buffer[int(last_edge.first_index) - last_i:int(last_edge.first_index) - last_i + len(sep)]))*/
	return int(last_edge.first_index) - last_i
}

func (tree *SuffixTree) BurrowsWheelerCoder() (<-chan byte, <-chan int) {
	histogram, end, out, sentinel, written := [257]uint{}, len(tree.buffer), make(chan byte, 8), make(chan int, 1), 0

	for _, v := range tree.buffer {
		histogram[v]++
	}
	histogram[256]++

	var walk func(node, depth uint)
	walk = func(node, depth uint) {
		node <<= SYMBOL_SIZE
		edges := make(chan Edge, 4)
		go func() {
			for c := uint(0); c <= 256; c++ {
				if histogram[c] == 0 {
					continue
				}
				if edge, has := tree.edges[node+c]; has {
					edges <- edge
				}
			}
			close(edges)
		}()

		for edge := range edges {
			depth := depth + uint(edge.last_index) - uint(edge.first_index) + 1
			if int(edge.last_index) < end {
				walk(uint(edge.end_node), depth)
			} else if depth > uint(end) {
				/*out <- '$'*/
				sentinel <- written
				/*written++*/
			} else {
				/*if depth == 1 {
					fmt.Printf("here %v %c\n", written, tree.buffer[uint(end)-depth])
				}*/
				out <- tree.buffer[uint(end)-depth]
				written++
			}
		}
	}
	go func() { walk(0, 0); close(out) }()
	return out, sentinel
}

func burrowsWheelerDecoder(input []byte, key int) (output []byte) {
	length, sum := len(input), 0
	minor, major, output := make([]int, length), [257]int{}, make([]byte, length-1)
	for k, v := range input {
		v := int(v)
		if k == key {
			v = 256
		}
		minor[k] = major[v]
		major[v]++
	}

	for k, v := range major {
		major[k] = sum
		sum += v
	}

	key = length - 1
	for c := length - 2; c >= 0; c-- {
		output[c], key = input[key], major[input[key]]+minor[key]
	}
	return
}

func BurrowsWheelerCoder(input <-chan []byte) (Coder8, <-chan int) {
	output, sentinels := make(chan []byte), make(chan int, BUFFER_COUNT)

	var buffer []uint8
	encode := func(block []byte) {
		if cap(buffer) < len(block) {
			buffer = make([]uint8, len(block))
		} else {
			buffer = buffer[:len(block)]
		}
		copy(buffer, block)

		//start := time.Now()
		tree := BuildSuffixTree(buffer)
		//fmt.Printf("build: %v\n", time.Now().Sub(start).String())

		//start = time.Now()
		edges := tree.edges
		histogram, end, written := [257]uint{}, len(buffer), 0

		for _, v := range buffer {
			histogram[v]++
		}
		histogram[256]++

		var walk func(node, depth uint)
		walk = func(node, depth uint) {
			node <<= SYMBOL_SIZE
			for c := uint(0); c <= 256; c++ {
				if histogram[c] == 0 {
					continue
				}

				edge, has := edges[node|c]
				if !has {
					continue
				}

				depth := depth + uint(edge.last_index) - uint(edge.first_index) + 1
				if int(edge.last_index) < end {
					walk(uint(edge.end_node), depth)
				} else if depth > uint(end) {
					sentinels <- written
				} else {
					block[written], written = buffer[uint(end)-depth], written + 1
				}
			}
		}

		walk(0, 0)
		//fmt.Printf("walk: %v\n", time.Now().Sub(start).String())
	}

	go func() {
		for block := range input {
			encode(block)
			output <- block
		}

		close(output)
	}()

	return Coder8{Alphabit:256, Input:output}, sentinels
}

func BurrowsWheelerDecoder(input <-chan []byte, sentinels <-chan int) Coder8 {
	inverse := func(buffer []byte, key int) {
		length, sum := len(buffer), 0
		minor, major, input := make([]int, length + 1), [257]int{}, make([]byte, length + 1)

		copy(input, buffer[:key])
		copy(input[key + 1:], buffer[key:])
		for k, v := range input {
			v := int(v)
			if k == key {
				v = 256
			}
			minor[k] = major[v]
			major[v]++
		}

		for k, v := range major {
			major[k] = sum
			sum += v
		}

		key = length
		for c := length - 1; c >= 0; c-- {
			buffer[c], key = input[key], major[input[key]]+minor[key]
		}
	}

	buffer, i := []byte(nil), 0
	add := func(symbol uint8) bool {
		if len(buffer) == 0 {
			next, ok := <-input
			if !ok {
				return true
			}
			buffer = next
		}

		buffer[i], i = symbol, i + 1
		if i == len(buffer) {
			inverse(buffer, <-sentinels)
			next, ok := <-input
			if !ok {
				return true
			}
			buffer, i = next, 0
		}
		return false
	}

        return Coder8{Alphabit:256, Output:add}

}
