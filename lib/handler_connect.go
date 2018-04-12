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

func hijackedHandler(remote *net.TCPConn, local net.Conn, bufrw *bufio.ReadWriter) {
	defer local.Close()
	bufrw.Flush()
	complete := make(chan bool)
	go func() {
		io.Copy(remote, bufrw)
		remote.CloseWrite()
		complete <- true
	}()
	go func() {
		io.Copy(bufrw, remote)
		bufrw.Flush()
		complete <- true
	}()
	<-complete
	<-complete
}

func (srv *Server) connectHandler(w http.ResponseWriter, req *http.Request) {
	addr, err := net.ResolveTCPAddr("tcp", req.Host)
	if err != nil {
		http.Error(w, "DNS Resolution Failed: "+req.Host, http.StatusBadGateway)
		return
	}
	if _, ok := srv.RestrictedPorts[addr.Port]; ok {
		http.Error(w, "Connection to port %d is restricted", http.StatusForbidden)
		return
	}
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	header := w.Header()
	// XXX: this doesn't seem to remove all the headers
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
			// src to dest
			io.Copy(conn, req.Body)
			conn.CloseWrite()
			req.Body.Close() // probably not needed

			srv.updateRequest(req, eventUpClosed)
			complete <- true
		}()
		go func() {
			// dest to src
			io.Copy(flushWriter{w}, conn)
			// workaround
			req.Body.Close()

			srv.updateRequest(req, eventDownClosed)
			complete <- true
		}()
		<-complete
		<-complete
	}
}
