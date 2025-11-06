package provider

import "math"

func round6(x float64) float64 {
	return math.Round(x*1e6) / 1e6
}

func f32ptrToTF64(v *float32) float64 {
	if v == nil {
		return 0
	}
	return round6(float64(*v))
}
