package metadata

// ValidChannelID is a portable identity, never a path or Windows device name.
// Lowercase ASCII also prevents case/Unicode aliases in pin lookup and tags.
func ValidChannelID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for i, c := range []byte(id) {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || i > 0 && (c == '-' || c == '_') {
			continue
		}
		return false
	}
	switch id {
	case "con", "prn", "aux", "nul":
		return false
	}
	if len(id) == 4 && (id[:3] == "com" || id[:3] == "lpt") && id[3] >= '1' && id[3] <= '9' {
		return false
	}
	return true
}
