package main

import (
	"fmt"
	"github.com/cespare/xxhash"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	"net/http"
	"os"
)

func main() {
	host, exists := os.LookupEnv("HOST")
	if !exists {
		host = "0.0.0.0"
	}

	port, exists := os.LookupEnv("PORT")
	if !exists {
		port = "8080"
	}

	router := httprouter.New()
	router.GET("/health", healthCheck)
	router.POST("/upload/:path", imageUpload)

	http.Handle("/", router)
	_ = http.ListenAndServe(fmt.Sprintf("%s:%s", host, port), nil)
}

func healthCheck(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	_, _ = fmt.Fprint(w, "ok")
}

func imageUpload(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	imageBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		println(err.Error())
		return
	}
	hash := xxhash.Sum64(imageBytes)

	_, _ = fmt.Fprint(w, hash)
}
