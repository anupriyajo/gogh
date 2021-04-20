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

func fetchLocation(lat float64, lon float64) (string, error) {
	apiUrl := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=json&lat=%f&lon=%f", lat, lon)
	resp, err := http.Get(apiUrl)
	if err != nil {
		return "", err
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(respBytes), nil
}

func resolveLocation(imageBytes []byte) (*LocationInfo, error) {
	knownImages := "images"
	imageHash := fmt.Sprintf("%x", xxhash.Sum64(imageBytes))
	result, _ := rdb.HGet(ctx, knownImages, imageHash).Result()
	if result == "" {
		lat, lon, err := findCoords(bytes.NewBuffer(imageBytes))
		if err != nil {
			return nil, err
		}

		result, err = fetchLocation(lat, lon)

		if err != nil {
			return nil, err
		}

		rdb.HSet(ctx, knownImages, imageHash, result).Result()
	}

	var response ApiResponse
	err := json.Unmarshal([]byte(result), &response)
	if err != nil {
		println("johari"+ err.Error())
		return nil, err
	}

	return &response.Address, nil
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

	location, err := resolveLocation(imageBytes)
	if err != nil {
		println(err.Error())
		return
	}

	fmt.Fprint(w, fmt.Sprintf("%v", location))
}
