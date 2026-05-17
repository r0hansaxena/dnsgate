package stats

import (
	"sync"
	"sync/atomic"
	"time"
)

const maxQPSCount = 3600

type Stats struct {
	active        int32
	domainCount   int32
	domainNormal  int32
	domainBlocked int32
	domainCustom  int32
	queryCount    int32
	queryBlocked  int32
	qpsAverage    int32
	qps           []int32
	timeStarted   int64
	lastTime      int64
	lastCount     int32
}

var (
	globalStats Stats
	once        sync.Once
	stopCh      chan bool
	qpsTicker   *time.Ticker
)

func init() {
	globalStats.reset()
	once.Do(startQPSTracker)
}

func startQPSTracker() {
	stopCh = make(chan bool)
	qpsTicker = time.NewTicker(100 * time.Millisecond)
	go func() {
		for {
			select {
			case <-stopCh:
				return
			case <-qpsTicker.C:
				globalStats.calcQPS()
			}
		}
	}()
}

func increase(c *int32) { atomic.AddInt32(c, 1) }
func decrease(c *int32) { atomic.AddInt32(c, -1) }
func reset(c *int32)    { atomic.StoreInt32(c, 0) }

func (s *Stats) reset() {
	reset(&s.domainCount)
	reset(&s.domainNormal)
	reset(&s.domainBlocked)
	reset(&s.domainCustom)
	reset(&s.queryCount)
	reset(&s.queryBlocked)
	reset(&s.qpsAverage)
	atomic.StoreInt64(&s.timeStarted, time.Now().Unix())
	s.qps = make([]int32, 0)
}

func (s *Stats) addNormalDomain()   { increase(&s.domainNormal); increase(&s.domainCount) }
func (s *Stats) removeNormalDomain() { decrease(&s.domainNormal); decrease(&s.domainCount) }
func (s *Stats) addCustomDomain()   { increase(&s.domainCustom); increase(&s.domainCount) }
func (s *Stats) removeCustomDomain() { decrease(&s.domainCustom); decrease(&s.domainCount) }
func (s *Stats) addBlockedDomain()  { increase(&s.domainBlocked); increase(&s.domainCount) }
func (s *Stats) removeBlockedDomain() { decrease(&s.domainBlocked); decrease(&s.domainCount) }
func (s *Stats) addQuery()          { increase(&s.queryCount) }
func (s *Stats) addQueryBlocked()   { increase(&s.queryBlocked) }
func (s *Stats) activate()          { atomic.CompareAndSwapInt32(&s.active, 0, 1) }
func (s *Stats) deactivate()        { atomic.CompareAndSwapInt32(&s.active, 1, 0) }
func (s *Stats) Active() bool       { return atomic.LoadInt32(&s.active) == 1 }

func (s *Stats) calcQPS() {
	now := time.Now().Unix()
	length := len(s.qps)
	lastIdx := length - 1
	count := atomic.LoadInt32(&s.queryCount)
	delta := count - s.lastCount
	if s.lastTime == now {
		s.qps[lastIdx] = s.qps[lastIdx] + delta
	} else if s.lastTime < now {
		if length >= maxQPSCount {
			s.qps = s.qps[1:]
		}
		s.qps = append(s.qps, delta)
	}
	elapsed := now - s.timeStarted
	if elapsed > 0 {
		s.qpsAverage = count / int32(elapsed)
	} else {
		s.qpsAverage = count
	}
	s.lastCount = s.lastCount + delta
	s.lastTime = now
}

func (s *Stats) DomainCount() int32  { return atomic.LoadInt32(&s.domainCount) }
func (s *Stats) DomainNormal() int32 { return atomic.LoadInt32(&s.domainNormal) }
func (s *Stats) DomainBlocked() int32 { return atomic.LoadInt32(&s.domainBlocked) }
func (s *Stats) DomainCustom() int32 { return atomic.LoadInt32(&s.domainCustom) }
func (s *Stats) QueryCount() int32   { return atomic.LoadInt32(&s.queryCount) }
func (s *Stats) QueryBlocked() int32 { return atomic.LoadInt32(&s.queryBlocked) }
func (s *Stats) QpsAverage() int32   { return atomic.LoadInt32(&s.qpsAverage) }
func (s *Stats) TimeStarted() int64  { return atomic.LoadInt64(&s.timeStarted) }
func (s *Stats) LastTime() int64     { return atomic.LoadInt64(&s.lastTime) }
func (s *Stats) LastCount() int32    { return atomic.LoadInt32(&s.lastCount) }

func (s *Stats) Qps() []int32 {
	qps := make([]int32, len(s.qps))
	copy(qps, s.qps)
	return qps
}

func (s *Stats) Dump() map[string]interface{} {
	return map[string]interface{}{
		"domain_count":   s.DomainCount(),
		"domain_normal":  s.DomainNormal(),
		"domain_blocked": s.DomainBlocked(),
		"domain_custom":  s.DomainCustom(),
		"query_count":    s.QueryCount(),
		"query_blocked":  s.QueryBlocked(),
		"qps_average":    s.QpsAverage(),
		"time_started":   s.TimeStarted(),
		"time_last":      s.LastTime(),
		"qps":            s.Qps(),
	}
}

func AddNormalDomain()   { globalStats.addNormalDomain() }
func RemoveNormalDomain() { globalStats.removeNormalDomain() }
func AddCustomDomain()   { globalStats.addCustomDomain() }
func RemoveCustomDomain() { globalStats.removeCustomDomain() }
func AddBlockedDomain()  { globalStats.addBlockedDomain() }
func RemoveBlockedDomain() { globalStats.removeBlockedDomain() }
func AddQuery()          { globalStats.addQuery() }
func AddQueryBlocked()   { globalStats.addQueryBlocked() }
func Active() bool       { return globalStats.Active() }
func Activate()          { globalStats.activate() }
func Deactivate()        { globalStats.deactivate() }
func Dump() map[string]interface{} { return globalStats.Dump() }

func Stop() {
	stopCh <- true
}
