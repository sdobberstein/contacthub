// Package davxml provides WebDAV XML encoding and decoding (RFC 4918).
//
// Response XML uses explicit D: and C: namespace prefixes throughout so that
// inner <D:prop> fragments remain self-contained when extracted and re-parsed
// by conformance test tools.
package davxml

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// Namespace URIs used in WebDAV and CardDAV.
const (
	NSdav     = "DAV:"
	NScarddav = "urn:ietf:params:xml:ns:carddav"
)

// --- Request decoding ---

// PropFind is the decoded body of a PROPFIND request (RFC 4918 §9.1).
type PropFind struct {
	XMLName  xml.Name  `xml:"DAV: propfind"`
	AllProp  *struct{} `xml:"DAV: allprop"`
	PropName *struct{} `xml:"DAV: propname"`
	Prop     *ReqProp  `xml:"DAV: prop"`
}

// ReqProp holds the property names from a <D:prop> element.
type ReqProp struct {
	Elems []propElem `xml:",any"`
}

type propElem struct {
	XMLName xml.Name
}

// ParsePropFind decodes a PROPFIND request body.
// An empty or nil body is treated as <allprop/> per RFC 4918 §9.1.
func ParsePropFind(body []byte) (*PropFind, error) {
	if len(body) == 0 {
		return &PropFind{AllProp: &struct{}{}}, nil
	}
	var pf PropFind
	if err := xml.Unmarshal(body, &pf); err != nil {
		return nil, fmt.Errorf("parse propfind: %w", err)
	}
	return &pf, nil
}

// IsAllProp reports whether the request asks for all live properties.
func (pf *PropFind) IsAllProp() bool { return pf.AllProp != nil }

// IsPropName reports whether the request asks for property names only.
func (pf *PropFind) IsPropName() bool { return pf.PropName != nil }

// RequestedProps returns the explicitly requested property names.
// Returns nil when allprop or propname is used.
func (pf *PropFind) RequestedProps() []xml.Name {
	if pf.Prop == nil {
		return nil
	}
	names := make([]xml.Name, len(pf.Prop.Elems))
	for i, e := range pf.Prop.Elems {
		names[i] = e.XMLName
	}
	return names
}

// --- Response building ---

// PropBuilder accumulates property XML fragments for a single <D:prop> element.
// All output uses explicit D: and C: namespace prefixes.
type PropBuilder struct {
	buf strings.Builder
}

// AddDAVText appends <D:local>text</D:local>.
func (b *PropBuilder) AddDAVText(local, text string) {
	fmt.Fprintf(&b.buf, "<D:%s>%s</D:%s>", local, escXML(text), local)
}

// AddDAVHref appends <D:local><D:href>href</D:href></D:local>.
func (b *PropBuilder) AddDAVHref(local, href string) {
	fmt.Fprintf(&b.buf, "<D:%s><D:href>%s</D:href></D:%s>", local, escXML(href), local)
}

// AddCardDAVHref appends <C:local><D:href>href</D:href></C:local>.
func (b *PropBuilder) AddCardDAVHref(local, href string) {
	fmt.Fprintf(&b.buf, "<C:%s><D:href>%s</D:href></C:%s>", local, escXML(href), local)
}

// AddDAVResourceType appends <D:resourcetype><D:t1/><D:t2/>...</D:resourcetype>.
func (b *PropBuilder) AddDAVResourceType(types ...string) {
	b.buf.WriteString("<D:resourcetype>")
	for _, t := range types {
		fmt.Fprintf(&b.buf, "<D:%s/>", t)
	}
	b.buf.WriteString("</D:resourcetype>")
}

// InnerXML returns the accumulated property fragments as a byte slice.
func (b *PropBuilder) InnerXML() []byte {
	return []byte(b.buf.String())
}

// --- Multistatus response ---

// PropStatData holds inner XML and an HTTP status code for one <D:propstat>.
type PropStatData struct {
	Inner  []byte
	Status int
}

// OK returns a PropStatData with status 200.
func OK(inner []byte) PropStatData {
	return PropStatData{Inner: inner, Status: 200}
}

// NotFoundRaw returns a PropStatData with status 404 from pre-built inner XML.
func NotFoundRaw(inner []byte) PropStatData {
	return PropStatData{Inner: inner, Status: 404}
}

// NotFound returns a PropStatData with status 404 containing empty property elements.
func NotFound(names ...xml.Name) PropStatData {
	var b PropBuilder
	for _, n := range names {
		switch n.Space {
		case NSdav:
			fmt.Fprintf(&b.buf, "<D:%s/>", n.Local)
		case NScarddav:
			fmt.Fprintf(&b.buf, "<C:%s/>", n.Local)
		default:
			// Unknown namespace — emit with a generated prefix.
			fmt.Fprintf(&b.buf, `<ns0:%s xmlns:ns0="%s"/>`, n.Local, escXML(n.Space))
		}
	}
	return PropStatData{Inner: b.InnerXML(), Status: 404}
}

// Multistatus is a 207 Multi-Status response builder (RFC 4918 §13.4).
type Multistatus struct {
	buf strings.Builder
}

// NewMultistatus begins a new multistatus document with namespace declarations.
func NewMultistatus() *Multistatus {
	m := &Multistatus{}
	m.buf.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	m.buf.WriteString(`<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">`)
	return m
}

// AddResponse appends one <D:response> element.
// PropStatData entries with empty Inner are silently skipped.
func (m *Multistatus) AddResponse(href string, propstats ...PropStatData) {
	fmt.Fprintf(&m.buf, "<D:response><D:href>%s</D:href>", escXML(href))
	for _, ps := range propstats {
		if len(ps.Inner) == 0 {
			continue
		}
		fmt.Fprintf(&m.buf,
			"<D:propstat><D:prop>%s</D:prop><D:status>HTTP/1.1 %d %s</D:status></D:propstat>",
			ps.Inner, ps.Status, statusText(ps.Status),
		)
	}
	m.buf.WriteString("</D:response>")
}

// Bytes returns the complete, closed multistatus XML document.
func (m *Multistatus) Bytes() []byte {
	return []byte(m.buf.String() + "</D:multistatus>")
}

func statusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	default:
		return "Unknown"
	}
}

// AddAddressbookResourceType appends the CardDAV address book resource type.
func (b *PropBuilder) AddAddressbookResourceType() {
	b.buf.WriteString("<D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>")
}

// AddCustomProp appends a namespaced property element as a self-closing tag.
// Used when building PROPPATCH responses for dead properties.
func (b *PropBuilder) AddCustomProp(ns, local string) {
	switch ns {
	case NSdav:
		fmt.Fprintf(&b.buf, "<D:%s/>", local)
	case NScarddav:
		fmt.Fprintf(&b.buf, "<C:%s/>", local)
	default:
		fmt.Fprintf(&b.buf, `<ns0:%s xmlns:ns0="%s"/>`, local, escXML(ns))
	}
}

// AddCustomPropValue appends a namespaced property element with a text value.
// Used when building PROPFIND responses for dead properties.
func (b *PropBuilder) AddCustomPropValue(ns, local, value string) {
	switch ns {
	case NSdav:
		fmt.Fprintf(&b.buf, "<D:%s>%s</D:%s>", local, escXML(value), local)
	case NScarddav:
		fmt.Fprintf(&b.buf, "<C:%s>%s</C:%s>", local, escXML(value), local)
	default:
		fmt.Fprintf(&b.buf, `<ns0:%s xmlns:ns0="%s">%s</ns0:%s>`,
			local, escXML(ns), escXML(value), local)
	}
}

// --- PROPPATCH decoding ---

// PropPatchOp is a single set or remove operation from a PROPPATCH request.
type PropPatchOp struct {
	Remove bool   // true = remove, false = set
	NS     string // property namespace
	Local  string // property local name
	Value  string // raw inner XML for set ops; empty for remove
}

// PropPatch is the decoded body of a PROPPATCH request (RFC 4918 §9.2).
type PropPatch struct {
	Ops []PropPatchOp
}

// propPatchXML mirrors the XML structure of a <D:propertyupdate> element.
type propPatchXML struct {
	XMLName xml.Name       `xml:"DAV: propertyupdate"`
	Sets    []propPatchSet `xml:"DAV: set"`
	Removes []propPatchSet `xml:"DAV: remove"`
}

type propPatchSet struct {
	Prop struct {
		Elems []propPatchElem `xml:",any"`
	} `xml:"DAV: prop"`
}

type propPatchElem struct {
	XMLName xml.Name
	Inner   []byte `xml:",innerxml"`
}

// ParsePropPatch decodes a PROPPATCH request body.
func ParsePropPatch(body []byte) (*PropPatch, error) {
	var pu propPatchXML
	if err := xml.Unmarshal(body, &pu); err != nil {
		return nil, fmt.Errorf("parse propertyupdate: %w", err)
	}
	pp := &PropPatch{}
	for _, s := range pu.Sets {
		for _, e := range s.Prop.Elems {
			pp.Ops = append(pp.Ops, PropPatchOp{
				NS:    e.XMLName.Space,
				Local: e.XMLName.Local,
				Value: strings.TrimSpace(string(e.Inner)),
			})
		}
	}
	for _, r := range pu.Removes {
		for _, e := range r.Prop.Elems {
			pp.Ops = append(pp.Ops, PropPatchOp{
				Remove: true,
				NS:     e.XMLName.Space,
				Local:  e.XMLName.Local,
			})
		}
	}
	return pp, nil
}

func escXML(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s)) //nolint:errcheck // strings.Builder.Write never fails
	return b.String()
}
