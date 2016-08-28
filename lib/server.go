package lib

import (
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

var hopByHopHeaders = []string{
	http.CanonicalHeaderKey("Connection"),
	http.CanonicalHeaderKey("Keep-Alive"),
	http.CanonicalHeaderKey("Public"),
	http.CanonicalHeaderKey("Proxy-Authenticate"),
	http.CanonicalHeaderKey("Proxy-Authorization"),
	http.CanonicalHeaderKey("TE"),
	http.CanonicalHeaderKey("Transfer-Encoding"),
	http.CanonicalHeaderKey("Upgrade"),
}

var proxyAuthorization = http.CanonicalHeaderKey("Proxy-Authorization")
var authorization = http.CanonicalHeaderKey("Authorization")

type sReq struct {
	Id        uint64
	Method    string
	URL       *url.URL
	Timestamp time.Time
}

type Server struct {
	User            string
	Pass            string
	Host            string
	AllowAnonymous  bool
	RestrictedPorts map[int]struct{}

	activeReqs map[uint64]sReq
	nextId     uint64
}

func (srv *Server) getReqId() uint64 {
	return atomic.AddUint64(&srv.nextId, 1)
}

func (srv *Server) checkAuth(r *http.Request, h string) bool {
	if srv.AllowAnonymous {
		r.Header.Del(h)
		return true
	}
	s := strings.SplitN(r.Header.Get(h), " ", 2)
	if len(s) != 2 {
		return false
	}
	r.Header.Del(h)
	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		return false
	}

	pair := strings.SplitN(string(b), ":", 2)
	if len(pair) != 2 {
		return false
	}

	return pair[0] == srv.User && pair[1] == srv.Pass
}

func proxyAuthRequired(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Proxy-Authenticate", `Basic realm="proxy"`)
	http.Error(w, http.StatusText(http.StatusProxyAuthRequired), http.StatusProxyAuthRequired)
}

func (srv *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := context.Background()
	id := srv.getReqId()
	if srv.activeReqs == nil {
		srv.activeReqs = make(map[uint64]sReq, 100)
	}
	srv.activeReqs[id] = sReq{
		Id:        id,
		Method:    req.Method,
		URL:       req.URL,
		Timestamp: time.Now(),
	}
	defer delete(srv.activeReqs, id)

	dump, err := httputil.DumpRequest(req, false)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("> %q\n", dump)

	if req.Host == srv.Host {
		srv.localHandler(w, req)
		return
	}

	if !srv.checkAuth(req, proxyAuthorization) {
		proxyAuthRequired(w, req)
		return
	}

	if req.Method == "CONNECT" {
		srv.connectHandler(w, req)
		return
	}

	var turl string
	if req.URL.Scheme != "" {
		turl = req.URL.String()
	} else {
		turl = "http://" + req.Host + req.URL.String()
	}
	freq, err := http.NewRequest(req.Method, turl, req.Body)
	if err != nil {
		log.Fatal(err)
	}
	freq.Header = req.Header

	// Remove hop-by-hop headers
	for _, v := range req.Header["Connection"] {
		req.Header[v] = nil
	}
	for _, v := range hopByHopHeaders {
		req.Header[v] = nil
	}

	dump, err = httputil.DumpRequestOut(freq, false)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf(">> %q\n", dump)

	resp, err := ctxhttp.Do(ctx, nil, freq)
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dump, err = httputil.DumpResponse(resp, false)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("<< %q\n", dump)

	h := w.Header()
	for k, v := range resp.Header {
		h[k] = v
	}

	// Public header MUST be removed. RFC2068 Section 14.35 Public
	h["Public"] = nil

	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Print(err)
	}
	resp.Body.Close()
}
