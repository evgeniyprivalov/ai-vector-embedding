package slice

import (
	"math"
	"testing"
)

func almostEqual(a, b float32, eps float32) bool {
	return float32(math.Abs(float64(a-b))) <= eps
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
	}{
		{
			name:     "empty slices",
			a:        []float32{},
			b:        []float32{},
			expected: 0,
		},
		{
			name:     "different lengths",
			a:        []float32{1, 2},
			b:        []float32{1},
			expected: 0,
		},
		{
			name:     "both zero vectors",
			a:        []float32{0, 0, 0},
			b:        []float32{0, 0, 0},
			expected: 0,
		},
		{
			name:     "identical vectors",
			a:        []float32{1, 2, 3},
			b:        []float32{1, 2, 3},
			expected: 1,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0},
			b:        []float32{0, 1},
			expected: 0,
		},
		{
			name:     "opposite direction vectors",
			a:        []float32{1, 0},
			b:        []float32{-1, 0},
			expected: -1,
		},
		{
			name:     "general case",
			a:        []float32{1, 2, 3},
			b:        []float32{4, 5, 6},
			expected: 0.9746318, // approximate
		},
	}

	const eps = 1e-5

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CosineSimilarity(tt.a, tt.b)
			if !almostEqual(got, tt.expected, eps) {
				t.Errorf("CosineSimilarity(%v, %v) = %v; want %v",
					tt.a, tt.b, got, tt.expected)
			}
		})
	}
}
