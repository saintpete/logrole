package server

import "net/http"

type server struct {
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// This header doesn't mean anything when served over HTTP, but
	// detecting HTTPS is a general way is hard, so let's just send it
	// every time.
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
	w.WriteHeader(http.StatusServiceUnavailable)

	_, err := w.Write([]byte("Hello World"))
	if err != nil {
		panic(err)
	}
}

func UpgradeInsecureHandler(h http.Handler, allowUnencryptedTraffic bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowUnencryptedTraffic == false {
			if r.Header.Get("X-Forwarded-Proto") == "http" {
				u := r.URL
				u.Scheme = "https"
				u.Host = r.Host
				http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

var Server = NewServer(false)

func NewServer(allowUnencryptedTraffic bool) http.Handler {
	return UpgradeInsecureHandler(&server{}, allowUnencryptedTraffic)
}
