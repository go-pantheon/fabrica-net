package health

import (
	httpgo "net/http"

	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-kratos/swagger-api/openapiv2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	*http.Server
}

func NewServer(addr string) *Server {
	s := http.NewServer(http.Address(addr))

	s.HandlePrefix("/q/", openapiv2.NewHandler())
	s.Handle("/metrics", promhttp.Handler())
	s.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(httpgo.StatusOK)
	})

	return &Server{s}
}
