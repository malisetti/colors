package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/EdlinOrg/prominentcolor"
	"github.com/mitchellh/hashstructure"
	"github.com/mseshachalam/colors/img"
	cache "github.com/patrickmn/go-cache"
)

// ErrorType is different app errors
type ErrorType string

const (
	// ErrNone no error
	ErrNone ErrorType = "no_error"
	// ErrSerialization errors that are caused due to serialization
	ErrSerialization = "serialization_error"
	// ErrSizeTooLarge errors that is caused due to size
	ErrSizeTooLarge = "size_too_large_error"
	// ErrUnknownDataFormat errors that are caused due to unknown data formats
	ErrUnknownDataFormat = "unknown_data_format_error"
	// ErrOthers other type of errors
	ErrOthers = "other_error"
)

// Error is error that has fields to contain about application errors
type Error struct {
	Message   string    `json:"msg"`
	ErrorType ErrorType `json:"type"`
}

// UploadType is the way of accessing an image
type UploadType string

const (
	// ImgURL says img can be fetched from url
	ImgURL UploadType = "url"
	// ImgBase64 says img can be fetched from base64 ecoded data
	ImgBase64 = "base64"
	// ImgUpload says img can be fetched from file upload
	ImgUpload = "file-upload"
)

// App holds things that are required for the app
type App struct {
	MaxBodySizeInBytes int64
	MaxProminentColors int
	Cache              *cache.Cache
	DiskCacheDir       string
}

// ProminentColorsFinderHandler exposes prominent colors functionality as an API
func (a *App) ProminentColorsFinderHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var response *HandleImgResponseBody
	// handle response forming here
	defer func() {
		responseBytes, err := json.Marshal(response)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "error: %s", err)
			return
		}

		if response.Error != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
		}

		w.Write(responseBytes)
	}()

	var buf bytes.Buffer
	handleImgRequestBody := new(HandleImgRequestBody)

	lr := io.LimitReader(io.TeeReader(r.Body, &buf), a.MaxBodySizeInBytes)
	dec := json.NewDecoder(lr)
	err := dec.Decode(handleImgRequestBody)
	if err != nil {
		_, _ = io.Copy(ioutil.Discard, r.Body)
		go func() {
			log.Printf("could not decode request body, failed with '%s'\n", err.Error())
			log.Println(buf.String()) // can be big
		}()
		// decode failed
		response.Error = CreateAppError(fmt.Errorf("could not decode request body, failed with '%s'", err.Error()), ErrSerialization)
		return
	}

	// look in cache
	if handleImgRequestBody.ProminentColors == 0 || handleImgRequestBody.ProminentColors > a.MaxProminentColors {
		handleImgRequestBody.ProminentColors = a.MaxProminentColors
	}

	var inCache bool

	defer func() {
		if inCache {
			return
		}
		uid, err := handleImgRequestBody.Hash(nil)
		if err == nil && response.Error == nil && len(response.ProminentColors) > 0 {
			a.Cache.Set(strconv.Itoa(int(uid)), response.ProminentColors, cache.DefaultExpiration)
		} else {
			log.Println(err)
		}
	}()

	h, err := handleImgRequestBody.Hash(nil)
	if err == nil {
		var cachedProminentColors []prominentcolor.ColorItem
		cachedData, inCache := a.Cache.Get(strconv.Itoa(int(h)))
		cachedProminentColors, ok := cachedData.([]prominentcolor.ColorItem)
		if inCache && ok && len(cachedProminentColors) > 0 {
			response.ProminentColors = img.TopColors(cachedProminentColors)
			return
		}
	}

	response = handleImgRequestBody.FindProminentColors(a.MaxBodySizeInBytes, a.DiskCacheDir)
	return
}

// HandleImgRequestBody is the request body when type is imgURL value is url when type is imgUpload value is base64 encoded image
type HandleImgRequestBody struct {
	Type            UploadType `json:"type"`
	Value           string     `json:"value"`
	ProminentColors int        `json:"num_prominent_colors"`
}

// Hash gives a hash to h
func (h *HandleImgRequestBody) Hash(opts *hashstructure.HashOptions) (uint64, error) {
	return hashstructure.Hash(h, opts)
}

// FindProminentColors converts request type to response type
func (h *HandleImgRequestBody) FindProminentColors(maxRequestBodySize int64, cacheDir string) *HandleImgResponseBody {
	var err error
	var errTyp ErrorType
	var colors []string

	colors, errTyp, err = func() ([]string, ErrorType, error) {
		var imgReader io.ReadCloser
		switch h.Type {
		case ImgURL:
			imgReader, err = img.GetReaderFromURL(h.Value, cacheDir)
			if err != nil {
				return nil, ErrOthers, err
			}
		case ImgBase64:
			imgReader, err = img.GetReaderFromBase64Data(h.Value)
			if err != nil {
				return nil, ErrOthers, err
			}
		default:
			return nil, ErrOthers, fmt.Errorf("requested type %s is not implemented for http requests", h.Type)
		}
		defer imgReader.Close()
		bufr := new(bytes.Buffer)
		const headerBufLen int64 = 512
		_, err = io.CopyN(bufr, imgReader, headerBufLen)
		if err != nil {
			return nil, ErrOthers, err
		}
		detectContentType := http.DetectContentType(bufr.Bytes())
		if !strings.HasPrefix(detectContentType, "image/") {
			return nil, ErrUnknownDataFormat, fmt.Errorf("%s may not be image", detectContentType)
		}
		_, err = io.CopyN(bufr, imgReader, maxRequestBodySize-headerBufLen)
		if err != nil && err != io.EOF {
			return nil, ErrSizeTooLarge, fmt.Errorf("%s %dmb is the limit of the acceptable image size", err, maxRequestBodySize>>20)
		}

		_, err = io.CopyN(ioutil.Discard, imgReader, 1)
		if err == nil {
			return nil, ErrSizeTooLarge, fmt.Errorf("%dmb is the limit of the acceptable image size", maxRequestBodySize>>20)
		}

		prominentColors, err := img.GetProminentColorsFromReader(bufr, h.ProminentColors)
		if err != nil {
			return nil, ErrOthers, err
		}

		return img.TopColors(prominentColors), ErrNone, nil
	}()

	response := new(HandleImgResponseBody)
	if err != nil {
		response.Error = CreateAppError(err, errTyp)
	} else {
		response.ProminentColors = colors
	}
	return response
}

// HandleImgResponseBody is the reponse body of the hanldeImg handler
type HandleImgResponseBody struct {
	ProminentColors []string `json:"prominent_colors"`
	Error           *Error   `json:"error"`
}

// CreateAppError creates an App Error
func CreateAppError(err error, typ ErrorType) *Error {
	return &Error{
		Message:   err.Error(),
		ErrorType: typ,
	}
}
