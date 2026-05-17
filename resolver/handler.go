package resolver

import (
	"net"
	"time"

	"github.com/miekg/dns"

	c "github.com/ray-g/dnsproxy/cache"
	r "github.com/ray-g/dnsproxy/cache/record"
	conf "github.com/ray-g/dnsproxy/config"
	h "github.com/ray-g/dnsproxy/hosts"
	"github.com/ray-g/dnsproxy/logger"
	"github.com/ray-g/dnsproxy/stats"
	"github.com/ray-g/dnsproxy/utils"
)

var (
	nullroute   = net.ParseIP("0.0.0.0")
	nullroutev6 = net.ParseIP("0:0:0:0:0:0:0:0")
)

type Question struct {
	Qname  string `json:"name"`
	Qtype  string `json:"type"`
	Qclass string `json:"class"`
}

func (q *Question) String() string {
	return q.Qname + " " + q.Qclass + " " + q.Qtype
}

type DNSHandler struct {
	config   *conf.DNSResolverConfig
	resolver *Resolver
	cache    c.Cache
	hosts    *h.Hosts
}

type DNSOperationData struct {
	Net string
	w   dns.ResponseWriter
	req *dns.Msg
}

func NewHandler(config *conf.DNSResolverConfig, cache c.Cache) *DNSHandler {
	handler := &DNSHandler{
		resolver: &Resolver{nil},
		cache:    cache,
		config:   config,
	}
	if config.Hosts.Enable {
		handler.hosts = h.NewHosts(&config.Hosts)
	}
	return handler
}

func (h *DNSHandler) do(Net string, w dns.ResponseWriter, req *dns.Msg) {
	stats.AddQuery()

	q := req.Question[0]
	Q := Question{utils.UnFqdn(q.Name), dns.TypeToString[q.Qtype], dns.ClassToString[q.Qclass]}
	key := Q.Qname
	IPQuery := utils.IsIPQuery(q)

	if stats.Active() {
		if IPQuery > 0 {
			if record, err := h.cache.Get(key); err != nil {
				logger.Debugf("%s didn't hit cache", Q.String())
			} else {
				if !record.Blocked {
					logger.Debugf("%s hit cache", Q.String())
					msg := *record.Msg
					msg.Id = req.Id
					h.WriteReplyMsg(w, &msg)
					return
				}
				logger.Debugf("%s hit cache and was blocked: forwarding request", Q.String())
				h.WriteReplyMsg(w, h.buildBlockedResponse(req, q, IPQuery))
				stats.AddQueryBlocked()
				logger.Noticef("%s found in blocklist", Q.Qname)
			}
		}

		if h.config.Hosts.Enable && IPQuery > 0 {
			if ips, ok := h.hosts.Get(Q.Qname, IPQuery); ok {
				mesg := h.buildHostsResponse(req, q, ips, IPQuery)
				w.WriteMsg(mesg)
				ttl := time.Duration(h.config.TTL) * time.Second
				h.cache.Set(key, r.NewCustomRecord(mesg, ttl))
				logger.Debug("%s found in hosts file", Q.Qname)
				stats.AddCustomDomain()
				return
			}
			logger.Debug("%s didn't found in hosts file", Q.Qname)
		}
	}

	mesg, err := h.resolver.Lookup(Net, req, h.config.Timeout, h.config.Interval, h.config.Nameservers, h.config.DoH.Enable, h.config.DoH.Endpoint)
	if err != nil {
		logger.Errorf("resolve query error %v", err)
		h.HandleFailed(w, req)
		return
	}

	if mesg.Truncated && Net == "udp" {
		mesg, err = h.resolver.Lookup("tcp", req, h.config.Timeout, h.config.Interval, h.config.Nameservers, h.config.DoH.Enable, h.config.DoH.Endpoint)
		if err != nil {
			logger.Errorf("resolve tcp query error %v", err)
			h.HandleFailed(w, req)
			return
		}
	}

	ttl := time.Duration(h.config.TTL) * time.Second
	for _, answer := range mesg.Answer {
		if candidate := time.Duration(answer.Header().Ttl) * time.Second; candidate > 0 && candidate < ttl {
			ttl = candidate
		}
	}

	h.WriteReplyMsg(w, mesg)

	if IPQuery > 0 && len(mesg.Answer) > 0 {
		if err = h.cache.Set(key, r.NewResolvedRecord(mesg, ttl)); err != nil {
			logger.Errorf("set %s cache failed: %v", Q.String(), err)
		}
		logger.Debugf("insert %s into cache with ttl %ds", Q.String(), ttl/time.Second)
		stats.AddNormalDomain()
	}
}

func (h *DNSHandler) buildBlockedResponse(req *dns.Msg, q dns.Question, IPQuery int) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(req)
	if h.config.NXDomainOnBlock {
		m.SetRcode(req, dns.RcodeNameError)
		return m
	}
	switch IPQuery {
	case utils.IPv4Query:
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: h.config.TTL},
			A:   nullroute,
		})
	case utils.IPv6Query:
		m.Answer = append(m.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: q.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: h.config.TTL},
			AAAA: nullroutev6,
		})
	}
	return m
}

func (h *DNSHandler) buildHostsResponse(req *dns.Msg, q dns.Question, ips []net.IP, IPQuery int) *dns.Msg {
	mesg := new(dns.Msg)
	mesg.SetReply(req)
	hdr := dns.RR_Header{Name: q.Name, Class: dns.ClassINET, Ttl: h.config.TTL}
	for _, ip := range ips {
		switch IPQuery {
		case utils.IPv4Query:
			hdr.Rrtype = dns.TypeA
			mesg.Answer = append(mesg.Answer, &dns.A{Hdr: hdr, A: ip})
		case utils.IPv6Query:
			hdr.Rrtype = dns.TypeAAAA
			mesg.Answer = append(mesg.Answer, &dns.AAAA{Hdr: hdr, AAAA: ip})
		}
	}
	return mesg
}

func (h *DNSHandler) DoTCP(w dns.ResponseWriter, req *dns.Msg) { h.do("tcp", w, req) }
func (h *DNSHandler) DoUDP(w dns.ResponseWriter, req *dns.Msg) { h.do("udp", w, req) }

func (h *DNSHandler) HandleFailed(w dns.ResponseWriter, message *dns.Msg) {
	m := new(dns.Msg)
	m.SetRcode(message, dns.RcodeServerFailure)
	h.WriteReplyMsg(w, m)
}

func (h *DNSHandler) WriteReplyMsg(w dns.ResponseWriter, message *dns.Msg) {
	defer func() {
		if rec := recover(); rec != nil {
			logger.Noticef("Recovered in WriteReplyMsg: %s", rec)
		}
	}()
	if err := w.WriteMsg(message); err != nil {
		logger.Error(err.Error())
	}
}
