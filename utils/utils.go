package utils

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/miekg/dns"
	"github.com/ray-g/dnsproxy/logger"
)

const (
	NotIPQuery = 0
	IPv4Query  = 4
	IPv6Query  = 6
)

var (
	domainPattern = regexp.MustCompile(`^([a-zA-Z0-9\*]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,6}$`)

	queryTypeMap = map[uint16]int{
		dns.TypeA:    IPv4Query,
		dns.TypeAAAA: IPv6Query,
	}
)

func IsIPQuery(q dns.Question) int {
	if q.Qclass != dns.ClassINET {
		return NotIPQuery
	}
	if t, ok := queryTypeMap[q.Qtype]; ok {
		return t
	}
	return NotIPQuery
}

func UnFqdn(s string) string {
	if dns.IsFqdn(s) {
		return s[:len(s)-1]
	}
	return s
}

func IsDomain(domain string) bool {
	if IsIP(domain) {
		return false
	}
	return domainPattern.MatchString(domain)
}

func IsIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

func EnsureDirectory(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create folders: %s", path)
		}
		return nil
	}
	if err == nil && !info.IsDir() {
		return fmt.Errorf("%s exists but not a folder", path)
	}
	return nil
}

func newSignalChannel() <-chan os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGHUP)
	return ch
}

func WaitSysSignal() {
	sig := <-newSignalChannel()
	logger.Debugf("Received signal: %v", sig)
}
