package trace

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"regexp"
)

// Tracer implements http.RoundTripper.  It prints each request and
// response/error to os.Stderr.
type Tracer struct {
	http.RoundTripper
}

// RoundTrip calls the nested RoundTripper while printing each request and
// response/error to os.Stderr on either side of the nested call.
func (t *Tracer) RoundTrip(req *http.Request) (*http.Response, error) {
	// Dump the request to os.Stderr.
	b, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		return nil, err
	}
	os.Stderr.Write(t.sanitize(b))
	os.Stderr.Write([]byte{'\n'})

	// Call the nested RoundTripper.
	resp, err := t.RoundTripper.RoundTrip(req)

	// If an error was returned, dump it to os.Stderr.
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return resp, err
	}

	// Dump the response to os.Stderr.
	b, err = httputil.DumpResponse(resp, req.URL.Query().Get("watch") != "true")
	if err != nil {
		return nil, err
	}
	os.Stderr.Write(b)
	os.Stderr.Write([]byte("---"))
	os.Stderr.Write([]byte{'\n'})

	return resp, err
}

func (t *Tracer) sanitize(raw []byte) []byte {
	s := string(raw)
	regExp := regexp.MustCompile("Authorization: Bearer .*\n")
	s = regExp.ReplaceAllString(s, "Authorization: Bearer xxx\n")
	return []byte(s)
}
