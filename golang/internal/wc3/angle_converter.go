package wc3

import "math"

// Angle is an angle in degrees (0 <= angle < 360).
type Angle float32

// Deg2Rad converts degrees to radians.
func Deg2Rad(deg float32) float32 {
	return deg * math.Pi / 180
}

// Rad2Deg converts radians to degrees.
func Rad2Deg(rad float32) float32 {
	return rad * 180 / math.Pi
}

// AngleToRadians converts WC3 angle units to radians.
func AngleToRadians(a Angle) float64 {
	return float64(Deg2Rad(float32(a)))
}

// RadiansToAngle converts radians to WC3 angle units.
func RadiansToAngle(rad float64) Angle {
	return Angle(Rad2Deg(float32(rad)))
}
