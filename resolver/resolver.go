package resolver

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/context"

	"github.com/ray-g/dnsproxy/logger"
	"github.com/ray-g/dnsproxy/utils"
)

type ResolvError struct {
	qname       string
	net         string
	nameservers []string
}

func (e ResolvError) Error() string {
	return fmt.Sprintf("%s resolv failed on %s (%s)", e.qname, strings.Join(e.nameservers, "; "), e.net)
}

type Resolver struct {
	config *dns.ClientConfig
}

func (r *Resolver) Lookup(net string, req *dns.Msg, timeout, interval int, nameServers []string, dohEnabled bool, dohEndpoint string) (*dns.Msg, error) {
	if dohEnabled && dohEndpoint != "" {
		if ans, err := r.DoHLookup(dohEndpoint, timeout, req); err == nil {
			return ans, nil
		} else {
			logger.Debugf("DoH Failed due to '%s' falling back to nameservers", err)
		}
	}
	return r.lookupWithNameservers(net, req, timeout, interval, nameServers)
}

func (r *Resolver) lookupWithNameservers(net string, req *dns.Msg, timeout, interval int, nameServers []string) (*dns.Msg, error) {
	qname := req.Question[0].Name
	timeo := r.Timeout(timeout)

	c := &dns.Client{
		Net:          net,
		ReadTimeout:  timeo,
		WriteTimeout: timeo,
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeo)
	defer cancel()

	res := make(chan *dns.Msg, 1)
	var wg sync.WaitGroup

	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()

	for _, nameserver := range nameServers {
		wg.Add(1)
		go func(ns string) {
			defer wg.Done()
			msg, _, err := c.ExchangeContext(ctx, req, ns)
			if err != nil {
				logger.Errorf("%s socket error on %s", qname, ns)
				logger.Errorf("error:%s", err.Error())
				return
			}
			if msg != nil && msg.Rcode != dns.RcodeSuccess {
				logger.Warningf("%s failed to get an valid answer on %s", qname, ns)
				if msg.Rcode == dns.RcodeServerFailure {
					return
				}
			} else {
				logger.Debugf("%s resolv on %s (%s)", utils.UnFqdn(qname), ns, net)
			}
			select {
			case res <- msg:
			default:
			}
		}(nameserver)

		select {
		case msg := <-res:
			return msg.Copy(), nil
		case <-ticker.C:
		}
	}

	wg.Wait()
	select {
	case msg := <-res:
		return msg.Copy(), nil
	default:
		return nil, ResolvError{qname, net, nameServers}
	}
}

func (r *Resolver) Timeout(timeout int) time.Duration {
	return time.Duration(timeout) * time.Second
}

func (r *Resolver) DoHLookup(url string, timeout int, req *dns.Msg) (*dns.Msg, error) {
	qname := req.Question[0].Name

	data, err := req.Pack()
	if err != nil {
		logger.Errorf("Failed to pack DNS message to wire format; %s", err)
		return nil, ResolvError{qname, "HTTPS", []string{url}}
	}

	client := http.Client{Timeout: r.Timeout(timeout)}
	resp, err := client.Post(url, "application/dns-message", bytes.NewReader(data))
	if err != nil {
		logger.Errorf("Request to DoH server failed; %s", err)
		return nil, ResolvError{qname, "HTTPS", []string{url}}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ResolvError{qname, "HTTPS", []string{url}}
	}
	if resp.Header.Get("Content-Type") != "application/dns-message" {
		return nil, ResolvError{qname, "HTTPS", []string{url}}
	}

	respPacket, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ResolvError{qname, "HTTPS", []string{url}}
	}

	var result dns.Msg
	if err := result.Unpack(respPacket); err != nil {
		logger.Errorf("Failed to unpack message from response; %s", err)
		return nil, ResolvError{qname, "HTTPS", []string{url}}
	}
	return &result, nil
}
