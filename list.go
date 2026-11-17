package immutable

import (
	"fmt"
	"math/bits"
	"reflect"
)

// List is a dense, ordered, indexed collections. They are analogous to slices
// in Go. A List is implemented as a relaxed-radix-balanced tree (a radix-32
// trie addressed by position) with a tail: the last up to 32 values are kept in
// a slice next to the trie, so appending to a list copies at most 32 values
// instead of a path through the trie, and a list of up to 32 values is just that
// slice. The zero value of a List is an empty list. A list is safe for
// concurrent use.
//
// The representation, and the rules every operation keeps (invariants I1-I4),
// are described in memory-bank/ADRs/ADR-005-list-tail-buffer.md:
//
//   - the trie holds the first size-len(tail) values, at trie indexes origin+i,
//     and is nil when there are none (then origin is 0);
//   - len(tail) is at most listNodeSize;
//   - when there is a trie, it ends on a leaf boundary, so a full tail is exactly
//     one leaf and moves into the trie without being split;
//   - a published list is never changed, tail array included, so an Append
//     makes a new tail instead of writing into spare capacity.
type List[T any] struct {
	root   listNode[T] // trie over the first size-len(tail) values; nil if there are none
	tail   []T         // the last 0 to listNodeSize values
	origin int         // trie index of element 0; 0 when root is nil
	size   int         // total number of elements: trie and tail
}

// NewList returns a new instance of List holding values.
func NewList[T any](values ...T) *List[T] { return newListFromValues(values) }

// newListFromValues returns a list holding a copy of values; the caller keeps
// ownership of the slice. Up to listNodeSize values are just a tail. Larger
// inputs are built bottom-up in a single O(n) pass (see buildListTrie) instead
// of appending one value at a time: whole leaves go into the trie and the last
// leaf, one to listNodeSize values, becomes the tail.
func newListFromValues[T any](values []T) *List[T] {
	if len(values) == 0 {
		return &List[T]{}
	}
	split := (len(values) - 1) / listNodeSize * listNodeSize // values in the trie
	l := &List[T]{tail: make([]T, len(values)-split), size: len(values)}
	copy(l.tail, values[split:])
	if split > 0 {
		l.root = buildListTrie(values[:split])
	}
	return l
}

// trieLen returns the number of values held by the trie; the rest are in the tail.
func (l *List[T]) trieLen() int { return l.size - len(l.tail) }

// clone returns a copy of the list.
func (l *List[T]) clone() *List[T] {
	other := *l
	return &other
}

// Len returns the number of elements in the list.
func (l *List[T]) Len() int { return l.size }

// cap returns the number of indexes the current root can address: a leaf holds
// listNodeSize values and every level above it multiplies that by listNodeSize,
// so a root of depth d covers 32^(d+1) indexes. append adds a level when the next
// index is no longer below this value.
func (l *List[T]) cap() int { return 1 << ((l.root.depth() + 1) * listNodeBits) }

// Get returns the value at the given index. Similar to slices, this method will
// panic if index is below zero or is greater than or equal to the list size.
func (l *List[T]) Get(index int) T {
	if index < 0 || index >= l.size {
		panic(fmt.Sprintf("immutable.List.Get: index %d out of bounds", index))
	}
	if n := l.trieLen(); index >= n {
		return l.tail[index-n]
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
	mode := valueEqualityMode[T]()
	for _, v := range l.tail {
		if valuesEqual(mode, v, value) {
			return true
		}
	}
	if l.root == nil {
		return false
	}
	itr := l.Iterator()
	for n := l.trieLen(); itr.index < n; {
		_, v := itr.Next()
		if valuesEqual(mode, v, value) {
			return true
		}
	}
	return false
}

// equalityMode says how Contains compares two values of type T.
type equalityMode uint8

const (
	equalByOperator equalityMode = iota // concrete comparable type: ==
	equalDeep                           // concrete non-comparable type: reflect.DeepEqual
	equalByDynamic                      // interface type: decided per value
)

// valueEqualityMode picks the comparison mode from the static type T, once per
// call instead of once per element.
func valueEqualityMode[T any]() equalityMode {
	t := reflect.TypeFor[T]()
	switch {
	case t.Kind() == reflect.Interface:
		return equalByDynamic
	case t.Comparable():
		return equalByOperator
	default:
		return equalDeep
	}
}

// valuesEqual compares a and b according to mode.
func valuesEqual[T any](mode equalityMode, a, b T) bool {
	switch mode {
	case equalByOperator:
		return any(a) == any(b)
	case equalDeep:
		return reflect.DeepEqual(a, b)
	}
	// Interface element type: comparability depends on the dynamic value.
	if ta := reflect.TypeOf(a); ta != nil && ta.Comparable() {
		return any(a) == any(b)
	}
	return reflect.DeepEqual(a, b)
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
	for _, v := range l.tail {
		if equal(v, value) {
			return true
		}
	}
	if l.root == nil {
		return false
	}
	itr := l.Iterator()
	for n := l.trieLen(); itr.index < n; {
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
	other := l
	if !mutable {
		other = l.clone()
	}
	if n := l.trieLen(); index >= n {
		if !mutable {
			other.tail = cloneSlice(l.tail, len(l.tail))
		}
		other.tail[index-n] = value
		return other
	}
	other.root = l.root.set(l.origin+index, value, mutable)
	return other
}

// cloneSlice returns a copy of s with room for exactly n values (n >= len(s)).
// Persistent operations use it instead of append so a published tail never has
// spare capacity that a later operation could write into.
func cloneSlice[T any](s []T, n int) []T {
	out := make([]T, n)
	copy(out, s)
	return out
}

// Append returns a new list with value added to the end of the list.
func (l *List[T]) Append(value T) *List[T] { return l.append(value, false) }

func (l *List[T]) append(value T, mutable bool) *List[T] {
	other := l
	if !mutable {
		other = l.clone()
	}
	if len(other.tail) == listNodeSize {
		other.pushTail(mutable)
	}
	if mutable {
		other.tail = append(other.tail, value)
	} else {
		other.tail = cloneSlice(other.tail, len(other.tail)+1)
		other.tail[len(other.tail)-1] = value
	}
	other.size++
	return other
}

// pushTail moves a full tail into the trie as one leaf. l must be owned by the
// caller: a fresh clone, or the list of a builder. The leaf goes at trie index
// origin+trieLen, which is a leaf boundary because the trie ends on one.
func (l *List[T]) pushTail(mutable bool) {
	leaf := &listLeafNode[T]{occupied: ^uint32(0)}
	copy(leaf.children[:], l.tail)
	if l.root == nil {
		l.root, l.origin = leaf, 0
	} else {
		index := l.origin + l.trieLen()
		// Expand the trie to the right if no slots remain.
		if index >= l.cap() {
			newRoot := &listBranchNode[T]{d: l.root.depth() + 1}
			newRoot.children[0] = l.root
			l.root = newRoot
		}
		l.root = l.root.setLeaf(index, leaf, mutable)
	}
	if mutable {
		clear(l.tail) // the values live in the leaf now; don't keep them reachable twice
		l.tail = l.tail[:0]
	} else {
		l.tail = nil
	}
	// The leaf holds the values that were in the tail, so size is unchanged.
}

// Prepend returns a new list with value(s) added to the beginning of the list.
func (l *List[T]) Prepend(value T) *List[T] { return l.prepend(value, false) }

func (l *List[T]) prepend(value T, mutable bool) *List[T] {
	other := l
	if !mutable {
		other = l.clone()
	}
	if l.root == nil {
		// There is no trie, only a tail.
		if len(l.tail) < listNodeSize {
			tail := make([]T, len(l.tail)+1)
			tail[0] = value
			copy(tail[1:], l.tail)
			other.tail = tail
			other.size++
			return other
		}
		// A full tail and one more value: the first listNodeSize values become
		// the trie's only leaf and the last value stays in the tail.
		leaf := &listLeafNode[T]{occupied: ^uint32(0)}
		leaf.children[0] = value
		copy(leaf.children[1:], l.tail[:listNodeSize-1])
		last := l.tail[listNodeSize-1]
		other.root, other.origin = leaf, 0
		other.tail = []T{last}
		other.size++
		return other
	}
	// Expand the trie to the left if no slots remain.
	if other.origin == 0 {
		newRoot := &listBranchNode[T]{d: other.root.depth() + 1}
		newRoot.children[listNodeSize-1] = other.root
		other.root = newRoot
		other.origin += (listNodeSize - 1) << (other.root.depth() * listNodeBits)
	}
	// Increase size and move origin back. Update first element to value. The
	// trie's right end does not move: origin falls by as much as its length grows.
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
	trieLen := l.trieLen()
	// Only values from the tail remain. They are copied, not shared, so the
	// values before start and after end can be collected.
	if start >= trieLen {
		return &List[T]{tail: cloneSlice(l.tail[start-trieLen:end-trieLen], end-start), size: end - start}
	}

	// Work out the new tail and where the part of the result in the trie ends.
	trieEnd := min(end, trieLen)
	var tail []T
	switch {
	case end > trieLen:
		// The result keeps the front of the old tail. The array is shared only
		// when nothing in it is cut off.
		if k := end - trieLen; k == len(l.tail) {
			tail = l.tail
		} else {
			tail = cloneSlice(l.tail[:k], k)
		}
	case (l.origin+end)&listNodeMask != 0:
		// The result ends inside a leaf. Move the values from the start of that
		// leaf to end into the tail, so the trie keeps ending on a leaf boundary.
		from := max(start, end-(l.origin+end)&listNodeMask)
		tail = make([]T, end-from)
		for i := range tail {
			tail[i] = l.root.get(l.origin + from + i)
		}
		trieEnd = from
	}

	other := l
	if !mutable {
		other = l.clone()
	}
	other.tail = tail
	other.size = end - start
	if trieEnd <= start { // nothing is left in the trie
		other.root, other.origin = nil, 0
		return other
	}
	// Update origin and the trie's length.
	other.origin = l.origin + start
	trieSize := trieEnd - start
	// Contract tree while the start & end are in the same child node.
	for other.root.depth() > 1 {
		i := (other.origin >> (other.root.depth() * listNodeBits)) & listNodeMask
		j := ((other.origin + trieSize - 1) >> (other.root.depth() * listNodeBits)) & listNodeMask
		if i != j {
			break
		}
		// Replace the current root with the single child & update origin offset.
		other.origin -= i << (other.root.depth() * listNodeBits)
		other.root = other.root.(*listBranchNode[T]).children[i]
	}
	// Ensure all references are removed before start & after end.
	other.root = other.root.deleteBefore(other.origin, mutable)
	other.root = other.root.deleteAfter(other.origin+trieSize-1, mutable)
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
	if index < itr.list.trieLen() { // values in the tail need no position in the trie
		itr.stack[0] = listIteratorElem[T]{node: itr.list.root}
		itr.depth = 0
		itr.seek(index)
	}
}

// Next returns the current index and its value & moves the iterator forward.
func (itr *ListIterator[T]) Next() (index int, value T) {
	var empty T
	if itr.Done() {
		return -1, empty
	}
	list := itr.list
	trieLen := list.trieLen()
	// The tail needs no stack: it is a slice.
	if itr.index >= trieLen {
		index, value = itr.index, list.tail[itr.index-trieLen]
		itr.index++
		return index, value
	}
	// Retrieve current index & value.
	elem := &itr.stack[itr.depth]
	index, value = itr.index, elem.node.(*listLeafNode[T]).children[elem.index]
	itr.index++
	if itr.index >= trieLen { // the trie is used up; the rest, if any, is the tail
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
	list := itr.list
	trieLen := list.trieLen()
	if itr.index >= trieLen {
		index, value = itr.index, list.tail[itr.index-trieLen]
		itr.index--
		if itr.index == trieLen-1 && itr.index >= 0 {
			// Stepped back from the tail into the trie: the stack has not been
			// kept for the tail, so position it from the root.
			itr.stack[0] = listIteratorElem[T]{node: list.root}
			itr.depth = 0
			itr.seek(itr.index)
		}
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
	setLeaf(index int, leaf *listLeafNode[T], mutable bool) listNode[T]
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

// setLeaf puts leaf in the slot that holds index, copying the path to it unless
// mutable. Nodes that do not exist yet are made here and so are never copied
// again.
func (n *listBranchNode[T]) setLeaf(index int, leaf *listLeafNode[T], mutable bool) listNode[T] {
	idx := (index >> (n.d * listNodeBits)) & listNodeMask
	var other *listBranchNode[T]
	if mutable {
		other = n
	} else {
		tmp := *n
		other = &tmp
	}
	if n.d == 1 {
		other.children[idx] = leaf
		return other
	}
	child := n.children[idx]
	if child == nil {
		other.children[idx] = (&listBranchNode[T]{d: n.d - 1}).setLeaf(index, leaf, true)
		return other
	}
	other.children[idx] = child.setLeaf(index, leaf, mutable)
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

// setLeaf is never reached: a leaf is the bottom of the trie, and the trie is
// widened before a leaf is added to a root that is itself a leaf.
func (n *listLeafNode[T]) setLeaf(index int, leaf *listLeafNode[T], mutable bool) listNode[T] {
	panic("immutable.List: setLeaf on a leaf node")
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

// buildListTrie builds a trie holding a copy of elements, with the first
// element at index 0 (origin 0). It copies the values into full 32-wide leaves
// and joins them with wireListLeaves. It does one allocation per node and never
// copies a path, so it is O(n).
func buildListTrie[T any](elements []T) listNode[T] {
	if len(elements) == 0 {
		return nil
	}
	leaves := make([]listNode[T], 0, (len(elements)+listNodeMask)/listNodeSize)
	for i := 0; i < len(elements); i += listNodeSize {
		chunk := elements[i:min(i+listNodeSize, len(elements))]
		leaf := &listLeafNode[T]{occupied: uint32(1)<<len(chunk) - 1}
		copy(leaf.children[:], chunk)
		leaves = append(leaves, leaf)
	}
	return wireListLeaves(leaves)
}

// wireListLeaves joins leaves into one trie by building each level of branch
// nodes from the level below, until a single root remains. leaves must not be
// empty, and every leaf except the last must be full; the last leaf's occupied
// mask must already be set. The result has the minimum depth for the number of
// leaves: the leaf itself for one leaf, a depth-1 branch up to 32 leaves, and so
// on. Only the last node of each level can be partly filled and unused children
// stay nil, which is the same shape a sequence of appends produces.
//
// The slice is used as scratch space and its contents are overwritten.
func wireListLeaves[T any](nodes []listNode[T]) listNode[T] {
	for depth := uint(1); len(nodes) > 1; depth++ {
		// Parent j is written after children [32j, 32j+32) were read, and
		// j <= 32j, so nothing unread is overwritten.
		parents := nodes[:0]
		for i := 0; i < len(nodes); i += listNodeSize {
			parent := &listBranchNode[T]{d: depth}
			copy(parent.children[:], nodes[i:min(i+listNodeSize, len(nodes))])
			parents = append(parents, parent)
		}
		nodes = parents
	}
	return nodes[0]
}
