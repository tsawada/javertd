package lib

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
)

type flushWriter struct {
	w io.Writer
}

func (fw flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
	return
}

func hijackedHandler(conn net.Conn, local net.Conn, bufrw *bufio.ReadWriter) {
	defer local.Close()
	bufrw.Flush()
	complete := make(chan bool)
	go func() {
		io.Copy(conn, bufrw)
		complete <- true
	}()
	go func() {
		io.Copy(bufrw, conn)
		complete <- true
	}()
	<-complete
	<-complete
}

func connectHandler(w http.ResponseWriter, req *http.Request) {
	conn, err := net.Dial("tcp", req.Host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	header := w.Header()
	header["Content-Length"] = nil
	header["Content-Type"] = nil
	header["Transfer-Encoding"] = nil
	w.WriteHeader(200)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	if hj, ok := w.(http.Hijacker); ok {
		// HTTP/1.x
		local, bufrw, err := hj.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		hijackedHandler(conn, local, bufrw)
	} else {
		// HTTP/2.x
		complete := make(chan bool)
		go func() {
			io.Copy(conn, req.Body)
			req.Body.Close() // probably not needed
			complete <- true
		}()
		go func() {
			io.Copy(flushWriter{w}, conn)
			complete <- true
		}()
		<-complete
		<-complete
	}
}
