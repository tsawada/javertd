package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/tsawada/proxy/lib"
)

var (
	port     = flag.Int("port", 1080, "HTTP port")
	host     = flag.String("hostname", "", "Serve Proxy on this hostname")
	user     = flag.String("username", "", "Username for Proxy auth")
	pass     = flag.String("password", "", "Password for Proxy auth")
	certFile = flag.String("cert", "", "Certificate file")
	keyFile  = flag.String("key", "", "Key file")
)

func flagCheck() error {
	flag.Parse()
	if *user == "" || *pass == "" {
		return errors.New("Please specify --username and --password")
	}
	if *host == "" {
		return errors.New("Please specify --hostname")
	}
	return nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if err := flagCheck(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
	m := &lib.Server{
		User: *user,
		Pass: *pass,
		Host: *host,
	}
	c := make(chan struct{})
	go func() {
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), m))
		c <- struct{}{}
	}()
	go func() {
		var certificate tls.Certificate
		var err error
		if *keyFile != "" && *certFile != "" {
			certificate, err = tls.LoadX509KeyPair(*certFile, *keyFile)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			cert, privKey := lib.SelfSigned(strings.Split(*host, ":")[0])
			certificate = tls.Certificate{
				Certificate: [][]byte{cert},
				PrivateKey:  privKey,
			}
			ioutil.WriteFile("privkey.pem", lib.PrivToPem(privKey), 0644)
		}
		s := http.Server{
			Addr:    ":8443",
			Handler: m,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{certificate},
			},
		}
		log.Fatal(s.ListenAndServeTLS("", ""))
		c <- struct{}{}
	}()
	<-c
	<-c
}
