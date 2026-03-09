package davxml_test

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	davxml "github.com/sdobberstein/contacthub/internal/dav/xml"
)

// --- ParsePropFind ---

func TestParsePropFind_EmptyBody_TreatedAsAllProp(t *testing.T) {
	pf, err := davxml.ParsePropFind(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pf.IsAllProp() {
		t.Error("empty body should be treated as allprop")
	}
}

func TestParsePropFind_AllProp(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><D:propfind xmlns:D="DAV:"><D:allprop/></D:propfind>`)
	pf, err := davxml.ParsePropFind(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pf.IsAllProp() {
		t.Error("want allprop=true")
	}
	if pf.IsPropName() {
		t.Error("want propname=false")
	}
}

func TestParsePropFind_PropName(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><D:propfind xmlns:D="DAV:"><D:propname/></D:propfind>`)
	pf, err := davxml.ParsePropFind(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pf.IsPropName() {
		t.Error("want propname=true")
	}
}

func TestParsePropFind_SpecificProps(t *testing.T) {
	body := []byte(`<?xml version="1.0"?>
		<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
			<D:prop>
				<D:current-user-principal/>
				<C:addressbook-home-set/>
			</D:prop>
		</D:propfind>`)
	pf, err := davxml.ParsePropFind(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pf.IsAllProp() || pf.IsPropName() {
		t.Error("want specific prop request, not allprop/propname")
	}
	props := pf.RequestedProps()
	if len(props) != 2 {
		t.Fatalf("want 2 props, got %d", len(props))
	}
	want := []xml.Name{
		{Space: "DAV:", Local: "current-user-principal"},
		{Space: "urn:ietf:params:xml:ns:carddav", Local: "addressbook-home-set"},
	}
	for i, w := range want {
		if props[i] != w {
			t.Errorf("prop[%d]: got {%s}%s, want {%s}%s",
				i, props[i].Space, props[i].Local, w.Space, w.Local)
		}
	}
}

func TestParsePropFind_InvalidXML(t *testing.T) {
	if _, err := davxml.ParsePropFind([]byte(`<notxml`)); err == nil {
		t.Error("want error for invalid XML")
	}
}

// --- PropBuilder ---

func TestPropBuilder_AddDAVText(t *testing.T) {
	var b davxml.PropBuilder
	b.AddDAVText("displayname", "Alice & Bob")
	got := string(b.InnerXML())
	if !strings.Contains(got, "<D:displayname>Alice &amp; Bob</D:displayname>") {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestPropBuilder_AddDAVHref(t *testing.T) {
	var b davxml.PropBuilder
	b.AddDAVHref("current-user-principal", "/dav/principals/users/alice/")
	got := string(b.InnerXML())
	if !strings.Contains(got, "<D:current-user-principal>") {
		t.Errorf("missing outer element: %q", got)
	}
	if !strings.Contains(got, "<D:href>/dav/principals/users/alice/</D:href>") {
		t.Errorf("missing href: %q", got)
	}
}

func TestPropBuilder_AddCardDAVHref(t *testing.T) {
	var b davxml.PropBuilder
	b.AddCardDAVHref("addressbook-home-set", "/dav/addressbooks/alice/")
	got := string(b.InnerXML())
	if !strings.Contains(got, "<C:addressbook-home-set>") {
		t.Errorf("missing C: prefix element: %q", got)
	}
	if !strings.Contains(got, "<D:href>/dav/addressbooks/alice/</D:href>") {
		t.Errorf("missing href: %q", got)
	}
}

func TestPropBuilder_AddDAVResourceType_Single(t *testing.T) {
	var b davxml.PropBuilder
	b.AddDAVResourceType("collection")
	got := string(b.InnerXML())
	want := "<D:resourcetype><D:collection/></D:resourcetype>"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPropBuilder_AddDAVResourceType_Multiple(t *testing.T) {
	var b davxml.PropBuilder
	b.AddDAVResourceType("collection", "principal")
	got := string(b.InnerXML())
	if !strings.Contains(got, "<D:collection/>") || !strings.Contains(got, "<D:principal/>") {
		t.Errorf("missing resource types: %q", got)
	}
}

// --- Multistatus ---

func TestMultistatus_IsValidXML(t *testing.T) {
	var b davxml.PropBuilder
	b.AddDAVText("displayname", "Alice")

	ms := davxml.NewMultistatus()
	ms.AddResponse("/dav/", davxml.OK(b.InnerXML()))

	out := ms.Bytes()
	if err := xml.Unmarshal(out, new(interface{})); err != nil {
		// xml.Unmarshal into interface{} may fail on unknown types; use tokenizer instead
		dec := xml.NewDecoder(bytes.NewReader(out))
		for {
			_, err := dec.Token()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				t.Fatalf("invalid XML output: %v\n%s", err, out)
			}
		}
	}
}

func TestMultistatus_ContainsExpectedElements(t *testing.T) {
	var b davxml.PropBuilder
	b.AddDAVHref("current-user-principal", "/dav/principals/users/alice/")

	ms := davxml.NewMultistatus()
	ms.AddResponse("/dav/", davxml.OK(b.InnerXML()))

	out := string(ms.Bytes())
	for _, want := range []string{
		`xmlns:D="DAV:"`,
		`xmlns:C="urn:ietf:params:xml:ns:carddav"`,
		"<D:multistatus",
		"<D:response>",
		"<D:href>/dav/</D:href>",
		"<D:propstat>",
		"<D:prop>",
		"HTTP/1.1 200 OK",
		"</D:multistatus>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestMultistatus_NotFound(t *testing.T) {
	miss := davxml.NotFound(
		xml.Name{Space: "DAV:", Local: "getetag"},
		xml.Name{Space: "urn:ietf:params:xml:ns:carddav", Local: "addressbook-description"},
	)

	ms := davxml.NewMultistatus()
	ms.AddResponse("/dav/", miss)
	out := string(ms.Bytes())

	if !strings.Contains(out, "HTTP/1.1 404 Not Found") {
		t.Errorf("want 404 status in output\n%s", out)
	}
	if !strings.Contains(out, "<D:getetag/>") {
		t.Errorf("want empty D:getetag element\n%s", out)
	}
	if !strings.Contains(out, "<C:addressbook-description/>") {
		t.Errorf("want empty C:addressbook-description element\n%s", out)
	}
}

func TestMultistatus_SkipsEmptyPropStats(t *testing.T) {
	ms := davxml.NewMultistatus()
	ms.AddResponse("/dav/", davxml.PropStatData{Inner: nil, Status: 200})
	out := string(ms.Bytes())
	if strings.Contains(out, "<D:propstat>") {
		t.Errorf("want no propstat for empty inner, got:\n%s", out)
	}
}

func TestPropBuilder_AddAddressbookResourceType(t *testing.T) {
	var b davxml.PropBuilder
	b.AddAddressbookResourceType()
	got := string(b.InnerXML())
	if !strings.Contains(got, "<D:collection/>") {
		t.Errorf("missing D:collection: %q", got)
	}
	if !strings.Contains(got, "<C:addressbook/>") {
		t.Errorf("missing C:addressbook: %q", got)
	}
}

func TestPropBuilder_AddCustomProp_DAVNamespace(t *testing.T) {
	var b davxml.PropBuilder
	b.AddCustomProp("DAV:", "getetag")
	got := string(b.InnerXML())
	if got != "<D:getetag/>" {
		t.Errorf("got %q, want <D:getetag/>", got)
	}
}

func TestPropBuilder_AddCustomProp_CustomNamespace(t *testing.T) {
	var b davxml.PropBuilder
	b.AddCustomProp("http://example.com/ns/", "color")
	got := string(b.InnerXML())
	if !strings.Contains(got, `xmlns:ns0="http://example.com/ns/"`) {
		t.Errorf("missing namespace declaration: %q", got)
	}
	if !strings.Contains(got, "color") {
		t.Errorf("missing element name: %q", got)
	}
}

// --- ParsePropPatch ---

func TestParsePropPatch_SetOperation(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:" xmlns:X="http://example.com/ns/">
  <D:set>
    <D:prop>
      <X:test-prop>hello</X:test-prop>
    </D:prop>
  </D:set>
</D:propertyupdate>`)

	pp, err := davxml.ParsePropPatch(body)
	if err != nil {
		t.Fatalf("ParsePropPatch: %v", err)
	}
	if len(pp.Ops) != 1 {
		t.Fatalf("want 1 op, got %d", len(pp.Ops))
	}
	op := pp.Ops[0]
	if op.Remove {
		t.Error("want set op, got remove")
	}
	if op.NS != "http://example.com/ns/" {
		t.Errorf("NS: got %q", op.NS)
	}
	if op.Local != "test-prop" {
		t.Errorf("Local: got %q", op.Local)
	}
	if op.Value != "hello" {
		t.Errorf("Value: got %q", op.Value)
	}
}

func TestParsePropPatch_RemoveOperation(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:" xmlns:X="http://example.com/ns/">
  <D:remove>
    <D:prop>
      <X:test-prop/>
    </D:prop>
  </D:remove>
</D:propertyupdate>`)

	pp, err := davxml.ParsePropPatch(body)
	if err != nil {
		t.Fatalf("ParsePropPatch: %v", err)
	}
	if len(pp.Ops) != 1 {
		t.Fatalf("want 1 op, got %d", len(pp.Ops))
	}
	if !pp.Ops[0].Remove {
		t.Error("want remove op")
	}
	if pp.Ops[0].Local != "test-prop" {
		t.Errorf("Local: got %q", pp.Ops[0].Local)
	}
}

func TestParsePropPatch_MultipleOps(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:" xmlns:X="http://example.com/ns/">
  <D:set>
    <D:prop>
      <X:prop-a>val-a</X:prop-a>
      <X:prop-b>val-b</X:prop-b>
    </D:prop>
  </D:set>
  <D:remove>
    <D:prop>
      <X:prop-c/>
    </D:prop>
  </D:remove>
</D:propertyupdate>`)

	pp, err := davxml.ParsePropPatch(body)
	if err != nil {
		t.Fatalf("ParsePropPatch: %v", err)
	}
	if len(pp.Ops) != 3 {
		t.Fatalf("want 3 ops, got %d", len(pp.Ops))
	}
	if pp.Ops[0].Remove || pp.Ops[1].Remove {
		t.Error("first two ops should be set")
	}
	if !pp.Ops[2].Remove {
		t.Error("third op should be remove")
	}
}

func TestParsePropPatch_InvalidXML(t *testing.T) {
	if _, err := davxml.ParsePropPatch([]byte(`<notxml`)); err == nil {
		t.Error("want error for invalid XML")
	}
}

func TestMultistatus_XmlEscapesHref(t *testing.T) {
	ms := davxml.NewMultistatus()
	ms.AddResponse("/dav/path?a=1&b=2", davxml.OK([]byte("<D:displayname>x</D:displayname>")))
	out := string(ms.Bytes())
	if strings.Contains(out, "&b=2") && !strings.Contains(out, "&amp;b=2") {
		t.Error("href should be XML-escaped")
	}
}
