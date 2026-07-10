package math

import "math"

func QuatNoRotation() QuaternionRotation {
	return QuaternionRotation{0, 0, 0, 1}
}

func Radians(deg float64) float64 {
	return deg * (math.Pi / 180)
}

func Degrees(rad float64) float64 {
	return rad * (180 / math.Pi)
}

func quaternionMultiply(q1, q2 QuaternionRotation) QuaternionRotation {
	x1, y1, z1, w1 := q1[0], q1[1], q1[2], q1[3]
	x2, y2, z2, w2 := q2[0], q2[1], q2[2], q2[3]

	x := w1*x2 + x1*w2 + y1*z2 - z1*y2
	y := w1*y2 - x1*z2 + y1*w2 + z1*x2
	z := w1*z2 + x1*y2 - y1*x2 + z1*w2
	w := w1*w2 - x1*x2 - y1*y2 - z1*z2

	return QuaternionRotation{x, y, z, w}
}

func quaternionNormalize(q QuaternionRotation) QuaternionRotation {
	x, y, z, w := q[0], q[1], q[2], q[3]
	magnitude := math.Sqrt(x*x + y*y + z*z + w*w)
	if magnitude == 0 {
		panic("cannot normalize a zero-length quaternion")
	}
	return QuaternionRotation{x / magnitude, y / magnitude, z / magnitude, w / magnitude}
}

// CalculateChildAbsoluteEulerRotation combines parent and child euler rotations.
func CalculateChildAbsoluteEulerRotation(parentEuler, childRelativeEuler EulerRotation) EulerRotation {
	parentQuat := EulerToQuaternion(parentEuler)
	childRelativeQuat := EulerToQuaternion(childRelativeEuler)
	combinedQuat := quaternionNormalize(quaternionMultiply(parentQuat, childRelativeQuat))
	return QuaternionToEuler(combinedQuat)
}

// QuaternionToEuler converts a quaternion to euler angles (radians).
func QuaternionToEuler(quat QuaternionRotation) EulerRotation {
	x, y, z, w := quat[0], quat[1], quat[2], quat[3]

	sinrCosp := 2 * (w*x + y*z)
	cosrCosp := 1 - 2*(x*x+y*y)
	roll := math.Atan2(sinrCosp, cosrCosp)

	sinp := 2 * (w*y - z*x)
	var pitch float64
	if math.Abs(sinp) >= 1 {
		pitch = math.Copysign(math.Pi/2, sinp)
	} else {
		pitch = math.Asin(sinp)
	}

	sinyCosp := 2 * (w*z + x*y)
	cosyCosp := 1 - 2*(y*y+z*z)
	yaw := math.Atan2(sinyCosp, cosyCosp)

	return EulerRotation{roll, pitch, yaw}
}

// EulerToQuaternion converts euler angles (radians) to a quaternion.
func EulerToQuaternion(eulerRad EulerRotation) QuaternionRotation {
	halfRoll := eulerRad[0] / 2
	halfPitch := eulerRad[1] / 2
	halfYaw := eulerRad[2] / 2

	sinRoll := math.Sin(halfRoll)
	cosRoll := math.Cos(halfRoll)
	sinPitch := math.Sin(halfPitch)
	cosPitch := math.Cos(halfPitch)
	sinYaw := math.Sin(halfYaw)
	cosYaw := math.Cos(halfYaw)

	x := sinRoll*cosPitch*cosYaw - cosRoll*sinPitch*sinYaw
	y := cosRoll*sinPitch*cosYaw + sinRoll*cosPitch*sinYaw
	z := cosRoll*cosPitch*sinYaw - sinRoll*sinPitch*cosYaw
	w := cosRoll*cosPitch*cosYaw + sinRoll*sinPitch*sinYaw

	return QuaternionRotation{x, y, z, w}
}
