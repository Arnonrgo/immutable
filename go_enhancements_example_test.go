package immutable

import (
	"cmp"
	"testing"
)

// Test demonstrating Go 1.21+ built-in min/max usage
func TestBuiltinMinMax(t *testing.T) {
	// Test built-in min function
	result := min(10, 5, 15)
	if result != 5 {
		t.Errorf("min(10, 5, 15) = %d, want 5", result)
	}

	// Test built-in max function
	result = max(10, 5, 15)
	if result != 15 {
		t.Errorf("max(10, 5, 15) = %d, want 15", result)
	}

	// Test with single argument
	single := min(42)
	if single != 42 {
		t.Errorf("min(42) = %d, want 42", single)
	}
}

// Test demonstrating cmp.Ordered usage instead of constraints.Ordered
func TestCmpOrderedReplacement(t *testing.T) {
	// Test that our defaultCompare function works with cmp.Ordered
	result := defaultCompare(5, 3)
	if result != 1 {
		t.Errorf("defaultCompare(5, 3) = %d, want 1", result)
	}

	result = defaultCompare(3, 5)
	if result != -1 {
		t.Errorf("defaultCompare(3, 5) = %d, want -1", result)
	}

	result = defaultCompare(5, 5)
	if result != 0 {
		t.Errorf("defaultCompare(5, 5) = %d, want 0", result)
	}
}

// Test built-in clear function
func TestBuiltinClear(t *testing.T) {
	// Test clear on slice
	slice := []int{1, 2, 3, 4, 5}
	clear(slice)

	// Verify all elements are zero
	for i, v := range slice {
		if v != 0 {
			t.Errorf("slice[%d] = %d after clear, want 0", i, v)
		}
	}

	// Test clear on map
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	clear(m)

	if len(m) != 0 {
		t.Errorf("map length = %d after clear, want 0", len(m))
	}
}

// Demonstration of utility function using built-in min/max
func optimalNodeSize(entriesCount, maxSize, minSize int) int {
	return min(max(entriesCount, minSize), maxSize)
}

func TestOptimalNodeSize(t *testing.T) {
	tests := []struct {
		entries, maxSize, minSize, want int
	}{
		{15, 32, 4, 15}, // entries within bounds
		{2, 32, 4, 4},   // entries below minimum
		{50, 32, 4, 32}, // entries above maximum
		{10, 20, 8, 10}, // normal case
	}

	for _, tt := range tests {
		got := optimalNodeSize(tt.entries, tt.maxSize, tt.minSize)
		if got != tt.want {
			t.Errorf("optimalNodeSize(%d, %d, %d) = %d, want %d",
				tt.entries, tt.maxSize, tt.minSize, got, tt.want)
		}
	}
}

// Test using cmp.Compare directly (Go 1.21+)
func TestCmpCompare(t *testing.T) {
	result := cmp.Compare(5, 3)
	if result != 1 {
		t.Errorf("cmp.Compare(5, 3) = %d, want 1", result)
	}

	result = cmp.Compare("apple", "banana")
	if result != -1 {
		t.Errorf("cmp.Compare('apple', 'banana') = %d, want -1", result)
	}

	result = cmp.Compare(3.14, 3.14)
	if result != 0 {
		t.Errorf("cmp.Compare(3.14, 3.14) = %d, want 0", result)
	}
}

// Test that our hasher works with the new cmp.Ordered constraint
func TestHasherWithCmpOrdered(t *testing.T) {
	// Test with different ordered types that have built-in hashers
	intHasher := NewHasher(42)
	if intHasher == nil {
		t.Error("NewHasher(int) returned nil")
	}

	stringHasher := NewHasher("test")
	if stringHasher == nil {
		t.Error("NewHasher(string) returned nil")
	}

	// Note: float64 doesn't have a built-in hasher in our implementation
	// so we skip that test
}
