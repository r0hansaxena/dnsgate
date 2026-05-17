package resolver

import (
	"time"

	"github.com/miekg/dns"

	"github.com/ray-g/dnsproxy/logger"
	"github.com/ray-g/dnsproxy/stats"
)

const (
	defaultReadTimeout  = 5 * time.Second
	defaultWriteTimeout = 5 * time.Second
)

type Server struct {
	addr      string
	rTimeout  time.Duration
	wTimeout  time.Duration
	handler   *DNSHandler
	udpServer *dns.Server
	tcpServer *dns.Server
}

func NewServer(addr string, handler *DNSHandler) *Server {
	return &Server{
		addr:     addr,
		handler:  handler,
		rTimeout: defaultReadTimeout,
		wTimeout: defaultWriteTimeout,
	}
}

func (s *Server) Run() {
	s.udpServer = s.newDNSServer("udp", s.handler.DoUDP)
	s.tcpServer = s.newDNSServer("tcp", s.handler.DoTCP)

	go s.start(s.udpServer)
	go s.start(s.tcpServer)
}

func (s *Server) newDNSServer(proto string, handlerFunc dns.HandlerFunc) *dns.Server {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", handlerFunc)

	srv := &dns.Server{
		Addr:         s.addr,
		Net:          proto,
		Handler:      mux,
		ReadTimeout:  s.rTimeout,
		WriteTimeout: s.wTimeout,
	}
	if proto == "udp" {
		srv.UDPSize = 65535
	}
	return srv
}

func (s *Server) start(ds *dns.Server) {
	logger.Infof("start %s listener on %s", ds.Net, s.addr)
	if err := ds.ListenAndServe(); err != nil {
		logger.Fatalf("start %s listener on %s failed: %s", ds.Net, s.addr, err.Error())
	}
}

func (s *Server) Stop() {
	stats.Deactivate()
	if s.udpServer != nil {
		s.udpServer.Shutdown()
	}
	if s.tcpServer != nil {
		s.tcpServer.Shutdown()
	}
}
