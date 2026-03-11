package vcard_test

import (
	"strings"
	"testing"

	"github.com/sdobberstein/contacthub/internal/vcard"
)

const sampleV4 = "BEGIN:VCARD\r\n" +
	"VERSION:4.0\r\n" +
	"UID:urn:uuid:aaaaaaaa-0000-0000-0000-000000000001\r\n" +
	"FN:Alice Test\r\n" +
	"N:Test;Alice;;;\r\n" +
	"KIND:individual\r\n" +
	"ORG:ACME Corp;Engineering\r\n" +
	"BDAY:19900101\r\n" +
	"ANNIVERSARY:20150601\r\n" +
	"END:VCARD\r\n"

func TestComputeETag_NonEmpty(t *testing.T) {
	etag := vcard.ComputeETag(sampleV4)
	if etag == "" {
		t.Fatal("ETag should not be empty")
	}
	// 16 bytes = 32 hex chars.
	if len(etag) != 32 {
		t.Errorf("ETag length: want 32 hex chars, got %d (%q)", len(etag), etag)
	}
}

func TestComputeETag_Deterministic(t *testing.T) {
	a, b := vcard.ComputeETag(sampleV4), vcard.ComputeETag(sampleV4)
	if a != b {
		t.Errorf("ETag must be deterministic: got %q and %q", a, b)
	}
}

func TestComputeETag_DifferentForDifferentInput(t *testing.T) {
	a := vcard.ComputeETag("BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Alice\r\nEND:VCARD\r\n")
	b := vcard.ComputeETag("BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Bob\r\nEND:VCARD\r\n")
	if a == b {
		t.Error("different vCards should produce different ETags")
	}
}

func TestExtractUID(t *testing.T) {
	uid := vcard.ExtractUID(sampleV4)
	if uid != "urn:uuid:aaaaaaaa-0000-0000-0000-000000000001" {
		t.Errorf("ExtractUID: got %q", uid)
	}
}

func TestExtractUID_Missing(t *testing.T) {
	blob := "BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Alice\r\nEND:VCARD\r\n"
	if got := vcard.ExtractUID(blob); got != "" {
		t.Errorf("want empty UID, got %q", got)
	}
}

func TestExtractFN(t *testing.T) {
	if got := vcard.ExtractFN(sampleV4); got != "Alice Test" {
		t.Errorf("ExtractFN: got %q, want %q", got, "Alice Test")
	}
}

func TestExtractKind(t *testing.T) {
	if got := vcard.ExtractKind(sampleV4); got != "individual" {
		t.Errorf("ExtractKind: got %q", got)
	}
}

func TestExtractKind_DefaultsToIndividual(t *testing.T) {
	blob := "BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Alice\r\nEND:VCARD\r\n"
	if got := vcard.ExtractKind(blob); got != "individual" {
		t.Errorf("ExtractKind default: got %q", got)
	}
}

func TestExtractOrg_FirstComponentOnly(t *testing.T) {
	got := vcard.ExtractOrg(sampleV4)
	if got != "ACME Corp" {
		t.Errorf("ExtractOrg: got %q, want %q", got, "ACME Corp")
	}
}

func TestExtractBDay(t *testing.T) {
	if got := vcard.ExtractBDay(sampleV4); got != "19900101" {
		t.Errorf("ExtractBDay: got %q", got)
	}
}

func TestExtractAnniversary(t *testing.T) {
	if got := vcard.ExtractAnniversary(sampleV4); got != "20150601" {
		t.Errorf("ExtractAnniversary: got %q", got)
	}
}

func TestNewUID_Format(t *testing.T) {
	uid := vcard.NewUID()
	if !strings.HasPrefix(uid, "urn:uuid:") {
		t.Errorf("NewUID: want urn:uuid: prefix, got %q", uid)
	}
	// urn:uuid: + 36-char UUID = 45 chars.
	if len(uid) != 45 {
		t.Errorf("NewUID length: got %d, want 45", len(uid))
	}
}

func TestNewUID_Unique(t *testing.T) {
	a, b := vcard.NewUID(), vcard.NewUID()
	if a == b {
		t.Errorf("NewUID should return unique values: both returned %q", a)
	}
}

func TestReplaceUID_ReplacesExisting(t *testing.T) {
	result := vcard.ReplaceUID(sampleV4, "urn:uuid:newuid-0000-0000-0000-000000000001")
	if !strings.Contains(result, "UID:urn:uuid:newuid-0000-0000-0000-000000000001") {
		t.Errorf("ReplaceUID did not replace UID, got:\n%s", result)
	}
	if strings.Contains(result, "urn:uuid:aaaaaaaa-0000-0000-0000-000000000001") {
		t.Error("ReplaceUID left old UID in result")
	}
}

func TestReplaceUID_InsertsWhenMissing(t *testing.T) {
	blob := "BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Alice\r\nEND:VCARD\r\n"
	result := vcard.ReplaceUID(blob, "urn:uuid:newuid")
	if !strings.Contains(result, "UID:urn:uuid:newuid") {
		t.Errorf("ReplaceUID did not insert UID, got:\n%s", result)
	}
}

// --- Unfold ---

func TestUnfold_RemovesCRLFSpace(t *testing.T) {
	folded := "FN:Alice\r\n  Smith\r\n"
	got := vcard.Unfold(folded)
	if strings.Contains(got, "\r\n ") {
		t.Errorf("Unfold left CRLF+space: %q", got)
	}
	if !strings.Contains(got, "Alice Smith") {
		t.Errorf("Unfold lost content: %q", got)
	}
}

func TestUnfold_RemovesCRLFTab(t *testing.T) {
	folded := "FN:Alice\r\n\tSmith\r\n"
	got := vcard.Unfold(folded)
	if strings.Contains(got, "\r\n\t") {
		t.Errorf("Unfold left CRLF+tab: %q", got)
	}
}

// --- Validate ---

func TestValidate_ValidV4(t *testing.T) {
	blob := "BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Alice\r\nEND:VCARD\r\n"
	if err := vcard.Validate(blob); err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestValidate_ValidV3(t *testing.T) {
	blob := "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Alice\r\nN:Test;Alice;;;\r\nEND:VCARD\r\n"
	if err := vcard.Validate(blob); err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestValidate_MissingVersion(t *testing.T) {
	blob := "BEGIN:VCARD\r\nFN:Alice\r\nN:Test;Alice;;;\r\nEND:VCARD\r\n"
	if err := vcard.Validate(blob); err == nil {
		t.Error("want error for missing VERSION")
	}
}

func TestValidate_InvalidVersion(t *testing.T) {
	blob := "BEGIN:VCARD\r\nVERSION:2.1\r\nFN:Alice\r\nN:Test;Alice;;;\r\nEND:VCARD\r\n"
	if err := vcard.Validate(blob); err == nil {
		t.Error("want error for VERSION:2.1")
	}
}

func TestValidate_MissingFN(t *testing.T) {
	blob := "BEGIN:VCARD\r\nVERSION:4.0\r\nN:Test;Alice;;;\r\nEND:VCARD\r\n"
	if err := vcard.Validate(blob); err == nil {
		t.Error("want error for missing FN")
	}
}

func TestValidate_V3MissingN(t *testing.T) {
	blob := "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Alice\r\nEND:VCARD\r\n"
	if err := vcard.Validate(blob); err == nil {
		t.Error("want error for v3.0 missing N")
	}
}

func TestValidate_V4MissingN_OK(t *testing.T) {
	// N is optional in vCard 4.0 (RFC 6350 §6.2.2 cardinality *1).
	blob := "BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Alice\r\nEND:VCARD\r\n"
	if err := vcard.Validate(blob); err != nil {
		t.Errorf("N should be optional in v4.0, got: %v", err)
	}
}
