package vcard

import (
	"strings"
)

// ToV4 converts a vCard 3.0 blob to vCard 4.0 and returns the result.
// If the blob is already VERSION:4.0, it is returned unchanged.
// Conversion covers: VERSION line, EMAIL type parameters, and PHOTO
// with ENCODING=b/BASE64 → data URI form.
func ToV4(blob string) string {
	if strings.ToUpper(strings.TrimSpace(extractProp(blob, "VERSION"))) == "4.0" {
		return blob
	}
	lines := strings.Split(blob, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		eol := ""
		if strings.HasSuffix(line, "\r") {
			eol = "\r"
			line = line[:len(line)-1]
		}
		converted := convertLineToV4(line)
		if converted != "" {
			out = append(out, converted+eol)
		}
	}
	return strings.Join(out, "\n")
}

// ToV3 converts a vCard 4.0 blob to vCard 3.0 and returns the result.
// If the blob is already VERSION:3.0, it is returned unchanged.
// Conversion covers: VERSION line, EMAIL type parameters, and PHOTO
// data URIs → ENCODING=b form.
func ToV3(blob string) string {
	if strings.ToUpper(strings.TrimSpace(extractProp(blob, "VERSION"))) == "3.0" {
		return blob
	}
	lines := strings.Split(blob, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		eol := ""
		if strings.HasSuffix(line, "\r") {
			eol = "\r"
			line = line[:len(line)-1]
		}
		out = append(out, convertLineToV3(line)+eol)
	}
	return strings.Join(out, "\n")
}

func convertLineToV4(line string) string {
	upper := strings.ToUpper(line)
	switch {
	case upper == "VERSION:3.0":
		return "VERSION:4.0"
	case strings.HasPrefix(upper, "EMAIL"):
		return convertEmailToV4(line)
	case strings.HasPrefix(upper, "PHOTO") && hasEncodingB(upper):
		return convertPhotoToV4(line)
	case upper == "PROFILE:VCARD":
		return "" // 3.0-only; drop it
	default:
		return line
	}
}

func convertLineToV3(line string) string {
	upper := strings.ToUpper(line)
	switch {
	case upper == "VERSION:4.0":
		return "VERSION:3.0"
	case strings.HasPrefix(upper, "EMAIL"):
		return convertEmailToV3(line)
	case strings.HasPrefix(upper, "PHOTO") && strings.Contains(line, "data:"):
		return convertPhotoToV3(line)
	default:
		return line
	}
}

// convertEmailToV4 converts an EMAIL line from 3.0 to 4.0 format.
// TYPE=INTERNET is removed; remaining types are lowercased.
func convertEmailToV4(line string) string {
	prop, params, value, ok := splitPropLine(line)
	if !ok {
		return line
	}
	newParams := emailParamsToV4(params)
	if newParams == "" {
		return prop + ":" + value
	}
	return prop + ";" + newParams + ":" + value
}

// convertEmailToV3 converts an EMAIL line from 4.0 to 3.0 format.
// INTERNET is added to the TYPE list and all type values are uppercased.
func convertEmailToV3(line string) string {
	prop, params, value, ok := splitPropLine(line)
	if !ok {
		return line
	}
	newParams := emailParamsToV3(params)
	return prop + ";" + newParams + ":" + value
}

// emailParamsToV4 removes INTERNET from the TYPE param and lowercases the rest.
func emailParamsToV4(params string) string {
	if params == "" {
		return ""
	}
	parts := strings.Split(params, ";")
	var out []string
	for _, p := range parts {
		if !strings.HasPrefix(strings.ToUpper(p), "TYPE=") {
			out = append(out, p)
			continue
		}
		vals := strings.Split(p[5:], ",")
		var keep []string
		for _, v := range vals {
			if strings.EqualFold(v, "INTERNET") {
				continue
			}
			keep = append(keep, strings.ToLower(v))
		}
		if len(keep) > 0 {
			out = append(out, "TYPE="+strings.Join(keep, ","))
		}
	}
	return strings.Join(out, ";")
}

// emailParamsToV3 ensures INTERNET is in the TYPE param and uppercases all values.
func emailParamsToV3(params string) string {
	if params == "" {
		return "TYPE=INTERNET"
	}
	parts := strings.Split(params, ";")
	var out []string
	typeFound := false
	for _, p := range parts {
		if !strings.HasPrefix(strings.ToUpper(p), "TYPE=") {
			out = append(out, p)
			continue
		}
		typeFound = true
		vals := strings.Split(p[5:], ",")
		var keep []string
		hasInternet := false
		for _, v := range vals {
			if strings.EqualFold(v, "INTERNET") {
				hasInternet = true
			}
			keep = append(keep, strings.ToUpper(v))
		}
		if !hasInternet {
			keep = append([]string{"INTERNET"}, keep...)
		}
		out = append(out, "TYPE="+strings.Join(keep, ","))
	}
	if !typeFound {
		out = append(out, "TYPE=INTERNET")
	}
	return strings.Join(out, ";")
}

// convertPhotoToV4 converts PHOTO;ENCODING=b;TYPE=JPEG:<data> → PHOTO:data:image/jpeg;base64,<data>.
func convertPhotoToV4(line string) string {
	colonIdx := strings.Index(line, ":")
	if colonIdx < 0 {
		return line
	}
	propParams := strings.ToUpper(line[:colonIdx])
	data := line[colonIdx+1:]

	mimeType := "image/jpeg"
	if strings.Contains(propParams, "TYPE=PNG") {
		mimeType = "image/png"
	} else if strings.Contains(propParams, "TYPE=GIF") {
		mimeType = "image/gif"
	}
	return "PHOTO:data:" + mimeType + ";base64," + data
}

// convertPhotoToV3 converts PHOTO:data:image/jpeg;base64,<data> → PHOTO;ENCODING=b;TYPE=JPEG:<data>.
func convertPhotoToV3(line string) string {
	dataIdx := strings.Index(line, "data:")
	if dataIdx < 0 {
		return line
	}
	rest := line[dataIdx+5:]

	semiIdx := strings.Index(rest, ";")
	if semiIdx < 0 {
		return line
	}
	mimeType := rest[:semiIdx]
	rest = rest[semiIdx+1:]

	commaIdx := strings.Index(rest, ",")
	if commaIdx < 0 {
		return line
	}
	data := rest[commaIdx+1:]

	typeStr := "JPEG"
	switch strings.ToLower(mimeType) {
	case "image/png":
		typeStr = "PNG"
	case "image/gif":
		typeStr = "GIF"
	}
	return "PHOTO;ENCODING=b;TYPE=" + typeStr + ":" + data
}

// splitPropLine splits "PROP;PARAMS:value" into (prop, params, value, true).
// params does not include the leading semicolon.
// Returns ok=false if no colon is found.
func splitPropLine(line string) (prop, params, value string, ok bool) {
	colonIdx := strings.Index(line, ":")
	if colonIdx < 0 {
		return "", "", "", false
	}
	value = line[colonIdx+1:]
	propPart := line[:colonIdx]
	semiIdx := strings.Index(propPart, ";")
	if semiIdx < 0 {
		return propPart, "", value, true
	}
	return propPart[:semiIdx], propPart[semiIdx+1:], value, true
}

// hasEncodingB reports whether a (uppercased) property line contains ENCODING=B or ENCODING=BASE64.
func hasEncodingB(upper string) bool {
	return strings.Contains(upper, "ENCODING=B;") ||
		strings.Contains(upper, "ENCODING=B:") ||
		strings.Contains(upper, "ENCODING=BASE64")
}
