// Package buildnum contains stuff to do with generating build numbers.
package buildnum

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"

	"github.com/sirupsen/logrus"
)

const (
	// HealthPath is the URL path for the HTTP endpoint that returns health status.
	HealthPath = "/health"
	// ReadyPath URL path for the HTTP endpoint that returns ready status.
	ReadyPath = "/ready"
)

// HTTPBuildNumberServer runs an HTTP server to serve build numbers, similar to Prow's tot
// (https://github.com/kubernetes/test-infra/tree/master/prow/cmd/tot)
type HTTPBuildNumberServer struct {
	bindAddress string
	port        int
	path        string
	issuer      BuildNumberIssuer
}

// NewHTTPBuildNumberServer creates a new, initialised HTTPBuildNumberServer.
// Use 'bindAddress' to control the address/interface the HTTP service will listen on; to listen on all interfaces
// (i.e. 0.0.0.0 or ::) provide a blank string.
// Build numbers will be generated using the specified BuildNumberIssuer.
func NewHTTPBuildNumberServer(bindAddress string, port int, issuer BuildNumberIssuer) *HTTPBuildNumberServer {
	return &HTTPBuildNumberServer{
		bindAddress: bindAddress,
		port:        port,
		path:        "/vend/",
		issuer:      issuer,
	}
}

// Start the HTTP server.
// This call will block until the server exits.
func (s *HTTPBuildNumberServer) Start() error {
	mux := http.NewServeMux()
	mux.Handle(s.path, http.HandlerFunc(s.vend))
	mux.Handle(HealthPath, http.HandlerFunc(s.health))
	mux.Handle(ReadyPath, http.HandlerFunc(s.ready))

	log.Logger().Infof("Serving build numbers at http://%s:%d%s", s.bindAddress, s.port, s.path)
	return http.ListenAndServe(":"+strconv.Itoa(s.port), mux)
}

// health returns either HTTP 204 if the build number service is healthy, otherwise nothing ('cos it's dead).
func (s *HTTPBuildNumberServer) health(w http.ResponseWriter, r *http.Request) {
	log.Logger().Debug("Health check")
	w.WriteHeader(http.StatusNoContent)
}

// ready returns either HTTP 204 if the build number service is ready to serve /vend requests, otherwise HTTP 503.
func (s *HTTPBuildNumberServer) ready(w http.ResponseWriter, r *http.Request) {
	log.Logger().Debug("Ready check")
	if s.issuer.Ready() {
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

// Serve an incoming request to the server's base URL (default: /vend). The generated build number (or other
// output) will be written to the provided ResponseWriter.
func (s *HTTPBuildNumberServer) vend(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.generateBuildNumber(w, r)
	case http.MethodHead:
		log.Logger().Info("HEAD Todo...")
	case http.MethodPost:
		log.Logger().Info("POST Todo...")
	default:
		log.Logger().Errorf("Unsupported method %s for %s", r.Method, s.path)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

// Generate a build number, reading the pipeline ID from the Request and writing the build number (or error details)
// to the specified ResponseWriter.
func (s *HTTPBuildNumberServer) generateBuildNumber(w http.ResponseWriter, r *http.Request) {
	//Check for a pipeline identifier following the base path.
	if !(len(r.URL.Path) > len(s.path)) {
		msg := fmt.Sprintf("Missing pipeline identifier in URL path %s", r.URL.Path)
		log.Logger().Errorf(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	pipeline := r.URL.Path[len(s.path):]
	pID := kube.NewPipelineIDFromString(pipeline)
	buildNum, err := s.issuer.NextBuildNumber(pID)

	if err != nil {
		logrus.WithError(err).Errorf("Unable to get next build number for pipeline %s", pipeline)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	log.Logger().Infof("Vending build number %s for pipeline %s to %s.", buildNum, pipeline, r.RemoteAddr)
	fmt.Fprintf(w, "%s", buildNum)
}
