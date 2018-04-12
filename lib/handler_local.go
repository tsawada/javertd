package lib

import (
	"html/template"
	"log"
	"net/http"
	"runtime"
)

var templateHtml = `
<html>
<head>Statusz</head>
 <body>
  <p>Build with {{.Version}}</p>
  <table>
   <tr>
    <td>ReqID</td>
    <td>Timestamp</td>
    <td>Method</td>
    <td>UpClosed</td>
    <td>DownClosed</td>
    <td>URL</td>
   </tr>
   {{ range $id, $req := .Active }}
   <tr>
    <td>{{ $id }}</td>
    <td>{{ $req.Timestamp.String }}</td>
    <td>{{ $req.Method }}</td>
    <td>{{ $req.UpClosed }}</td>
    <td>{{ $req.DownClosed }}</td>
    <td>{{ $req.URL.String }}</td>
   </tr>
   {{ end }}
  </table>
 </body>
</html>`

var t = template.Must(template.New("n").Parse(templateHtml))

func (s *Server) localHandler(w http.ResponseWriter, req *http.Request) {
	//	http.NotFound(w, req)
	s.status(w, req)
}

type te struct {
	Version string
	Active  map[uint64]sReq
}

func unauthorized(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("WWW-Authenticate", `Basic realm="proxy"`)
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}

func (srv *Server) status(w http.ResponseWriter, req *http.Request) {
	if !srv.checkAuth(req, authorization) {
		unauthorized(w, req)
		return
	}

	srv.mu.Lock()
	err := t.Execute(w, &te{
		Version: runtime.Version(),
		Active:  srv.activeReqs,
	})
	srv.mu.Unlock()
	if err != nil {
		log.Printf("error: %v", err)
	}
}
