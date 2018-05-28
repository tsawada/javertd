package lib

import (
	"bufio"
	"fmt"
	"io"
	"log"
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
	} else {
		panic("doesn't support flush!")
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
	if hj, ok := w.(http.Hijacker); ok {
		// HTTP/1.x
		// XXX: this doesn't seem to remove all the headers
		header := w.Header()
		header["Content-Length"] = nil
		header["Content-Type"] = nil
		header["Transfer-Encoding"] = nil
		w.WriteHeader(200)

		local, bufrw, err := hj.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		hijackedHandler(conn, local, bufrw)
	} else {
		// HTTP/2.x
		w.Header()["Content-Type"] = nil
		w.Header()["Date"] = nil
		w.WriteHeader(200)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		} else {
			panic("no flusher")
		}
		log.Printf("Connected: %s", req.Host)
		complete := make(chan error)
		defer req.Body.Close()
		go func() {
			// src to dest
			_, err := io.Copy(conn, req.Body)

			srv.updateRequest(req, eventUpClosed)
			complete <- err
		}()
		go func() {
			// dest to src
			_, err := io.Copy(flushWriter{w}, conn)
			req.Body.Close()

			srv.updateRequest(req, eventDownClosed)
			complete <- err
		}()
		err1 := <-complete
		err2 := <-complete
		if err1 != nil {
			log.Printf("%s: %v", req.Host, err1)
		}
		if err2 != nil {
			log.Printf("%s: %v", req.Host, err2)
		}
	}
}
