package math

import "math"

func V3All(value float64) Vector3 {
	return Vector3{value, value, value}
}

func V3Sum(a, b Vector3) Vector3 {
	return Vector3{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

func V3Sub(a, b Vector3) Vector3 {
	return Vector3{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
}

func V3Mean(a, b Vector3) Vector3 {
	return Vector3{(a[0] + b[0]) / 2, (a[1] + b[1]) / 2, (a[2] + b[2]) / 2}
}

func V3Negative(a Vector3) Vector3 {
	return Vector3{-a[0], -a[1], -a[2]}
}

func V3Scale(a Vector3, b float64) Vector3 {
	return Vector3{a[0] * b, a[1] * b, a[2] * b}
}

func V3Mul(a, b Vector3) Vector3 {
	return Vector3{a[0] * b[0], a[1] * b[1], a[2] * b[2]}
}

func V3Rotate(v Vector3, eulerAngleRadians Vector3) Vector3 {
	return RotateVector(v, eulerAngleRadians)
}

func V3Normalize(v Vector3) Vector3 {
	mag := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	if mag == 0 {
		return Vector3{}
	}
	return Vector3{v[0] / mag, v[1] / mag, v[2] / mag}
}

func V3Dot(a, b Vector3) float64 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

func V3Min(a, b Vector3) Vector3 {
	return Vector3{math.Min(a[0], b[0]), math.Min(a[1], b[1]), math.Min(a[2], b[2])}
}

func V3Max(a, b Vector3) Vector3 {
	return Vector3{math.Max(a[0], b[0]), math.Max(a[1], b[1]), math.Max(a[2], b[2])}
}

func V3Distance(a, b Vector3) float64 {
	dx, dy, dz := a[0]-b[0], a[1]-b[1], a[2]-b[2]
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func V3Lerp(v0, v1 Vector3, t float64) Vector3 {
	return Vector3{
		v0[0] + t*(v1[0]-v0[0]),
		v0[1] + t*(v1[1]-v0[1]),
		v0[2] + t*(v1[2]-v0[2]),
	}
}

func V3LerpScalar(v0, v1, t float64) float64 {
	return v0 + t*(v1-v0)
}

func V3NoInterpScalar(v0, _, _ float64) float64 {
	return v0
}

func V3NoInterp[T ~[]float64](v0 T, _, _ float64) T {
	out := make(T, len(v0))
	copy(out, v0)
	return out
}

func V3Slerp(q0, q1 QuaternionRotation, t float64) QuaternionRotation {
	dot := q0[0]*q1[0] + q0[1]*q1[1] + q0[2]*q1[2] + q0[3]*q1[3]
	dot = math.Max(-1.0, math.Min(1.0, dot))

	if dot < 0.0 {
		q1 = QuaternionRotation{-q1[0], -q1[1], -q1[2], -q1[3]}
		dot = -dot
	}

	const dotThreshold = 0.9995
	if dot > dotThreshold {
		result := QuaternionRotation{
			q0[0] + t*(q1[0]-q0[0]),
			q0[1] + t*(q1[1]-q0[1]),
			q0[2] + t*(q1[2]-q0[2]),
			q0[3] + t*(q1[3]-q0[3]),
		}
		mag := math.Sqrt(result[0]*result[0] + result[1]*result[1] + result[2]*result[2] + result[3]*result[3])
		return QuaternionRotation{result[0] / mag, result[1] / mag, result[2] / mag, result[3] / mag}
	}

	theta0 := math.Acos(dot)
	theta := theta0 * t
	sinTheta := math.Sin(theta)
	sinTheta0 := math.Sin(theta0)

	s0 := math.Cos(theta) - dot*sinTheta/sinTheta0
	s1 := sinTheta / sinTheta0

	return QuaternionRotation{
		(s0 * q0[0]) + (s1 * q1[0]),
		(s0 * q0[1]) + (s1 * q1[1]),
		(s0 * q0[2]) + (s1 * q1[2]),
		(s0 * q0[3]) + (s1 * q1[3]),
	}
}

func RotateVector(position Vector3, angle EulerRotation) Vector3 {
	x, y, z := position[0], position[1], position[2]
	rx, ry, rz := angle[0], angle[1], angle[2]

	rxM := [3][3]float64{{1, 0, 0}, {0, math.Cos(rx), -math.Sin(rx)}, {0, math.Sin(rx), math.Cos(rx)}}
	ryM := [3][3]float64{{math.Cos(ry), 0, math.Sin(ry)}, {0, 1, 0}, {-math.Sin(ry), 0, math.Cos(ry)}}
	rzM := [3][3]float64{{math.Cos(rz), -math.Sin(rz), 0}, {math.Sin(rz), math.Cos(rz), 0}, {0, 0, 1}}

	multiply := func(matrix [3][3]float64, vector Vector3) Vector3 {
		return Vector3{
			matrix[0][0]*vector[0] + matrix[0][1]*vector[1] + matrix[0][2]*vector[2],
			matrix[1][0]*vector[0] + matrix[1][1]*vector[1] + matrix[1][2]*vector[2],
			matrix[2][0]*vector[0] + matrix[2][1]*vector[1] + matrix[2][2]*vector[2],
		}
	}

	v1 := multiply(rxM, Vector3{x, y, z})
	v2 := multiply(ryM, v1)
	return multiply(rzM, v2)
}
