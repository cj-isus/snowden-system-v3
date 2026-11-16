package main

import (
	"encoding/json"
	"errors"

	"github.com/snowden-system/windows/backend/metadata"
)

// runPin prepares public publisher data only. A new destination prevents
// overwriting the certificate or an existing policy. Sign the full channel
// envelope afterwards; this object alone is not authenticated delivery.
func runPin(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: metadatatool pin <public-leaf.pem> <new-pin.json>")
	}
	raw, err := readBindingFile(args[0], metadata.MaxPinCertificateSize)
	if err != nil {
		return err
	}
	pin, err := metadata.NewCertificatePin(raw)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(pin, "", "  ")
	if err != nil {
		return err
	}
	return publishBindingFile(args[1], encoded)
}
