package lib

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNoAuth(t *testing.T) {
	s := Server{Host: "example.com"}
	r, _ := http.NewRequest("GET", "http://other.com", strings.NewReader(""))
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, r)
	if rr.Code != http.StatusProxyAuthRequired {
		t.Errorf("got %v want %v", rr.Code, http.StatusProxyAuthRequired)
	}
	b := rr.Body.String()
	b_want := http.StatusText(http.StatusProxyAuthRequired) + "\n"
	if b != b_want {
		t.Errorf("got %#v want %#v", b, b_want)
	}
}

func TestGet(t *testing.T) {
	s := &Server{Host: "localhost", User: "user", Pass: "pass"}
	proxy := httptest.NewServer(s)
	defer proxy.Close()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()
	proxy_url, _ := url.Parse(proxy.URL)
	proxy_url.User = url.UserPassword("user", "pass")
	pt := &http.Transport{
		Proxy: http.ProxyURL(proxy_url),
	}
	c := &http.Client{Transport: pt}
	resp, err := c.Get(ts.URL)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Get failed: got %v want %v", resp.StatusCode, 200)
	}
}

func TestConnect(t *testing.T) {
	s := &Server{Host: "localhost", User: "user", Pass: "pass"}
	proxy := httptest.NewServer(s)
	defer proxy.Close()
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	ts.StartTLS()
	defer ts.Close()
	proxy_url, _ := url.Parse(proxy.URL)
	proxy_url.User = url.UserPassword("user", "pass")
	pt := &http.Transport{
		Proxy:           http.ProxyURL(proxy_url),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	c := &http.Client{Transport: pt}
	resp, err := c.Get(ts.URL)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Get failed: got %v want %v", resp.StatusCode, 200)
	}
}

func BenchmarkHello(b *testing.B) {
	for i := 0; i < b.N; i++ {
		fmt.Sprintf("hello")
	}
}
