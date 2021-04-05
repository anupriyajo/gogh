package main

import (
	"context"
	"fmt"
	"github.com/cespare/xxhash"
	"github.com/go-redis/redis/v8"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	"net/http"
	"os"
)

var ctx = context.Background()

var rdb = redis.NewClient(&redis.Options{
	Addr:     "localhost:6379",
	Password: "", // no password set
	DB:       0,  // use default DB
})

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

func imageUnique(imageBytes []byte) (bool, error) {
	knownImages := "images"
	imageHash := xxhash.Sum64(imageBytes)
	result, err := rdb.SIsMember(ctx, knownImages, imageHash).Result()
	if err != nil {
		return false, err
	}
	if !result {
		rdb.SAdd(ctx, knownImages, imageHash)
	}

	return !result, nil
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

	isUnique, err := imageUnique(imageBytes)
	if err != nil {
		println(err.Error())
		return
	}

	_, _ = fmt.Fprint(w, isUnique)
}
