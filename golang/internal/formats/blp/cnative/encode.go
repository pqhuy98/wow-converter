package cnative

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"github.com/pqhuy98/wow-converter/internal/workspace"
)

// ErrUnavailable is returned when the native encoder library is missing.
var ErrUnavailable = errors.New("native blp encoder unavailable")

type encodeFnType func(png *byte, pngLen uintptr, outBuf **byte, outLen *uintptr) uintptr
type freeFnType func(buf *byte)

var (
	loadOnce  sync.Once
	loaded    bool
	encodeFn  encodeFnType
	freeFn    freeFnType
	loadErr   error
)

func libFileName() string {
	if runtime.GOOS == "windows" {
		return "blpencode.dll"
	}
	return "libblpencode.so"
}

func candidateLibPaths() []string {
	name := libFileName()
	root := workspace.FindRepoRoot()
	paths := []string{
		filepath.Join(root, "bin", "blp-native", name),
	}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), name))
		paths = append(paths, filepath.Join(filepath.Dir(exe), "bin", "blp-native", name))
	}
	return paths
}

func loadLibrary() {
	for _, path := range candidateLibPaths() {
		if st, err := os.Stat(path); err != nil || st.IsDir() {
			continue
		}
		if err := loadFromPath(path); err == nil {
			loaded = true
			return
		}
	}
	loadErr = ErrUnavailable
}

func loadFromPath(path string) error {
	return platformLoad(path)
}

// Available reports whether the C++ encoder library is loaded.
func Available() bool {
	loadOnce.Do(loadLibrary)
	return loaded
}

// EncodePng runs the C++ PNG→BLP1 encoder from bin/blp-native.
func EncodePng(png []byte) ([]byte, error) {
	loadOnce.Do(loadLibrary)
	if !loaded {
		if loadErr != nil {
			return nil, loadErr
		}
		return nil, ErrUnavailable
	}
	if len(png) == 0 {
		return nil, errors.New("empty png input")
	}

	var outBuf *byte
	var outLen uintptr
	rc := encodeFn(&png[0], uintptr(len(png)), &outBuf, &outLen)
	if rc != 0 {
		return nil, fmt.Errorf("native blp encode failed (code %d)", rc)
	}
	if outBuf == nil || outLen == 0 {
		return nil, errors.New("native blp encode returned empty output")
	}

	out := unsafe.Slice(outBuf, outLen)
	copied := make([]byte, len(out))
	copy(copied, out)
	freeFn(outBuf)
	return copied, nil
}
