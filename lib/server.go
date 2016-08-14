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
)

var (
	User string
	Pass string
	Host string
)

type Server struct {
	http.Server
}

func (s *Server) ListenAndServe() error {
	return s.Server.ListenAndServe()
}

func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	return s.Server.ListenAndServeTLS(certFile, keyFile)
}

func LocalHandler(w http.ResponseWriter, req *http.Request) {
	http.NotFound(w, req)
}

func checkAuth(r *http.Request) bool {
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

	return pair[0] == User && pair[1] == Pass
}

func ProxyAuthRequired(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Proxy-Authenticate", `Basic realm="MY REALM"`)
	//http.Error(w, "407 Proxy Authentication Required", http.StatusProxyAuthRequired)
	w.WriteHeader(407)
	w.Write([]byte(http.StatusText(http.StatusProxyAuthRequired)))
}

func HelloHandler(w http.ResponseWriter, req *http.Request) {
	dump, err := httputil.DumpRequest(req, false)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("> %q\n", dump)
	fmt.Printf("%s %s %s\n", req.URL.Scheme, req.URL.Host, req.URL.String())

	if req.Host == Host {
		LocalHandler(w, req)
		return
	}

	if !checkAuth(req) {
		fmt.Fprintf(os.Stderr, "auth req\n")
		ProxyAuthRequired(w, req)
		return
	}

	if req.Method == "CONNECT" {
		connectHandler(w, req)
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

	dump, err = httputil.DumpRequestOut(freq, false)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf(">> %q\n", dump)

	resp, err := http.DefaultClient.Do(freq)
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

	for k, h := range resp.Header {
		for _, v := range h {
			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Print(err)
	}
	resp.Body.Close()
}
