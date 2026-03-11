// Package vcard provides minimal vCard parsing helpers for contacthub.
// Full 3.0 ↔ 4.0 conversion is handled in Phase 7; this package covers
// the extraction and manipulation needed for WebDAV storage.
package vcard

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ErrInvalidVCard is returned by Validate for structurally invalid vCards.
var ErrInvalidVCard = errors.New("invalid vCard")

// Validate checks that blob satisfies the minimum RFC 2426/6350 requirements:
//   - VERSION must be present and "3.0" or "4.0"
//   - FN must be present (required in both v3.0 and v4.0)
//   - N must be present when VERSION is "3.0" (required by RFC 2426 §3.1.2)
//
// The blob should already be unfolded before calling Validate.
func Validate(blob string) error {
	version := strings.ToUpper(strings.TrimSpace(extractProp(blob, "VERSION")))
	if version == "" {
		return fmt.Errorf("%w: missing VERSION", ErrInvalidVCard)
	}
	if version != "3.0" && version != "4.0" {
		return fmt.Errorf("%w: unsupported VERSION %q (must be 3.0 or 4.0)", ErrInvalidVCard, version)
	}
	if extractProp(blob, "FN") == "" {
		return fmt.Errorf("%w: missing required FN property", ErrInvalidVCard)
	}
	if version == "3.0" && extractProp(blob, "N") == "" {
		return fmt.Errorf("%w: missing required N property (vCard 3.0)", ErrInvalidVCard)
	}
	return nil
}

// Unfold removes vCard line folding (CRLF + linear whitespace) per RFC 6350 §3.2.
func Unfold(blob string) string {
	return unfold(blob)
}


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

// maxPhotoBytes is the maximum allowed decoded size of an inline PHOTO (250 KB).
const maxPhotoBytes = 250 * 1024

// ExtractVersion returns the value of the VERSION property, or "".
func ExtractVersion(blob string) string {
	return extractProp(blob, "VERSION")
}

// ValidatePhotoSize returns an error if any inline PHOTO in blob exceeds 250 KB decoded.
// Both vCard 4.0 data URIs (PHOTO:data:...;base64,...) and 3.0 ENCODING=b form are checked.
func ValidatePhotoSize(blob string) error {
	unfolded := unfold(blob)
	for _, line := range strings.Split(unfolded, "\n") {
		line = strings.TrimRight(line, "\r")
		upper := strings.ToUpper(line)
		if !strings.HasPrefix(upper, "PHOTO") {
			continue
		}
		var b64 string
		if idx := strings.Index(line, "base64,"); idx >= 0 {
			b64 = line[idx+7:]
		} else if strings.Contains(upper, "ENCODING=B") {
			if idx := strings.Index(line, ":"); idx >= 0 {
				b64 = line[idx+1:]
			}
		}
		if b64 == "" {
			continue
		}
		// Approximate decoded size: base64 chars × 3/4.
		if len(b64)*3/4 > maxPhotoBytes {
			return fmt.Errorf("inline photo exceeds 250 KB limit")
		}
	}
	return nil
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

// unfold removes vCard line folding per RFC 6350 §3.2.
// Handles both CRLF+whitespace (spec-compliant) and LF+whitespace (lenient).
func unfold(blob string) string {
	// CRLF folding first (spec form).
	blob = strings.ReplaceAll(blob, "\r\n ", "")
	blob = strings.ReplaceAll(blob, "\r\n\t", "")
	// LF-only folding (tolerated per common practice).
	blob = strings.ReplaceAll(blob, "\n ", "")
	blob = strings.ReplaceAll(blob, "\n\t", "")
	return blob
}
