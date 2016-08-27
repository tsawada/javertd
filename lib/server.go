package lib

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Public",
	"Proxy-Authenticate",
	"Transfer-Encoding",
	"Upgrade"}

type Server struct {
	User            string
	Pass            string
	Host            string
	AllowAnonymous  bool
	RestrictedPorts map[int]struct{}
}

func (srv *Server) checkAuth(r *http.Request) bool {
	if srv.AllowAnonymous {
		r.Header.Del("Proxy-Authorization")
		return true
	}
	s := strings.SplitN(r.Header.Get("Proxy-Authorization"), " ", 2)
	if len(s) != 2 {
		return false
	}
	r.Header.Del("Proxy-Authorization")
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
	dump, err := httputil.DumpRequest(req, false)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("> %q\n", dump)
	fmt.Printf("%s %s %s\n", req.URL.Scheme, req.URL.Host, req.URL.String())

	if req.Host == srv.Host {
		localHandler(w, req)
		return
	}

	if !srv.checkAuth(req) {
		fmt.Fprintf(os.Stderr, "auth req\n")
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
	fmt.Printf(">> %q\n", dump)

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
	fmt.Printf("<< %q\n", dump)

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
