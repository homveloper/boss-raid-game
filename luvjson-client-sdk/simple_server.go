package main

import (
	"log"
	"net/http"
	"path/filepath"
)

func main() {
	// 정적 파일 서버 설정
	fs := http.FileServer(http.Dir("."))

	// WASM 파일에 대한 MIME 타입 설정
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if filepath.Ext(r.URL.Path) == ".wasm" {
			w.Header().Set("Content-Type", "application/wasm")
		}
		fs.ServeHTTP(w, r)
	})

	// 서버 시작
	log.Println("Starting server at port 8080...")
	log.Println("Open http://localhost:8080/examples/js/simple.html in your browser")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
