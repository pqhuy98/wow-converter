package blp

import (
	"os"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/formats/blp/cnative"
)

func nativeModeFromEnv() string {
	return strings.TrimSpace(os.Getenv("BLP_NATIVE"))
}

func nativeForcedOff() bool {
	switch nativeModeFromEnv() {
	case "0", "false", "off":
		return true
	default:
		return false
	}
}

// NativeEncoderAvailable reports whether the in-process C++ encoder is linked.
func NativeEncoderAvailable() bool {
	if nativeForcedOff() {
		return false
	}
	return cnative.Available()
}

func encodeNative(png []byte, blpPath string) error {
	if !NativeEncoderAvailable() {
		return cnative.ErrUnavailable
	}

	blpData, err := cnative.EncodePng(png)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dirOf(blpPath), 0o755); err != nil {
		// Match TS: ignore mkdir errors.
	}
	if err := os.WriteFile(blpPath, blpData, 0o644); err != nil {
		return err
	}
	return nil
}

func dirOf(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[:i]
	}
	return "."
}

// ShutdownNativePool is a no-op for the in-process C++ encoder.
func ShutdownNativePool() {}
