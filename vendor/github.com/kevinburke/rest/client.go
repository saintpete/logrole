package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime"
	"strings"
	"time"
)

type UploadType string

var JSON UploadType = "application/json"
var FormURLEncoded UploadType = "application/x-www-form-urlencoded"

const Version = "0.15"

var defaultTimeout = 6500 * time.Millisecond
var defaultHttpClient = &http.Client{Timeout: defaultTimeout}

// Client is a generic Rest client for making HTTP requests.
type Client struct {
	ID     string
	Token  string
	Client *http.Client
	Base   string
	// Set UploadType to JSON or FormURLEncoded to control how data is sent to
	// the server.
	UploadType UploadType
	// ErrorParser is invoked when the client gets a 400-or-higher status code
	// from the server.
	ErrorParser func(*http.Response) error
}

// NewClient returns a new Client with the given user and password. Base is the
// scheme+domain to hit for all requests. By default, the request timeout is
// set to 6.5 seconds.
func NewClient(user, pass, base string) *Client {
	return &Client{
		ID:          user,
		Token:       pass,
		Client:      defaultHttpClient,
		Base:        base,
		UploadType:  JSON,
		ErrorParser: DefaultErrorParser,
	}
}

// DialSocket configures the client to dial a Unix socket instead of a TCP port.
func (c *Client) DialSocket(socket string) {
	dialSock := func(proto, addr string) (conn net.Conn, err error) {
		return net.Dial("unix", socket)
	}
	// TODO check whether this copies timeouts
	transport := &http.Transport{
		Dial: dialSock,
	}
	if c.Client == nil {
		c.Client = &http.Client{
			Timeout: defaultTimeout,
		}
	}
	c.Client.Transport = transport
}

// NewRequest creates a new Request and sets basic auth based on the client's
// authentication information.
func (c *Client) NewRequest(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, c.Base+path, body)
	if err != nil {
		return nil, err
	}
	if c.ID != "" || c.Token != "" {
		req.SetBasicAuth(c.ID, c.Token)
	}
	gv := strings.Replace(runtime.Version(), "go", "", 1)
	ua := fmt.Sprintf("rest-client/%s (https://github.com/kevinburke/rest) go/%s (%s/%s)",
		Version, gv, runtime.GOOS, runtime.GOARCH)
	req.Header.Add("User-Agent", ua)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Accept-Charset", "utf-8")
	if method == "POST" || method == "PUT" {
		uploadType := c.UploadType
		if uploadType == "" {
			uploadType = JSON
		}
		req.Header.Add("Content-Type", fmt.Sprintf("%s; charset=utf-8", uploadType))
	}
	return req, nil
}

// Do performs the HTTP request. If the HTTP response is in the 2xx range,
// Unmarshal the response body into v. If the response status code is 400 or
// above, attempt to Unmarshal the response into an Error. Otherwise return
// a generic http error.
func (c *Client) Do(r *http.Request, v interface{}) error {
	b := new(bytes.Buffer)
	if os.Getenv("DEBUG_HTTP_TRAFFIC") == "true" || os.Getenv("DEBUG_HTTP_REQUEST") == "true" {
		bits, err := httputil.DumpRequestOut(r, true)
		if err != nil {
			return err
		}
		if len(bits) > 0 && bits[len(bits)-1] != '\n' {
			bits = append(bits, '\n')
		}
		b.Write(bits)
	}
	res, err := c.Client.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if os.Getenv("DEBUG_HTTP_TRAFFIC") == "true" || os.Getenv("DEBUG_HTTP_RESPONSES") == "true" {
		bits, err := httputil.DumpResponse(res, true)
		if err != nil {
			return err
		}
		if len(bits) > 0 && bits[len(bits)-1] != '\n' {
			bits = append(bits, '\n')
		}
		b.Write(bits)
	}
	if b.Len() > 0 {
		_, err = b.WriteTo(os.Stderr)
		if err != nil {
			return err
		}
	}
	if res.StatusCode >= 400 {
		err := c.ErrorParser(res)
		return err
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if v == nil || res.StatusCode == http.StatusNoContent {
		return nil
	} else {
		return json.Unmarshal(resBody, v)
	}
}

// DefaultErrorParser attempts to parse the response body as a rest.Error. If
// it cannot do so, return an error containing the entire response body.
func DefaultErrorParser(resp *http.Response) error {
	resBody, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	rerr := new(Error)
	err = json.Unmarshal(resBody, rerr)
	if err != nil {
		return fmt.Errorf("invalid response body: %s", string(resBody))
	}
	if rerr.Title == "" {
		return fmt.Errorf("invalid response body: %s", string(resBody))
	} else {
		rerr.StatusCode = resp.StatusCode
		return rerr
	}
}
