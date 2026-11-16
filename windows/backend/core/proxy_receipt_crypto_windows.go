//go:build windows

package core

import (
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Owned by the original receipt store before any DPAPI call. Native allocator
// output is bounded before copying; the allocator's transient RSS is not bounded
// by this Go limit. No description is requested/allocated on Unprotect.
type proxyReceiptCryptoDomain uint8

const (
	proxyCryptoReceipt proxyReceiptCryptoDomain = iota
	proxyCryptoAnchor
	proxyCryptoStage
)

type proxyReceiptCrypto struct {
	call   *proxyReceiptCryptoCall
	domain proxyReceiptCryptoDomain // closed fixed-purpose set; zero retains original receipt compatibility
}
type proxyReceiptCryptoCall struct {
	input, entropy                     []byte
	in, purpose, output                windows.DataBlob
	started, callReturned, returned    bool
	freeStarted, freeReturned, retired bool
	result                             error
}

func (owner *proxyReceiptCrypto) retired() bool {
	return owner != nil && (owner.call == nil || owner.call.retired && owner.call.returned)
}

func (owner *proxyReceiptCrypto) transform(input []byte, seal bool) (raw []byte, result error) {
	if owner == nil {
		return nil, ErrProxyReceiptUnavailable
	}
	plainLimit, cipherLimit, purpose := maxProxyReceiptPlain, maxProxyReceiptCipher, proxyReceiptPurpose
	switch owner.domain {
	case proxyCryptoReceipt:
	case proxyCryptoAnchor:
		plainLimit, cipherLimit, purpose = maxProxyAnchorPlain, maxProxyAnchorCipher, proxyAnchorPurpose
	case proxyCryptoStage:
		plainLimit, cipherLimit, purpose = maxProxyStagePlain, maxProxyStageCipher, proxyStagePurpose
	default:
		return nil, ErrProxyReceiptUnavailable
	}
	inputLimit, outputLimit := cipherLimit, plainLimit
	if seal {
		inputLimit, outputLimit = plainLimit, cipherLimit
	}
	if !owner.retired() || len(input) == 0 || len(input) > inputLimit {
		return nil, ErrProxyReceiptUnavailable
	}
	call := &proxyReceiptCryptoCall{input: input, entropy: []byte(purpose)}
	owner.call = call // retain original Go inputs and native output before lazy API resolution/call
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result, raw = ErrProxyStateUncertain, nil
		}
		call.returned, call.result = returned, result
		// Unknown original API/free completion retains all buffers; no retry or
		// speculative second LocalFree. Ordinary known retirement can drop them.
		if returned && call.retired {
			call.input, call.entropy = nil, nil
			call.in, call.purpose = windows.DataBlob{}, windows.DataBlob{}
		}
	}()
	call.in = windows.DataBlob{Size: uint32(len(input)), Data: &call.input[0]}
	call.purpose = windows.DataBlob{Size: uint32(len(call.entropy)), Data: &call.entropy[0]}
	call.started = true
	var err error
	if seal {
		err = windows.CryptProtectData(&call.in, nil, &call.purpose, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &call.output)
	} else {
		err = windows.CryptUnprotectData(&call.in, nil, &call.purpose, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &call.output)
	}
	call.callReturned = true
	runtime.KeepAlive(call)
	if err != nil {
		if call.output.Data != nil || call.output.Size != 0 {
			returned = true // failed API's partial output is not an authenticated allocation receipt
			return nil, ErrProxyStateUncertain
		}
		call.retired, returned = true, true
		return nil, ErrProxyReceiptUnavailable
	}
	if call.output.Data == nil {
		if call.output.Size == 0 {
			call.retired = true
		}
		returned = true
		return nil, ErrProxyReceiptUnavailable
	}
	if call.output.Size > 0 && uint64(call.output.Size) <= uint64(outputLimit) {
		raw = make([]byte, int(call.output.Size))
		copy(raw, unsafe.Slice(call.output.Data, int(call.output.Size)))
	}
	call.freeStarted = true
	remaining, freeErr := windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(call.output.Data))))
	call.freeReturned = true
	if freeErr != nil || remaining != 0 {
		returned = true
		return nil, ErrProxyStateUncertain // original pointer retained, never replay LocalFree
	}
	call.output = windows.DataBlob{}
	call.retired, returned = true, true
	if len(raw) == 0 {
		return nil, ErrProxyReceiptUnavailable
	}
	return raw, nil
}
