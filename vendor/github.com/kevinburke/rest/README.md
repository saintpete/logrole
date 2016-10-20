# rest

This library contains a number of useful middlewares for writing a HTTP server
in Go. For more information and package documentation, please [see the godoc
documentation][gddo].

### Defining Custom Error Responses

`rest` exposes a number of error handlers - for example, `rest.ServerError(w, r, err)`.
By default these error handlers will write a generic JSON response over the
wire, using fields specified by the [HTTP problem spec][spec].

You can define a custom error handler if you like (say if you want to return
a HTML server error, or 404 error or similar) by calling RegisterHandler:

```go
rest.RegisterHandler(500, func(w http.ResponseWriter, r *http.Request) {
    err := rest.CtxErr(r)
    fmt.Println("Server error:", err)
    w.Header().Set("Content-Type", "text/html")
    w.WriteHeader(500)
    w.Write([]byte("<html><body>Server Error</body></html>"))
})
```

[spec]: https://tools.ietf.org/html/draft-ietf-appsawg-http-problem-03

### Debugging

Set the DEBUG_HTTP_TRAFFIC environment variable to print out all
request/response traffic being made by the client.

[gddo]: https://godoc.org/github.com/kevinburke/rest
