package server

import (
	"bytes"
	"html/template"
	"io"
	"sync"
)

var templatePool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

// Render renders the given template to a bytes.Buffer. If the template renders
// successfully, we write it to the ResponseWriter, otherwise we return the
// error.
func render(w io.Writer, tpl *template.Template, name string, data interface{}) error {
	b := templatePool.Get().(*bytes.Buffer)
	defer templatePool.Put(b)
	if err := tpl.ExecuteTemplate(b, name, data); err != nil {
		return err
	}
	_, err := io.Copy(w, b)
	return err
}
