package main

import (
	"flag"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/mseshachalam/colors/app"
	cache "github.com/patrickmn/go-cache"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// make these configurable
const (
	defaultMaxRequestBodySize int64  = 1 // in mb
	defaultMaxProminentColors int    = 5
	defaultPort               string = ":8080"
	defaultDiskCacheDir       string = "./cache"
)

func main() {
	// receive flags
	portPtr := flag.String("port", defaultPort, "port number for the server to run on")
	maxReqBodySizePtr := flag.Int64("max-req-body-size", defaultMaxRequestBodySize, "maximum request body size in mb")
	maxProminentColorsPtr := flag.Int("max-prominent-colors", defaultMaxProminentColors, "maximum promiment colors that can be used to limit the user's choice")
	diskCacheDirPtr := flag.String("disk-cache-dir", defaultDiskCacheDir, "disk cache directory")

	flag.Parse()

	var c = cache.New(5*time.Minute, 10*time.Minute)
	absPath, err := filepath.Abs(*diskCacheDirPtr)
	if err != nil {
		log.Fatalln(err)
	} else {
		log.Printf("using %s as cache dir while fetching images from urls\n", absPath)
	}

	app := &app.App{
		MaxBodySizeInBytes: *maxReqBodySizePtr << 20, // in bytes
		MaxProminentColors: *maxProminentColorsPtr,
		Cache:              c,
		DiskCacheDir:       absPath,
	}

	r := mux.NewRouter()
	r.HandleFunc("/", app.ProminentColorsFinderHandler).Methods(http.MethodPost).Headers("Content-Type", "application/json") // add rate limiting

	srv := &http.Server{
		Handler: handlers.CORS()(r),
		Addr:    *portPtr,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 2 * time.Second,
		ReadTimeout:  2 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
