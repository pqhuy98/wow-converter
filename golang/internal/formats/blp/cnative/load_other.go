//go:build !windows && !linux

package cnative

import (
	"fmt"
	"runtime"
)

func platformLoad(path string) error {
	return fmt.Errorf("native blp encoder unsupported on %s", runtime.GOOS)
}
