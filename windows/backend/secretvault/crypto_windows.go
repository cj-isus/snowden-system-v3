package secretvault

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Низкоуровневый доступ к DPAPI (crypt32.dll) через LazyProc — без codegen.
// CRYPTPROTECT_UI_FORBIDDEN = 0x1: никаких диалогов из фонового процесса.

var (
	modCrypt32             = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtectData   = modCrypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = modCrypt32.NewProc("CryptUnprotectData")
)

const cryptProtectUIForbidden = 0x1

type dataBlob struct {
	Size uint32
	Data *byte
}

func blobFromBytes(b []byte) dataBlob {
	if len(b) == 0 {
		return dataBlob{}
	}
	return dataBlob{Size: uint32(len(b)), Data: &b[0]}
}

func bytesFromBlob(b dataBlob) []byte {
	if b.Size == 0 || b.Data == nil {
		return nil
	}
	return unsafe.Slice(b.Data, b.Size)
}

// dpapiProtect шифрует данные в контексте текущего пользователя Windows.
func dpapiProtect(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, fmt.Errorf("dpapi: empty input")
	}
	in := blobFromBytes(plain)
	var out dataBlob
	r1, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, // szDataDescr: описание не нужно (не хранит секретов, но и не мешает)
		0, // pOptionalEntropy
		0, // pvReserved
		0, // pPromptStruct
		cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if r1 == 0 {
		return nil, fmt.Errorf("dpapi: CryptProtectData: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	result := make([]byte, out.Size)
	copy(result, bytesFromBlob(out))
	return result, nil
}

// dpapiUnprotect расшифровывает; ошибка = чужой профиль/повреждение.
func dpapiUnprotect(cipher []byte) ([]byte, error) {
	if len(cipher) == 0 {
		return nil, fmt.Errorf("dpapi: empty input")
	}
	in := blobFromBytes(cipher)
	var out dataBlob
	r1, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, // ppszDataDescr
		0, // pOptionalEntropy
		0, // pvReserved
		0, // pPromptStruct
		cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if r1 == 0 {
		return nil, fmt.Errorf("dpapi: CryptUnprotectData: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	result := make([]byte, out.Size)
	copy(result, bytesFromBlob(out))
	return result, nil
}
