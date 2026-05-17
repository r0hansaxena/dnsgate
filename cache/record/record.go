package record

import (
	"net"
	"time"

	"github.com/miekg/dns"
)

type Record struct {
	Blocked  bool      `json:"blocked"`
	NoExpire bool      `json:"no_expire"`
	UpdateAt time.Time `json:"update_at"`
	ExpireAt time.Time `json:"expire_at"`
	Msg      *dns.Msg
}

func (r *Record) Expired() bool {
	return !r.NoExpire && r.ExpireAt.Before(time.Now())
}

func newRecord(msg *dns.Msg, blocked, noexpire bool, ttl time.Duration) *Record {
	now := time.Now()
	return &Record{
		Msg:      msg,
		Blocked:  blocked,
		NoExpire: noexpire,
		UpdateAt: now,
		ExpireAt: now.Add(ttl),
	}
}

func NewResolvedRecord(msg *dns.Msg, ttl time.Duration) *Record {
	return newRecord(msg, false, false, ttl)
}

func NewCustomRecord(msg *dns.Msg, ttl time.Duration) *Record {
	return newRecord(msg, false, false, ttl)
}

func NewBlockedRecord() *Record {
	return newRecord(blockedMessage, true, true, 0)
}

var blockedMessage = buildBlockedMessage()

func buildBlockedMessage() *dns.Msg {
	m := new(dns.Msg)
	m.Answer = append(m.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   "domain.blocked",
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    600,
		},
		A: net.ParseIP("0.0.0.0"),
	})
	return m
}
