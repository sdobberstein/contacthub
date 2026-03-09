// Package vcard provides minimal vCard parsing helpers for contacthub.
// Full 3.0 ↔ 4.0 conversion is handled in Phase 7; this package covers
// the extraction and manipulation needed for WebDAV storage.
package vcard

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ComputeETag returns the first 16 bytes of SHA-256(blob) as a lowercase hex string.
// The result is stored without surrounding quotes; callers add quotes for HTTP headers.
func ComputeETag(blob string) string {
	h := sha256.Sum256([]byte(blob))
	return hex.EncodeToString(h[:16])
}

// NewUID generates a new RFC 6350 UID in urn:uuid: form.
func NewUID() string {
	return "urn:uuid:" + uuid.New().String()
}

// ExtractUID returns the value of the UID property, or "" if not present.
func ExtractUID(blob string) string {
	return extractProp(blob, "UID")
}

// ExtractFN returns the value of the FN property, or "" if not present.
func ExtractFN(blob string) string {
	return extractProp(blob, "FN")
}

// ExtractKind returns the value of the KIND property, defaulting to "individual".
func ExtractKind(blob string) string {
	k := extractProp(blob, "KIND")
	if k == "" {
		return "individual"
	}
	return strings.ToLower(k)
}

// ExtractOrg returns the first component of the ORG property, or "" if absent.
func ExtractOrg(blob string) string {
	raw := extractProp(blob, "ORG")
	// ORG value may have components separated by semicolons; take the first.
	if idx := strings.IndexByte(raw, ';'); idx >= 0 {
		return raw[:idx]
	}
	return raw
}

// ExtractBDay returns the value of the BDAY property, or "".
func ExtractBDay(blob string) string {
	return extractProp(blob, "BDAY")
}

// ExtractAnniversary returns the value of the ANNIVERSARY property, or "".
func ExtractAnniversary(blob string) string {
	return extractProp(blob, "ANNIVERSARY")
}

// ReplaceUID replaces the UID property value in blob with newUID and returns
// the modified blob. If no UID line is found, one is inserted before END:VCARD.
func ReplaceUID(blob, newUID string) string {
	lines := strings.Split(blob, "\n")
	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "UID:") || strings.HasPrefix(upper, "UID;") {
			hasCR := strings.HasSuffix(line, "\r")
			newLine := "UID:" + newUID
			if hasCR {
				newLine += "\r"
			}
			lines[i] = newLine
			return strings.Join(lines, "\n")
		}
	}
	// No UID found — insert before END:VCARD.
	return strings.Replace(blob, "END:VCARD", fmt.Sprintf("UID:%s\r\nEND:VCARD", newUID), 1)
}

// extractProp searches the unfolded blob for a property named propName
// and returns its value. Comparison is case-insensitive; the first match wins.
func extractProp(blob, propName string) string {
	unfolded := unfold(blob)
	upper := strings.ToUpper(propName)
	for _, line := range strings.Split(unfolded, "\n") {
		line = strings.TrimRight(line, "\r")
		lineUpper := strings.ToUpper(line)
		// Plain property: "NAME:value"
		if strings.HasPrefix(lineUpper, upper+":") {
			return strings.TrimSpace(line[len(propName)+1:])
		}
		// Property with parameters: "NAME;PARAM=val:value"
		if strings.HasPrefix(lineUpper, upper+";") {
			idx := strings.IndexByte(line, ':')
			if idx >= 0 {
				return strings.TrimSpace(line[idx+1:])
			}
		}
	}
	return ""
}

// unfold removes vCard line folding (CRLF + linear whitespace) per RFC 5545 §3.1.
func unfold(blob string) string {
	blob = strings.ReplaceAll(blob, "\r\n ", "")
	blob = strings.ReplaceAll(blob, "\r\n\t", "")
	return blob
}
