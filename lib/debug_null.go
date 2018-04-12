// +build !debug

package lib

import (
	"net/http"
)

type debugInfo struct {
}

func (*debugInfo) startRequest(req *http.Request) *http.Request {
	return req
}

func (*debugInfo) updateRequest(*http.Request, int) {
}

func (*debugInfo) endRequest(*http.Request) {
}

func (*Server) status(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (*debugInfo) logOutgoingRequest(*http.Request) {
}

func (*debugInfo) logResponse(*http.Response) {
}
