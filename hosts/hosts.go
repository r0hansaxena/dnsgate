package hosts

import (
	"bufio"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"

	conf "github.com/ray-g/dnsproxy/config"
	"github.com/ray-g/dnsproxy/logger"
	"github.com/ray-g/dnsproxy/utils"
)

type Hosts struct {
	fileBackend     *FileHosts
	refreshInterval time.Duration
}

func NewHosts(hs *conf.HostsFileConfig) *Hosts {
	fileBackend := &FileHosts{
		file:  hs.HostsFile,
		hosts: make(map[string]string),
	}
	h := &Hosts{fileBackend, time.Second * time.Duration(hs.RefreshInterval)}
	h.refresh()
	return h
}

func (h *Hosts) Get(domain string, family int) ([]net.IP, bool) {
	sips, _ := h.fileBackend.Get(domain)
	if sips == nil {
		return nil, false
	}

	var ips []net.IP
	for _, sip := range sips {
		var ip net.IP
		switch family {
		case utils.IPv4Query:
			ip = net.ParseIP(sip).To4()
		case utils.IPv6Query:
			ip = net.ParseIP(sip).To16()
		default:
			continue
		}
		if ip != nil {
			ips = append(ips, ip)
		}
	}
	return ips, ips != nil
}

func (h *Hosts) refresh() {
	ticker := time.NewTicker(h.refreshInterval)
	go func() {
		for {
			h.fileBackend.Refresh()
			<-ticker.C
		}
	}()
}

type FileHosts struct {
	file  string
	hosts map[string]string
	mu    sync.RWMutex
}

func (f *FileHosts) Get(domain string) ([]string, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	domain = strings.ToLower(domain)
	if ip, ok := f.hosts[domain]; ok {
		return []string{ip}, true
	}

	sld, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return nil, false
	}

	for host, ip := range f.hosts {
		if strings.HasPrefix(host, "*.") {
			old, err := publicsuffix.EffectiveTLDPlusOne(host)
			if err != nil {
				continue
			}
			if sld == old {
				return []string{ip}, true
			}
		}
	}
	return nil, false
}

func (f *FileHosts) Refresh() {
	buf, err := os.Open(f.file)
	if err != nil {
		logger.Warn("Update hosts records from file failed %s", err)
		return
	}
	defer buf.Close()

	f.mu.Lock()
	defer f.mu.Unlock()

	f.hosts = make(map[string]string)

	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		f.parseLine(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		logger.Warn("Error reading hosts file: %s", err)
	}
	logger.Debug("update hosts records from %s, total %d records.", f.file, len(f.hosts))
}

func (f *FileHosts) parseLine(line string) {
	line = strings.TrimSpace(strings.Replace(line, "\t", " ", -1))
	if strings.HasPrefix(line, "#") || line == "" {
		return
	}
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return
	}
	ip := parts[0]
	if !utils.IsIP(ip) {
		return
	}
	for _, domain := range parts[1:] {
		domain = strings.TrimSpace(domain)
		if domain != "" {
			f.hosts[strings.ToLower(domain)] = ip
		}
	}
}
