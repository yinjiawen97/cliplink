//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	clipUser32           = syscall.MustLoadDLL("user32")
	clipKernel32         = syscall.MustLoadDLL("kernel32")
	clipOpenClipboard    = clipUser32.MustFindProc("OpenClipboard")
	clipCloseClipboard   = clipUser32.MustFindProc("CloseClipboard")
	clipEmptyClipboard   = clipUser32.MustFindProc("EmptyClipboard")
	clipGetClipboardData = clipUser32.MustFindProc("GetClipboardData")
	clipSetClipboardData = clipUser32.MustFindProc("SetClipboardData")
	clipGlobalAlloc      = clipKernel32.MustFindProc("GlobalAlloc")
	clipGlobalFree       = clipKernel32.MustFindProc("GlobalFree")
	clipGlobalLock       = clipKernel32.MustFindProc("GlobalLock")
	clipGlobalUnlock     = clipKernel32.MustFindProc("GlobalUnlock")
)

const (
	cfUnicodeText = 13
	gmoveable     = 0x0002
)

func readClipboard() (string, error) {
	r, _, err := clipOpenClipboard.Call(0)
	if r == 0 {
		return "", fmt.Errorf("OpenClipboard: %w", err)
	}
	defer clipCloseClipboard.Call()

	h, _, _ := clipGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return "", nil
	}

	p, _, err := clipGlobalLock.Call(h)
	if p == 0 {
		return "", fmt.Errorf("GlobalLock: %w", err)
	}
	defer clipGlobalUnlock.Call(h)

	ptr := (*[1 << 20]uint16)(unsafe.Pointer(p))
	n := 0
	for ptr[n] != 0 {
		n++
	}
	return string(utf16.Decode(ptr[:n])), nil
}

func writeClipboard(text string) error {
	r, _, err := clipOpenClipboard.Call(0)
	if r == 0 {
		return fmt.Errorf("OpenClipboard: %w", err)
	}
	defer clipCloseClipboard.Call()

	r, _, err = clipEmptyClipboard.Call()
	if r == 0 {
		return fmt.Errorf("EmptyClipboard: %w", err)
	}

	encoded := utf16.Encode([]rune(text))
	encoded = append(encoded, 0) // null terminator

	h, _, err := clipGlobalAlloc.Call(gmoveable, uintptr(len(encoded)*2))
	if h == 0 {
		return fmt.Errorf("GlobalAlloc: %w", err)
	}

	p, _, err := clipGlobalLock.Call(h)
	if p == 0 {
		clipGlobalFree.Call(h)
		return fmt.Errorf("GlobalLock: %w", err)
	}

	dst := (*[1 << 20]uint16)(unsafe.Pointer(p))[:len(encoded)]
	copy(dst, encoded)
	clipGlobalUnlock.Call(h)

	r, _, err = clipSetClipboardData.Call(cfUnicodeText, h)
	if r == 0 {
		clipGlobalFree.Call(h)
		return fmt.Errorf("SetClipboardData: %w", err)
	}
	return nil
}
