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

func getProxiedClient(s *httptest.Server) *http.Client {
	u, _ := url.Parse(s.URL)
	srv, ok := s.Config.Handler.(*Server)
	if !ok {
		panic("")
	}
	if !srv.AllowAnonymous {
		u.User = url.UserPassword(srv.User, srv.Pass)
	}
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(u)}}
}

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
	proxy := httptest.NewServer(&Server{Host: "localhost", User: "user", Pass: "pass"})
	defer proxy.Close()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()
	c := getProxiedClient(proxy)
	resp, err := c.Get(ts.URL)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Get failed: got %v want %v", resp.StatusCode, http.StatusOK)
	}
}

func TestConnect(t *testing.T) {
	proxy := httptest.NewServer(&Server{Host: "localhost", User: "user", Pass: "pass"})
	defer proxy.Close()
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	ts.StartTLS()
	defer ts.Close()
	c := getProxiedClient(proxy)
	c.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	resp, err := c.Get(ts.URL)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Get failed: got %v want %v", resp.StatusCode, 200)
	}
}

func createEchoServer() net.Listener {
	// Extract net.Listener from httptest.Server, to share httptest.serve flag
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

	// net/http modifies response headers, thus directly use TCPConn
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

func hijackHandler(f func(*http.Request) *http.Response) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			panic("Couln't hijack")
		}
		c, bufrw, err := hj.Hijack()
		if err != nil {
			panic("Hijack() failed")
		}
		f(r).Write(bufrw)
		bufrw.Flush()
		c.Close()
	})
}

// Even when remote server replies with HTTP/1.0, the proxy should use HTTP/1.1
func TestTransformHTTP10(t *testing.T) {
	proxy := httptest.NewServer(&Server{Host: "localhost", User: "user", Pass: "pass"})
	defer proxy.Close()
	ts := httptest.NewServer(hijackHandler(func(*http.Request) *http.Response {
		// Returns 200 OK HTTP/1.0
		return &http.Response{
			StatusCode: http.StatusOK,
			ProtoMajor: 1,
			ProtoMinor: 0,
		}
	}))
	defer ts.Close()

	c := getProxiedClient(proxy)
	resp, err := c.Get(ts.URL)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Get failed: got %v want %v", resp.StatusCode, 200)
	}
	if resp.ProtoMajor != 1 || resp.ProtoMinor != 1 {
		t.Errorf("got %v want %v", resp.Proto, "1.1")
	}
}

func BenchmarkGet(b *testing.B) {
	proxy := httptest.NewServer(&Server{Host: "localhost", User: "user", Pass: "pass"})
	defer proxy.Close()
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer ts.Close()
	c := getProxiedClient(proxy)

	for i := 0; i < b.N; i++ {
		resp, err := c.Get(ts.URL)
		if err != nil || resp.StatusCode != http.StatusOK {
			b.FailNow()
		}
	}
}
