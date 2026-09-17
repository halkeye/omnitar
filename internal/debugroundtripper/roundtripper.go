package debugroundtripper

import (
	"bytes"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
)

type RoundTripper struct {
	logger *logrus.Logger
}

func (t RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var err error
	var reqBody []byte
	var respBody []byte

	if req.Body != nil {
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			panic(err)
		}
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
	}
	t.logger.WithFields(logrus.Fields{
		"req.method":        req.Method,
		"req.body":          string(reqBody),
		"req.url":           req.URL.String(),
		"req.header":        req.Header,
		"req.form":          req.Form,
		"req.postform":      req.PostForm,
		"req.multipartform": req.MultipartForm,
	}).Debug("Making request")
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	if resp.Body != nil {
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
	}
	t.logger.WithField("resp.status", resp.Status).WithField("resp.body", string(respBody)).Debug("got response")
	// Do work after the response is received

	return resp, err
}
