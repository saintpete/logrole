package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"

	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/views"
)

type audioServer struct {
	Client    views.Client
	SecretKey *[32]byte
	Proxy     *httputil.ReverseProxy
}

var audioRoute = regexp.MustCompile("^/audio/(?P<encrypted>([-_a-zA-Z0-9=]+))$")

func newAudioReverseProxy() (*httputil.ReverseProxy, error) {
	u, err := url.Parse(twilio.BaseURL)
	if err != nil {
		return nil, err
	}
	return &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Host = u.Host
			r.URL.Scheme = "https"
		},
	}, nil
}

// GET /audio/<encrypted URL>
//
// Decode the encrypted URL, then make a request to retrieve the resource in
// question and forward it to the frontend.
//
// TODO: add some sort of caching layer, since the images are not changing.
func (a *audioServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	encoded := audioRoute.FindStringSubmatch(r.URL.Path)[1]
	u, wroteError := decryptURL(w, r, encoded, a.SecretKey)
	if wroteError {
		return
	}
	// Note this also rewrites the path in the logs, but that's probably OK,
	// since only admins have access to the server logs.
	r.URL.Path = u.Path
	r.URL.RawQuery = u.RawQuery
	a.Client.SetBasicAuth(r)
	a.Proxy.ServeHTTP(w, r)
}
