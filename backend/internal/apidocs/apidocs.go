// Package apidocs embeds the API reference page and generated OpenAPI contract.
package apidocs

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var files embed.FS

func Mount(mux *http.ServeMux) {
	sub, _ := fs.Sub(files, "static")
	mux.Handle("GET /swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(sub))))
	// @api {"method":"GET","path":"/swagger","summary":"打开接口文档","auth":"public","status":307,"response":null,"errors":{}}
	mux.HandleFunc("GET /swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusTemporaryRedirect)
	})
	// @api {"method":"GET","path":"/openapi.json","summary":"读取OpenAPI契约","auth":"public","status":200,"response":{"type":"object"},"errors":{"503":"文档不可用"}}
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		b, e := files.ReadFile("static/openapi.json")
		if e != nil {
			http.Error(w, "文档不可用", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(b)
	})
}
