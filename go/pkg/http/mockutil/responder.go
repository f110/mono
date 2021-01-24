package mockutil

import (
	"net/http"

	"github.com/jarcoal/httpmock"
)

func NewMultipleResponder(responders ...httpmock.Responder) httpmock.Responder {
	index := 0
	return func(req *http.Request) (*http.Response, error) {
		defer func() {
			index++
		}()

		if len(responders) <= index {
			return responders[len(responders)-1](req)
		}

		return responders[index](req)
	}
}
