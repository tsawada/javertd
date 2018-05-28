// +build debug

package lib

import (
	"context"
	//	"log"
	"net/http"
	//	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

const reqIDKey = 0

type sReq struct {
	Id         uint64
	Method     string
	URL        *url.URL
	Timestamp  time.Time
	UpClosed   bool
	DownClosed bool
}

type debugInfo struct {
	activeReqs map[uint64]sReq
	nextId     uint64
	mu         sync.Mutex
}

func getReqId(req *http.Request) uint64 {
	return req.Context().Value(reqIDKey).(uint64)
}

func setReqId(req *http.Request, reqId uint64) *http.Request {
	ctx := context.WithValue(req.Context(), reqIDKey, reqId)
	return req.WithContext(ctx)
}

// Request tracking for debugging purpose
func (srv *debugInfo) startRequest(req *http.Request) *http.Request {
	if srv.activeReqs == nil {
		srv.activeReqs = make(map[uint64]sReq, 100)
	}
	id := atomic.AddUint64(&srv.nextId, 1)
	req = setReqId(req, id)
	srv.mu.Lock()
	srv.activeReqs[id] = sReq{
		Id:        id,
		Method:    req.Method,
		URL:       req.URL,
		Timestamp: time.Now(),
	}
	srv.mu.Unlock()
	/*

		dump, err := httputil.DumpRequest(req, false)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("> %q\n", dump)
	*/

	return req
}

func (srv *debugInfo) updateRequest(req *http.Request, ev int) {
	id := getReqId(req)
	switch ev {
	case eventUpClosed:
		srv.mu.Lock()
		r := srv.activeReqs[id]
		r.UpClosed = true
		srv.activeReqs[id] = r
		srv.mu.Unlock()
	case eventDownClosed:
		srv.mu.Lock()
		r := srv.activeReqs[id]
		r.DownClosed = true
		srv.activeReqs[id] = r
		srv.mu.Unlock()
	}
}

func (srv *debugInfo) endRequest(req *http.Request) {
	id := getReqId(req)
	srv.mu.Lock()
	delete(srv.activeReqs, id)
	srv.mu.Unlock()
}

func (srv *debugInfo) logOutgoingRequest(req *http.Request) {
	/*
		dump, err := httputil.DumpRequestOut(req, false)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf(">> %q\n", dump)
	*/
}

func (srv *debugInfo) logResponse(r *http.Response) {
	/*
		dump, err := httputil.DumpResponse(r, false)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("<< %q\n", dump)
	*/

}
