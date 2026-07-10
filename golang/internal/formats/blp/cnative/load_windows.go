//go:build windows

package cnative

import (
	"syscall"
	"unsafe"
)

func platformLoad(path string) error {
	return loadWindows(path)
}

func loadWindows(path string) error {
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return err
	}

	encodeProc, err := dll.FindProc("blp_encode_png")
	if err != nil {
		_ = dll.Release()
		return err
	}
	freeProc, err := dll.FindProc("blp_encode_free")
	if err != nil {
		_ = dll.Release()
		return err
	}

	encodeFn = func(png *byte, pngLen uintptr, outBuf **byte, outLen *uintptr) uintptr {
		r0, _, _ := encodeProc.Call(
			uintptr(unsafe.Pointer(png)),
			pngLen,
			uintptr(unsafe.Pointer(outBuf)),
			uintptr(unsafe.Pointer(outLen)),
		)
		return r0
	}
	freeFn = func(buf *byte) {
		_, _, _ = freeProc.Call(uintptr(unsafe.Pointer(buf)))
	}
	return nil
}
