package components

import (
	"math"
	"strconv"
)

func F(x *float64) string {
	num := 0.0
	if x != nil {
		num = *x
	}
	if math.Abs(num) > 999999 {
		num = math.Copysign(999999, num)
	}
	return strconv.FormatFloat(math.Round(num*10000)/10000, 'f', -1, 64)
}

func FVal(x float64) string {
	num := x
	if math.Abs(num) > 999999 {
		num = math.Copysign(999999, num)
	}
	rounded := math.Round(num*10000) / 10000
	// Match TS parseFloat(num.toFixed(4)).toString() which normalizes -0 to 0.
	s := strconv.FormatFloat(rounded, 'f', -1, 64)
	if s == "-0" {
		return "0"
	}
	return s
}

func FVector(vector []float64) string {
	parts := make([]string, len(vector))
	for i, v := range vector {
		parts[i] = FVal(v)
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}

func FVector3(v [3]float64) string {
	return FVector(v[:])
}
