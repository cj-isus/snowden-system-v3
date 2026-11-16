package main

import (
	"strconv"
	"unicode"
	"unicode/utf8"
)

const appLogTextBytes = 4096
const appLogTextOmitted = " [сообщение усечено]"

// Record framing/retention only, not secret redaction or authenticated origin.
// Run before every App memory/queue/event insertion; producers still must not
// supply credentials. Native engine.log is a separate upstream file.
func projectAppLogRecord(level, text string) (string, string) {
	switch level {
	case "debug", "info", "warn", "error":
		return level, projectAppLogText(text)
	default:
		return "warn", "Сообщение с неподдерживаемым уровнем журнала; исходный текст не принят."
	}
}

func projectAppLogText(text string) string {
	var output [appLogTextBytes]byte
	var scratch [32]byte
	const hex = "0123456789abcdef"
	used, safe := 0, 0 // safe is a whole-piece boundary leaving room for the marker
	for offset := 0; offset < len(text); {
		r, size := utf8.DecodeRuneInString(text[offset:])
		var escaped []byte
		switch {
		case r == utf8.RuneError && size == 1:
			b := text[offset]
			escaped = scratch[:4]
			escaped[0], escaped[1], escaped[2], escaped[3] = '\\', 'x', hex[b>>4], hex[b&15]
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029':
			quoted := strconv.AppendQuoteRuneToASCII(scratch[:0], r)
			escaped = quoted[1 : len(quoted)-1] // visible escape, without outer quotes
		}
		length := size
		if escaped != nil {
			length = len(escaped)
		}
		if used+length > len(output) {
			copy(output[safe:], appLogTextOmitted)
			return string(output[:safe+len(appLogTextOmitted)])
		}
		if escaped != nil {
			copy(output[used:], escaped)
		} else {
			copy(output[used:], text[offset:offset+size])
		}
		used += length
		offset += size
		if used <= len(output)-len(appLogTextOmitted) {
			safe = used
		}
	}
	return string(output[:used])
}
