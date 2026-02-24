package immutable

import (
	"testing"
)

func TestSetsPut(t *testing.T) {
	s := NewSet[string](nil)
	s2 := s.Add("1").Add("1")
	s2.Add("2")
	if s.Len() != 0 {
		t.Fatalf("Unexpected mutation of set")
	}
	if s.Has("1") {
		t.Fatalf("Unexpected set element")
	}
	if s2.Len() != 1 {
		t.Fatalf("Unexpected non-mutation of set")
	}
	if !s2.Has("1") {
		t.Fatalf("Set element missing")
	}
	itr := s2.Iterator()
	counter := 0
	for !itr.Done() {
		i, v := itr.Next()
		t.Log(i, v)
		counter++
	}
	if counter != 1 {
		t.Fatalf("iterator wrong length")
	}
}

func TestSetsDelete(t *testing.T) {
	s := NewSet[string](nil)
	s2 := s.Add("1")
	s3 := s.Delete("1")
	if s2.Len() != 1 {
		t.Fatalf("Unexpected non-mutation of set")
	}
	if !s2.Has("1") {
		t.Fatalf("Set element missing")
	}
	if s3.Len() != 0 {
		t.Fatalf("Unexpected set length after delete")
	}
	if s3.Has("1") {
		t.Fatalf("Unexpected set element after delete")
	}
}

func TestSortedSetsPut(t *testing.T) {
	s := NewSortedSet[string](nil)
	s2 := s.Add("1").Add("1").Add("0")
	if s.Len() != 0 {
		t.Fatalf("Unexpected mutation of set")
	}
	if s.Has("1") {
		t.Fatalf("Unexpected set element")
	}
	if s2.Len() != 2 {
		t.Fatalf("Unexpected non-mutation of set")
	}
	if !s2.Has("1") {
		t.Fatalf("Set element missing")
	}

	itr := s2.Iterator()
	counter := 0
	for !itr.Done() {
		i, v := itr.Next()
		t.Log(i, v)
		if counter == 0 && i != "0" {
			t.Fatalf("sort did not work for first el")
		}
		if counter == 1 && i != "1" {
			t.Fatalf("sort did not work for second el")
		}
		counter++
	}
	if counter != 2 {
		t.Fatalf("iterator wrong length")
	}
}

func TestSortedSetsDelete(t *testing.T) {
	s := NewSortedSet[string](nil)
	s2 := s.Add("1")
	s3 := s.Delete("1")
	if s2.Len() != 1 {
		t.Fatalf("Unexpected non-mutation of set")
	}
	if !s2.Has("1") {
		t.Fatalf("Set element missing")
	}
	if s3.Len() != 0 {
		t.Fatalf("Unexpected set length after delete")
	}
	if s3.Has("1") {
		t.Fatalf("Unexpected set element after delete")
	}
}

func TestSetBuilder(t *testing.T) {
	b := NewSetBuilder[string](nil)
	b.Add("foo")
	b.Add("bar")
	b.Add("foo") // duplicate

	if b.Len() != 2 {
		t.Fatalf("expected Len()=2, got %d", b.Len())
	}
	if !b.Has("foo") || !b.Has("bar") {
		t.Fatalf("expected Has to return true for added values")
	}

	b.Delete("bar")
	if b.Len() != 1 {
		t.Fatalf("expected Len()=1 after delete, got %d", b.Len())
	}

	s := b.Set()
	if s.Len() != 1 {
		t.Fatalf("expected set length 1, got %d", s.Len())
	}
	if !s.Has("foo") {
		t.Fatalf("expected set to contain foo")
	}
	if s.Has("bar") {
		t.Fatalf("expected set to not contain bar")
	}
}

func TestSetBuilder_Invalidation(t *testing.T) {
	b := NewSetBuilder[int](nil)
	b.Add(1)
	_ = b.Set()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on double Set() call")
		}
	}()
	_ = b.Set()
}

func TestSortedSetBuilder(t *testing.T) {
	b := NewSortedSetBuilder[string](nil)
	b.Add("test3")
	b.Add("test1")
	b.Add("test2")

	s := b.SortedSet()
	items := s.Items()

	if len(items) != 3 {
		t.Fatalf("Set has wrong number of items")
	}
	if items[0] != "test1" {
		t.Fatalf("First item incorrectly sorted")
	}
	if items[1] != "test2" {
		t.Fatalf("Second item incorrectly sorted")
	}
	if items[2] != "test3" {
		t.Fatalf("Third item incorrectly sorted")
	}
}

func TestSortedSetBuilder_Invalidation(t *testing.T) {
	b := NewSortedSetBuilder[int](nil)
	b.Add(1)
	_ = b.SortedSet()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on double SortedSet() call")
		}
	}()
	_ = b.SortedSet()
}
