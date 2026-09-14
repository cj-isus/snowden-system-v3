package main

import (
	"crypto/rand"
	"syscall"
	"unsafe"
)

// randReadImpl — криптостойкий filler.
func randReadImpl(b []byte) (int, error) {
	return rand.Read(b)
}

// handleCurrentProcess — псевдо-хендл текущего процесса (-1).
func handleCurrentProcess() syscall.Handle {
	return syscall.Handle(^uintptr(0)) // GetCurrentProcess() = (HANDLE)-1
}

// isAdmin — входит ли процесс в контекст администратора (TokenElevation).
func isAdmin() bool {
	var token syscall.Token
	if err := syscall.OpenProcessToken(handleCurrentProcess(), syscall.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()

	type tokenElevation struct {
		TokenIsElevated uint32
	}
	var elevation tokenElevation
	var retLen uint32
	// TokenElevation = 20
	err := syscall.GetTokenInformation(token, 20, (*byte)(unsafe.Pointer(&elevation)), uint32(unsafe.Sizeof(elevation)), &retLen)
	if err != nil {
		return false
	}
	return elevation.TokenIsElevated != 0
}
