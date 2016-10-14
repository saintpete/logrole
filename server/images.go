package server

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"

	"github.com/kevinburke/handlers"
	"github.com/kevinburke/rest"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/services"
)

// An imageServer provides an opaque proxy for image requests.
type imageServer struct {
	SecretKey *[32]byte
}

var imageRoute = regexp.MustCompile("^/images/(?P<encrypted>([-_a-zA-Z0-9=]+))$")

// GET /images/<encrypted URL>
//
// Decode the encrypted URL, then make a request to retrieve the resource in
// question and forward it to the frontend.
//
// TODO: add some sort of caching layer, since the images are not changing.
func (i *imageServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	encoded := imageRoute.FindStringSubmatch(r.URL.Path)[1]
	urlStr, err := services.Unopaque(encoded, i.SecretKey)
	if err != nil {
		rest.BadRequest(w, r, &rest.Error{
			Title: err.Error(),
		})
		return
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		handlers.Logger.Warn("Could not parse decrypted string as URL", "str", urlStr)
		rest.BadRequest(w, r, &rest.Error{
			Title: "Could not parse decrypted string as a URL",
		})
		return
	}
	// TODO: only allow images to a defined set of hosts. I'm not sure of all
	// of the different URLs used by Twilio to host media content.
	//
	// Not too worried about this, though, since we control the URL on the
	// server side and it's encrypted.
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		handlers.Logger.Warn("Could not create proxy request", "err", err)
		rest.BadRequest(w, r, &rest.Error{
			Title: "Could not create proxy request",
		})
		return
	}
	resp, err := twilio.MediaClient.Do(req)
	if err != nil {
		rest.ServerError(w, r, err)
		return
	}
	defer resp.Body.Close()
	if ctype := resp.Header.Get("Content-Type"); ctype == "" {
		rest.ServerError(w, r, errors.New("Proxied request had no content-type header"))
		return
	} else {
		w.Header().Set("Content-Type", ctype)
	}
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, resp.Body); err != nil {
		rest.ServerError(w, r, err)
		return
	}
}
