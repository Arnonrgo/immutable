package immutable

import (
	"fmt"
	"math/bits"
	"reflect"
)

const (
	// listSliceThreshold is the threshold at which a list will be converted
	// from a slice-based implementation to a trie-based one.
	listSliceThreshold = 32
)

// List is a dense, ordered, indexed collections. They are analogous to slices
// in Go. A List is implemented as a relaxed-radix-balanced tree. The zero value
// of a List is an empty list. A list is safe for concurrent use.
// For smaller lists (under listSliceThreshold elements), it uses a slice internally
// for better performance, and will transparently switch to a trie for larger lists.
type List[T any] struct {
	root   listNode[T] // root node
	origin int         // offset to zero index element
	size   int         // total number of elements in use
}

// NewList returns a new empty instance of List.
func NewList[T any](values ...T) *List[T] {
	if len(values) > listSliceThreshold {
		l := &List[T]{
			root:   &listLeafNode[T]{},
			origin: 0,
			size:   0,
		}
		for _, value := range values {
			l = l.append(value, true)
		}
		return l
	}

	// For small lists, use the slice-based implementation.
	newValues := make([]T, len(values))
	copy(newValues, values)
	return &List[T]{
		root: &listSliceNode[T]{elements: newValues},
		size: len(values),
	}
}

// clone returns a copy of the list.
func (l *List[T]) clone() *List[T] {
	other := *l
	return &other
}

// Len returns the number of elements in the list.
func (l *List[T]) Len() int { return l.size }

// cap returns the total number of possible elements for the current depth.
func (l *List[T]) cap() int { return 1 << (l.root.depth() * listNodeBits) }

// Get returns the value at the given index. Similar to slices, this method will
// panic if index is below zero or is greater than or equal to the list size.
func (l *List[T]) Get(index int) T {
	if index < 0 || index >= l.size {
		panic(fmt.Sprintf("immutable.List.Get: index %d out of bounds", index))
	}
	if sliceNode, ok := l.root.(*listSliceNode[T]); ok {
		return sliceNode.elements[index]
	}
	return l.root.get(l.origin + index)
}

// Contains returns true if the list contains the given value.
// For comparable element types, this uses ==. For non-comparable types, it
// falls back to reflect.DeepEqual. This method does not mutate the list and is
// safe for concurrent use across goroutines.
func (l *List[T]) Contains(value T) bool {
	if l.size == 0 {
		return false
	}
	// Equality function with fast path for comparable types.
	eq := func(a, b T) bool {
		ta := reflect.TypeOf(a)
		if ta != nil && ta.Comparable() {
			return any(a) == any(b)
		}
		return reflect.DeepEqual(a, b)
	}
	// Optimize for slice-backed lists.
	if sliceNode, ok := l.root.(*listSliceNode[T]); ok {
		for i := 0; i < len(sliceNode.elements); i++ {
			if eq(sliceNode.elements[i], value) {
				return true
			}
		}
		return false
	}
	// Fallback to iterator for trie-backed lists.
	itr := l.Iterator()
	for !itr.Done() {
		_, v := itr.Next()
		if eq(v, value) {
			return true
		}
	}
	return false
}

// ContainsFunc returns true if the list contains a value equal to the provided
// value using the caller-supplied equality function.
// The equality function should define equivalence for two values of type T and
// must be free of side effects. This method does not mutate the list and is safe
// for concurrent use across goroutines.
func (l *List[T]) ContainsFunc(value T, equal func(a, b T) bool) bool {
	if l.size == 0 {
		return false
	}
	assert(equal != nil, "immutable.List.ContainsFunc: equal function must not be nil")
	if sliceNode, ok := l.root.(*listSliceNode[T]); ok {
		for i := 0; i < len(sliceNode.elements); i++ {
			if equal(sliceNode.elements[i], value) {
				return true
			}
		}
		return false
	}
	itr := l.Iterator()
	for !itr.Done() {
		_, v := itr.Next()
		if equal(v, value) {
			return true
		}
	}
	return false
}

// Set returns a new list with value set at index. Similar to slices, this
// method will panic if index is below zero or if the index is greater than
// or equal to the list size.
func (l *List[T]) Set(index int, value T) *List[T] { return l.set(index, value, false) }

func (l *List[T]) set(index int, value T, mutable bool) *List[T] {
	if index < 0 || index >= l.size {
		panic(fmt.Sprintf("immutable.List.Set: index %d out of bounds", index))
	}
	// If it's a slice node, the logic is simple.
	if sliceNode, ok := l.root.(*listSliceNode[T]); ok {
		other := l
		if !mutable {
			other = l.clone()
		}
		other.root = sliceNode.set(index, value, mutable)
		return other
	}
	// Otherwise, use the existing trie logic.
	other := l
	if !mutable {
		other = l.clone()
	}
	other.root = l.root.set(l.origin+index, value, mutable)
	return other
}

// Append returns a new list with value added to the end of the list.
func (l *List[T]) Append(value T) *List[T] { return l.append(value, false) }

func (l *List[T]) append(value T, mutable bool) *List[T] {
	// If it's a slice node and there's room, append to the slice.
	if sliceNode, ok := l.root.(*listSliceNode[T]); ok {
		if l.size < listSliceThreshold {
			newElements := make([]T, l.size+1)
			copy(newElements, sliceNode.elements)
			newElements[l.size] = value
			other := l
			if !mutable {
				other = l.clone()
			}
			other.root = &listSliceNode[T]{elements: newElements}
			other.size++
			return other
		}
		// If we are at the threshold, we need to convert to a trie.
		trieRoot := sliceNode.toTrie(true)
		tempList := &List[T]{root: trieRoot, size: l.size, origin: 0}
		return tempList.append(value, mutable)
	}
	// Standard trie-based append logic
	other := l
	if !mutable {
		other = l.clone()
	}
	// Expand list to the right if no slots remain.
	if other.size+other.origin >= l.cap() {
		newRoot := &listBranchNode[T]{d: other.root.depth() + 1}
		newRoot.children[0] = other.root
		other.root = newRoot
	}
	// Increase size and set the last element to the new value.
	other.size++
	other.root = other.root.set(other.origin+other.size-1, value, mutable)
	return other
}

// Prepend returns a new list with value(s) added to the beginning of the list.
func (l *List[T]) Prepend(value T) *List[T] { return l.prepend(value, false) }

func (l *List[T]) prepend(value T, mutable bool) *List[T] {
	// If it's a slice node and there's room, prepend to the slice.
	if sliceNode, ok := l.root.(*listSliceNode[T]); ok {
		if l.size < listSliceThreshold {
			newElements := make([]T, l.size+1)
			newElements[0] = value
			copy(newElements[1:], sliceNode.elements)
			other := l
			if !mutable {
				other = l.clone()
			}
			other.root = &listSliceNode[T]{elements: newElements}
			other.size++
			return other
		}
		// If we are at the threshold, we need to convert to a trie.
		trieRoot := sliceNode.toTrie(true)
		tempList := &List[T]{root: trieRoot, size: l.size, origin: 0}
		return tempList.prepend(value, mutable)
	}
	// Standard trie-based prepend logic
	other := l
	if !mutable {
		other = l.clone()
	}
	// Expand list to the left if no slots remain.
	if other.origin == 0 {
		newRoot := &listBranchNode[T]{d: other.root.depth() + 1}
		newRoot.children[listNodeSize-1] = other.root
		other.root = newRoot
		other.origin += (listNodeSize - 1) << (other.root.depth() * listNodeBits)
	}
	// Increase size and move origin back. Update first element to value.
	other.size++
	other.origin--
	other.root = other.root.set(other.origin, value, mutable)
	return other
}

// Slice returns a new list of elements between start index and end index.
// Similar to slices, this method will panic if start or end are below zero or
// greater than the list size. A panic will also occur if start is greater than
// end.
// Unlike Go slices, references to inaccessible elements will be automatically
// removed so they can be garbage collected.
func (l *List[T]) Slice(start, end int) *List[T] { return l.slice(start, end, false) }

func (l *List[T]) slice(start, end int, mutable bool) *List[T] {
	// Panics similar to Go slices.
	if start < 0 || start > l.size {
		panic(fmt.Sprintf("immutable.List.Slice: start index %d out of bounds", start))
	} else if end < 0 || end > l.size {
		panic(fmt.Sprintf("immutable.List.Slice: end index %d out of bounds", end))
	} else if start > end {
		panic(fmt.Sprintf("immutable.List.Slice: invalid slice index: [%d:%d]", start, end))
	}
	// Return the same list if the start and end are the entire range.
	if start == 0 && end == l.size {
		return l
	}
	if sliceNode, ok := l.root.(*listSliceNode[T]); ok {
		newElements := make([]T, end-start)
		copy(newElements, sliceNode.elements[start:end])
		return &List[T]{root: &listSliceNode[T]{elements: newElements}, size: end - start}
	}
	// Create copy, if immutable.
	other := l
	if !mutable {
		other = l.clone()
	}
	// Update origin/size.
	other.origin = l.origin + start
	other.size = end - start
	// Contract tree while the start & end are in the same child node.
	for other.root.depth() > 1 {
		i := (other.origin >> (other.root.depth() * listNodeBits)) & listNodeMask
		j := ((other.origin + other.size - 1) >> (other.root.depth() * listNodeBits)) & listNodeMask
		if i != j {
			break
		}
		// Replace the current root with the single child & update origin offset.
		other.origin -= i << (other.root.depth() * listNodeBits)
		other.root = other.root.(*listBranchNode[T]).children[i]
	}
	// Ensure all references are removed before start & after end.
	other.root = other.root.deleteBefore(other.origin, mutable)
	other.root = other.root.deleteAfter(other.origin+other.size-1, mutable)
	return other
}

// Iterator returns a new iterator for this list positioned at the first index.
func (l *List[T]) Iterator() *ListIterator[T] {
	itr := &ListIterator[T]{list: l}
	itr.First()
	return itr
}

// ListBuilder represents an efficient builder for creating new Lists.
type ListBuilder[T any] struct{ list *List[T] }

// NewListBuilder returns a new instance of ListBuilder.
func NewListBuilder[T any]() *ListBuilder[T] { return &ListBuilder[T]{list: NewList[T]()} }

// List returns the current copy of the list.
// The builder should not be used again after the list after this call.
func (b *ListBuilder[T]) List() *List[T] {
	assert(b.list != nil, "immutable.ListBuilder.List(): duplicate call to fetch list")
	list := b.list
	b.list = nil
	return list
}

// Len returns the number of elements in the underlying list.
func (b *ListBuilder[T]) Len() int {
	assert(b.list != nil, "immutable.ListBuilder: builder invalid after List() invocation")
	return b.list.Len()
}

// Get returns the value at the given index.
func (b *ListBuilder[T]) Get(index int) T {
	assert(b.list != nil, "immutable.ListBuilder: builder invalid after List() invocation")
	return b.list.Get(index)
}

// Set updates the value at the given index.
func (b *ListBuilder[T]) Set(index int, value T) {
	assert(b.list != nil, "immutable.ListBuilder: builder invalid after List() invocation")
	b.list = b.list.set(index, value, true)
}

// Append adds value to the end of the list.
func (b *ListBuilder[T]) Append(value T) {
	assert(b.list != nil, "immutable.ListBuilder: builder invalid after List() invocation")
	b.list = b.list.append(value, true)
}

// Prepend adds value to the beginning of the list.
func (b *ListBuilder[T]) Prepend(value T) {
	assert(b.list != nil, "immutable.ListBuilder: builder invalid after List() invocation")
	b.list = b.list.prepend(value, true)
}

// Slice updates the list with a sublist of elements between start and end index.
func (b *ListBuilder[T]) Slice(start, end int) {
	assert(b.list != nil, "immutable.ListBuilder: builder invalid after List() invocation")
	b.list = b.list.slice(start, end, true)
}

// Iterator returns a new iterator for the underlying list.
func (b *ListBuilder[T]) Iterator() *ListIterator[T] {
	assert(b.list != nil, "immutable.ListBuilder: builder invalid after List() invocation")
	return b.list.Iterator()
}

// Contains returns true if the underlying list contains the given value.
func (b *ListBuilder[T]) Contains(value T) bool {
	assert(b.list != nil, "immutable.ListBuilder: builder invalid after List() invocation")
	return b.list.Contains(value)
}

// ContainsFunc returns true if the underlying list contains the given value using provided equality.
func (b *ListBuilder[T]) ContainsFunc(value T, equal func(a, b T) bool) bool {
	assert(b.list != nil, "immutable.ListBuilder: builder invalid after List() invocation")
	return b.list.ContainsFunc(value, equal)
}

// ListIterator represents an ordered iterator over a list.
type ListIterator[T any] struct {
	list  *List[T]
	index int
	stack [32]listIteratorElem[T]
	depth int
}

func (itr *ListIterator[T]) Done() bool { return itr.index < 0 || itr.index >= itr.list.Len() }

// First positions the iterator on the first index.
func (itr *ListIterator[T]) First() {
	if itr.list.Len() != 0 {
		itr.Seek(0)
	}
}

// Last positions the iterator on the last index.
func (itr *ListIterator[T]) Last() {
	if n := itr.list.Len(); n != 0 {
		itr.Seek(n - 1)
	}
}

// Seek moves the iterator position to the given index in the list.
func (itr *ListIterator[T]) Seek(index int) {
	if index < 0 || index >= itr.list.Len() {
		panic(fmt.Sprintf("immutable.ListIterator.Seek: index %d out of bounds", index))
	}
	itr.index = index
	itr.stack[0] = listIteratorElem[T]{node: itr.list.root}
	itr.depth = 0
	itr.seek(index)
}

// Next returns the current index and its value & moves the iterator forward.
func (itr *ListIterator[T]) Next() (index int, value T) {
	var empty T
	if itr.Done() {
		return -1, empty
	}
	// Handle slice node case
	if sliceNode, ok := itr.list.root.(*listSliceNode[T]); ok {
		index, value = itr.index, sliceNode.elements[itr.index]
		itr.index++
		return index, value
	}
	// Retrieve current index & value.
	elem := &itr.stack[itr.depth]
	index, value = itr.index, elem.node.(*listLeafNode[T]).children[elem.index]
	itr.index++
	if itr.Done() {
		return index, value
	}
	for ; itr.depth > 0 && itr.stack[itr.depth].index >= listNodeSize-1; itr.depth-- {
	}
	itr.seek(itr.index)
	return index, value
}

// Prev returns the current index and value and moves the iterator backward.
func (itr *ListIterator[T]) Prev() (index int, value T) {
	var empty T
	if itr.Done() {
		return -1, empty
	}
	if sliceNode, ok := itr.list.root.(*listSliceNode[T]); ok {
		index, value = itr.index, sliceNode.elements[itr.index]
		itr.index--
		return index, value
	}
	elem := &itr.stack[itr.depth]
	index, value = itr.index, elem.node.(*listLeafNode[T]).children[elem.index]
	itr.index--
	if itr.Done() {
		return index, value
	}
	for ; itr.depth > 0 && itr.stack[itr.depth].index == 0; itr.depth-- {
	}
	itr.seek(itr.index)
	return index, value
}

// seek positions the stack to the given index from the current depth.
func (itr *ListIterator[T]) seek(index int) {
	if _, ok := itr.list.root.(*listSliceNode[T]); ok {
		return
	}
	for {
		elem := &itr.stack[itr.depth]
		elem.index = ((itr.list.origin + index) >> (elem.node.depth() * listNodeBits)) & listNodeMask
		switch node := elem.node.(type) {
		case *listBranchNode[T]:
			child := node.children[elem.index]
			itr.stack[itr.depth+1] = listIteratorElem[T]{node: child}
			itr.depth++
		case *listLeafNode[T]:
			return
		}
	}
}

// listIteratorElem represents the node and it's child index within the stack.
type listIteratorElem[T any] struct {
	node  listNode[T]
	index int
}

// Constants for bit shifts used for levels in the List trie.
const (
	listNodeBits = 5
	listNodeSize = 1 << listNodeBits
	listNodeMask = listNodeSize - 1
)

// A list node can be a branch or a leaf.
type listNode[T any] interface {
	depth() uint
	get(index int) T
	set(index int, v T, mutable bool) listNode[T]
	containsBefore(index int) bool
	containsAfter(index int) bool
	deleteBefore(index int, mutable bool) listNode[T]
	deleteAfter(index int, mutable bool) listNode[T]
}

// newListNode returns a leaf node for depth zero, otherwise returns a branch node.
func newListNode[T any](depth uint) listNode[T] {
	if depth == 0 {
		return &listLeafNode[T]{}
	}
	return &listBranchNode[T]{d: depth}
}

// listBranchNode represents a branch of a List tree at a given depth.
type listBranchNode[T any] struct {
	d        uint // depth
	children [listNodeSize]listNode[T]
}

func (n *listBranchNode[T]) depth() uint { return n.d }

func (n *listBranchNode[T]) get(index int) T {
	idx := (index >> (n.d * listNodeBits)) & listNodeMask
	return n.children[idx].get(index)
}

func (n *listBranchNode[T]) set(index int, v T, mutable bool) listNode[T] {
	idx := (index >> (n.d * listNodeBits)) & listNodeMask
	child := n.children[idx]
	if child == nil {
		child = newListNode[T](n.depth() - 1)
	}
	var other *listBranchNode[T]
	if mutable {
		other = n
	} else {
		tmp := *n
		other = &tmp
	}
	other.children[idx] = child.set(index, v, mutable)
	return other
}

func (n *listBranchNode[T]) containsBefore(index int) bool {
	idx := (index >> (n.d * listNodeBits)) & listNodeMask
	for i := 0; i < idx; i++ {
		if n.children[i] != nil {
			return true
		}
	}
	if n.children[idx] != nil && n.children[idx].containsBefore(index) {
		return true
	}
	return false
}

func (n *listBranchNode[T]) containsAfter(index int) bool {
	idx := (index >> (n.d * listNodeBits)) & listNodeMask
	for i := idx + 1; i < len(n.children); i++ {
		if n.children[i] != nil {
			return true
		}
	}
	if n.children[idx] != nil && n.children[idx].containsAfter(index) {
		return true
	}
	return false
}

func (n *listBranchNode[T]) deleteBefore(index int, mutable bool) listNode[T] {
	if !n.containsBefore(index) {
		return n
	}
	idx := (index >> (n.d * listNodeBits)) & listNodeMask
	var other *listBranchNode[T]
	if mutable {
		other = n
		for i := 0; i < idx; i++ {
			n.children[i] = nil
		}
	} else {
		other = &listBranchNode[T]{d: n.d}
		copy(other.children[idx:][:], n.children[idx:][:])
	}
	if other.children[idx] != nil {
		other.children[idx] = other.children[idx].deleteBefore(index, mutable)
	}
	return other
}

func (n *listBranchNode[T]) deleteAfter(index int, mutable bool) listNode[T] {
	if !n.containsAfter(index) {
		return n
	}
	idx := (index >> (n.d * listNodeBits)) & listNodeMask
	var other *listBranchNode[T]
	if mutable {
		other = n
		for i := idx + 1; i < len(n.children); i++ {
			n.children[i] = nil
		}
	} else {
		other = &listBranchNode[T]{d: n.d}
		copy(other.children[:idx+1], n.children[:idx+1])
	}
	if other.children[idx] != nil {
		other.children[idx] = other.children[idx].deleteAfter(index, mutable)
	}
	return other
}

// listLeafNode represents a leaf node in a List.
type listLeafNode[T any] struct {
	children [listNodeSize]T
	occupied uint32 // bitset with ones at occupied positions, position 0 is the LSB
}

func (n *listLeafNode[T]) depth() uint { return 0 }

func (n *listLeafNode[T]) get(index int) T { return n.children[index&listNodeMask] }

func (n *listLeafNode[T]) set(index int, v T, mutable bool) listNode[T] {
	idx := index & listNodeMask
	var other *listLeafNode[T]
	if mutable {
		other = n
	} else {
		tmp := *n
		other = &tmp
	}
	other.children[idx] = v
	other.occupied |= 1 << idx
	return other
}

func (n *listLeafNode[T]) containsBefore(index int) bool {
	idx := index & listNodeMask
	return bits.TrailingZeros32(n.occupied) < idx
}

func (n *listLeafNode[T]) containsAfter(index int) bool {
	idx := index & listNodeMask
	lastSetPos := 31 - bits.LeadingZeros32(n.occupied)
	return lastSetPos > idx
}

func (n *listLeafNode[T]) deleteBefore(index int, mutable bool) listNode[T] {
	if !n.containsBefore(index) {
		return n
	}
	idx := index & listNodeMask
	var other *listLeafNode[T]
	if mutable {
		other = n
		var empty T
		for i := 0; i < idx; i++ {
			other.children[i] = empty
		}
	} else {
		other = &listLeafNode[T]{occupied: n.occupied}
		copy(other.children[idx:][:], n.children[idx:][:])
	}
	other.occupied &= ^((1 << idx) - 1)
	return other
}

func (n *listLeafNode[T]) deleteAfter(index int, mutable bool) listNode[T] {
	if !n.containsAfter(index) {
		return n
	}
	idx := index & listNodeMask
	var other *listLeafNode[T]
	if mutable {
		other = n
		var empty T
		for i := idx + 1; i < len(n.children); i++ {
			other.children[i] = empty
		}
	} else {
		other = &listLeafNode[T]{occupied: n.occupied}
		copy(other.children[:idx+1][:], n.children[:idx+1][:])
	}
	other.occupied &= (1 << (idx + 1)) - 1
	return other
}

// A list node which is implemented as a slice. Used for small lists.
type listSliceNode[T any] struct{ elements []T }

func (n *listSliceNode[T]) depth() uint     { return 0 }
func (n *listSliceNode[T]) get(index int) T { return n.elements[index] }

func (n *listSliceNode[T]) set(index int, v T, mutable bool) listNode[T] {
	if mutable {
		n.elements[index] = v
		return n
	}
	newElements := make([]T, len(n.elements))
	copy(newElements, n.elements)
	newElements[index] = v
	return &listSliceNode[T]{elements: newElements}
}

func (n *listSliceNode[T]) containsBefore(index int) bool                    { return true }
func (n *listSliceNode[T]) containsAfter(index int) bool                     { return true }
func (n *listSliceNode[T]) deleteBefore(index int, mutable bool) listNode[T] { return n }
func (n *listSliceNode[T]) deleteAfter(index int, mutable bool) listNode[T]  { return n }

// toTrie converts a listSliceNode to a trie-based structure.
func (n *listSliceNode[T]) toTrie(mutable bool) listNode[T] {
	numElements := len(n.elements)
	if numElements == 0 {
		return &listLeafNode[T]{}
	}
	var leaves []listNode[T]
	for i := 0; i < numElements; i += listNodeSize {
		end := i + listNodeSize
		if end > numElements {
			end = numElements
		}
		chunk := n.elements[i:end]
		leaf := &listLeafNode[T]{}
		copy(leaf.children[:], chunk)
		leaf.occupied = (uint32(1) << len(chunk)) - 1
		leaves = append(leaves, leaf)
	}
	nodes := leaves
	depth := uint(1)
	for len(nodes) > 1 {
		var parents []listNode[T]
		for i := 0; i < len(nodes); i += listNodeSize {
			end := i + listNodeSize
			if end > len(nodes) {
				end = len(nodes)
			}
			chunk := nodes[i:end]
			parent := &listBranchNode[T]{d: depth}
			copy(parent.children[:], chunk)
			parents = append(parents, parent)
		}
		nodes = parents
		depth++
	}
	return nodes[0]
}
