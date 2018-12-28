package img

import (
	"encoding/base64"
	"fmt"
	"image"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/EdlinOrg/prominentcolor"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
)

// TopColors converts the given prominentColors to their hex representation without losing the original order
func TopColors(prominentColors []prominentcolor.ColorItem) (colorsInHex []string) {
	for _, prominentColor := range prominentColors {
		colorsInHex = append(colorsInHex, prominentColor.AsString())
	}
	return colorsInHex
}

// GetProminentColorsFromReader finds prominent colors from a reader
func GetProminentColorsFromReader(r io.Reader, n int) (rgbColors []prominentcolor.ColorItem, err error) {
	img, _, err := image.Decode(r)

	if err != nil {
		return nil, err
	}

	centroids, err := prominentcolor.KmeansWithAll(n, img, prominentcolor.ArgumentNoCropping, prominentcolor.DefaultSize, prominentcolor.GetDefaultMasks())
	if err != nil {
		return nil, err
	}

	return centroids, nil
}

// GetReaderFromFile returns a readcloser for given file
func GetReaderFromFile(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)

	return f, err
}

// GetReaderFromBase64Data returns a readcloser for given base64 data
func GetReaderFromBase64Data(data string) (r io.ReadCloser, err error) {
	r = ioutil.NopCloser(base64.NewDecoder(base64.StdEncoding, strings.NewReader(data)))
	return
}

// GetReaderFromURL returns a readcloser for given url
func GetReaderFromURL(url string, cacheDir string) (r io.ReadCloser, err error) {
	cli := http.Client{
		Transport: httpcache.NewTransport(diskcache.New(cacheDir)),
	}

	resp, err := cli.Get(url)
	if err != nil {
		return nil, err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "image/") {
		err = fmt.Errorf("%s may not be an image", contentType)
	}

	return resp.Body, err
}
