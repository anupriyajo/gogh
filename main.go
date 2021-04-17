package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cespare/xxhash/v2"
	"github.com/go-redis/redis/v8"
	"github.com/julienschmidt/httprouter"
	"github.com/rwcarlsen/goexif/exif"
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
	http.ListenAndServe(fmt.Sprintf("%s:%s", host, port), nil)
}

func findCoords(imageBytes *bytes.Buffer) (float64, float64, error) {
	x, err := exif.Decode(imageBytes)
	if err != nil {
		return 0, 0, err
	}

	lat, long, err := x.LatLong()
	if err != nil {
		return 0, 0, err
	}

	return lat, long, nil
}

type LocationInfo struct {
	HouseNumber string `json:"house_number"`
	Road        string `json:"road"`
	Suburb      string `json:"suburb"`
	Borough     string `json:"borough"`
	City        string `json:"city"`
	Postcode    string `json:"postcode"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
}

type ApiResponse struct {
	Address LocationInfo `json:"address"`
}

func fetchLocation(lat float64, lon float64) (*LocationInfo, error) {
	apiUrl := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=json&lat=%f&lon=%f", lat, lon)
	resp, err := http.Get(apiUrl)
	if err != nil {
		return nil, err
	}

	var apiResponse ApiResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResponse)
	if err != nil {
		return nil, err
	}

	return &apiResponse.Address, nil
}

func imageUnique(imageBytes []byte) (bool, error) {
	knownImages := "images"
	imageHash := fmt.Sprintf("%x", xxhash.Sum64(imageBytes))
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
	fmt.Fprint(w, "ok")
}

func imageUpload(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	imageBytes, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		println(err.Error())
		return
	}

	lat, lon, _ := findCoords(bytes.NewBuffer(imageBytes))
	location, err := fetchLocation(lat, lon)
	println(fmt.Sprintf("%v", location))

	isUnique, err := imageUnique(imageBytes)
	if err != nil {
		println(err.Error())
		return
	}

	fmt.Fprint(w, isUnique)
}
