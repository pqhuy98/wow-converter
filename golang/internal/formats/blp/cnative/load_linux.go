//go:build linux

package cnative

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>

typedef int (*blp_encode_png_fn)(const unsigned char*, size_t, unsigned char**, size_t*);
typedef void (*blp_encode_free_fn)(unsigned char*);

static void* open_lib(const char* path) {
    return dlopen(path, RTLD_NOW);
}

static void* sym(void* handle, const char* name) {
    return dlsym(handle, name);
}

static void close_lib(void* handle) {
    if (handle != NULL) dlclose(handle);
}

static int call_encode(void* fn, const unsigned char* png, size_t png_len, unsigned char** out_buf, size_t* out_len) {
    return ((blp_encode_png_fn)fn)(png, png_len, out_buf, out_len);
}

static void call_free(void* fn, unsigned char* buf) {
    ((blp_encode_free_fn)fn)(buf);
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func platformLoad(path string) error {
	return loadLinux(path)
}

func loadLinux(path string) error {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	handle := C.open_lib(cpath)
	if handle == nil {
		return fmt.Errorf("dlopen %s failed", path)
	}

	encodeName := C.CString("blp_encode_png")
	defer C.free(unsafe.Pointer(encodeName))
	encodeSym := C.sym(handle, encodeName)
	freeSymName := C.CString("blp_encode_free")
	defer C.free(unsafe.Pointer(freeSymName))
	freeSym := C.sym(handle, freeSymName)
	if encodeSym == nil || freeSym == nil {
		C.close_lib(handle)
		return fmt.Errorf("missing exports in %s", path)
	}

	encodeFn = func(png *byte, pngLen uintptr, outBuf **byte, outLen *uintptr) uintptr {
		var cOut *C.uchar
		var cLen C.size_t
		rc := C.call_encode(
			encodeSym,
			(*C.uchar)(unsafe.Pointer(png)),
			C.size_t(pngLen),
			&cOut,
			&cLen,
		)
		*outBuf = (*byte)(unsafe.Pointer(cOut))
		*outLen = uintptr(cLen)
		return uintptr(rc)
	}
	freeFn = func(buf *byte) {
		C.call_free(freeSym, (*C.uchar)(unsafe.Pointer(buf)))
	}
	return nil
}
