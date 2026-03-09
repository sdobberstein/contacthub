package handler

import (
	"encoding/xml"
	"io"
	"net/http"

	davxml "github.com/sdobberstein/contacthub/internal/dav/xml"
)

// DAVOptions responds to OPTIONS on any /dav/ path, advertising the DAV
// compliance classes required by CardDAV (RFC 4918 + RFC 3744 + RFC 6352).
func DAVOptions(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("DAV", "1, 2, access-control, addressbook")
	w.Header().Set("Allow", "OPTIONS, GET, PUT, DELETE, PROPFIND, REPORT, MKCOL, COPY, MOVE, PROPPATCH, ACL, LOCK, UNLOCK")
	w.WriteHeader(http.StatusOK)
}

// writeMultistatus sends a 207 Multi-Status response.
func writeMultistatus(w http.ResponseWriter, ms *davxml.Multistatus) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = w.Write(ms.Bytes()) //nolint:errcheck // write to ResponseWriter, error unrecoverable
}

// readBody reads and closes the request body. Returns nil, nil for absent bodies.
func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close() //nolint:errcheck // deferred close; read error checked below
	return io.ReadAll(r.Body)
}

// liveProp describes a single live WebDAV property and how to encode it.
type liveProp struct {
	ns, local string
	write     func(*davxml.PropBuilder)
}

// buildPropResponse assembles the 200-OK and 404-Not-Found PropStatData for the
// given propfind request against a set of live properties.
func buildPropResponse(pf *davxml.PropFind, props []liveProp) []davxml.PropStatData {
	var hit davxml.PropBuilder

	for _, lp := range props {
		if wantProp(pf, lp.ns, lp.local) {
			lp.write(&hit)
		}
	}

	var miss []xml.Name
	if !pf.IsAllProp() && !pf.IsPropName() {
		for _, req := range pf.RequestedProps() {
			found := false
			for _, lp := range props {
				if req.Space == lp.ns && req.Local == lp.local {
					found = true
					break
				}
			}
			if !found {
				miss = append(miss, req)
			}
		}
	}

	var result []davxml.PropStatData
	if inner := hit.InnerXML(); len(inner) > 0 {
		result = append(result, davxml.OK(inner))
	}
	if len(miss) > 0 {
		result = append(result, davxml.NotFound(miss...))
	}
	return result
}

// wantProp reports whether pf requests the property identified by ns+local.
func wantProp(pf *davxml.PropFind, ns, local string) bool {
	if pf.IsAllProp() {
		return true
	}
	for _, n := range pf.RequestedProps() {
		if n.Space == ns && n.Local == local {
			return true
		}
	}
	return false
}
