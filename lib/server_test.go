package lib

import (
	//"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	//"net/textproto"
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

func createEchoServer() net.Listener {
	ts := httptest.NewUnstartedServer(nil)
	l := ts.Listener
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				break
			}
			go func() {
				_, err := io.Copy(c, c)
				if err != nil {
					panic(fmt.Sprintf("failed to Copy: %v", err))
				}
				c.Close()
			}()
		}
	}()
	return l
}

func TestConnectHeader(t *testing.T) {
	s := &Server{Host: "localhost", AllowAnonymous: true}
	proxy := httptest.NewServer(s)
	defer proxy.Close()
	echo := createEchoServer()
	defer echo.Close()

	// http.Transport removes Transfer-Encoding automatically, thus not usable in this test
	//req, _ := http.NewRequest("CONNECT", proxy.URL, strings.NewReader(""))
	/*
		c := &http.Client{
			Transport: &http.Transport{
				DisableCompression: true},
		}
	*/
	c, err := net.DialTCP("tcp", nil, proxy.Listener.Addr().(*net.TCPAddr))
	w := "CONNECT " + echo.Addr().String() + " HTTP/1.1\r\n" +
		"Host: " + echo.Addr().String() + "\r\n\r\n\r\n"
	c.Write([]byte(w))
	c.CloseWrite()
	//textproto.NewReader(bufio.NewReader(c))
	res, err := ioutil.ReadAll(c)
	c.Close()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Response: %#v\n", string(res))

}

func BenchmarkHello(b *testing.B) {
	for i := 0; i < b.N; i++ {
		fmt.Sprintf("hello")
	}
}
