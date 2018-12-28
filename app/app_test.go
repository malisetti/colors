package app

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// https://blog.questionable.services/article/testing-http-handlers-go/
func TestProminentColorsFinderHandler(t *testing.T) {
	input, _ := os.Open("../input.json")
	defer input.Close()

	req, _ := http.NewRequest("POST", "/", input)

	rr := httptest.NewRecorder()
	app := &App{
		MaxBodySizeInBytes: 1 << 20, // in bytes
		MaxProminentColors: 5,
		Cache:              nil,
		DiskCacheDir:       "../cache",
	}
	handler := http.HandlerFunc(app.ProminentColorsFinderHandler)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func BenchmarkProminentColorsFinderHandler(b *testing.B) {
	input, _ := os.Open("../input.json")
	defer input.Close()

	buf, _ := ioutil.ReadAll(input)
	app := &App{
		MaxBodySizeInBytes: 1 << 20, // in bytes
		MaxProminentColors: 5,
		Cache:              nil,
		DiskCacheDir:       "../cache",
	}
	handler := http.HandlerFunc(app.ProminentColorsFinderHandler)
	rr := httptest.NewRecorder()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", "/", bytes.NewReader(buf))
		// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
		// directly and pass in our Request and ResponseRecorder.
		handler.ServeHTTP(rr, req)

		// Check the status code is what we expect.
		if status := rr.Code; status != http.StatusOK {
			b.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}
	}
}
