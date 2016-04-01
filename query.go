package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type CodeCounter map[int]int

func (cc CodeCounter) GobEncode() ([]byte, error) {
	buf := make([]byte, 0, len(cc)*6)
	w := bytes.NewBuffer(buf)
	for k, n := range cc {
		binary.Write(w, binary.LittleEndian, int16(k))
		binary.Write(w, binary.LittleEndian, int32(n))
	}
	return w.Bytes(), nil
}

func (cc CodeCounter) GobDecode(data []byte) error {
	rd := bytes.NewReader(data)
	for rd.Len() >= 6 {
		code := int16(0)
		n := int32(0)
		binary.Read(rd, binary.LittleEndian, &code)
		binary.Read(rd, binary.LittleEndian, &n)
		if c, ok := cc[int(code)]; ok {
			cc[int(code)] = c + int(n)
		} else {
			cc[int(code)] = int(n)
		}
	}
	return nil
}

type Result struct {
	HttpStatusStat CodeCounter
	TimeStat       time.Duration
	ErrStat        int
}

func NewResult() *Result {
	return &Result{
		HttpStatusStat: make(CodeCounter),
	}
}

func (r *Result) String() string {
	buf := &bytes.Buffer{}
	buf.WriteString(`{"httpstatus":[`)
	sz := len(r.HttpStatusStat)
	i := 0
	for s, n := range r.HttpStatusStat {
		fmt.Fprintf(buf, `{"%d":%d}`, s, n)
		if i != sz-1 {
			buf.WriteString(",")
		}
		i++
	}
	fmt.Fprintf(buf, `],"time":"%s","err":%d}`, r.TimeStat, r.ErrStat)
	return buf.String()
}

type statCollector struct {
	wg           *sync.WaitGroup
	downCounter  *DownCounter
	start, end   time.Time
	tmLock       *sync.Mutex
	HttpStatusCh chan int
	UsedTime     time.Duration
	ErrCh        chan error
}

func newCollector(cli, queryCount int) *statCollector {
	c := &statCollector{
		wg:           &sync.WaitGroup{},
		start:        time.Now().Add(time.Hour),      // init start time after now
		end:          time.Now().Add(-1 * time.Hour), // init end time before now
		downCounter:  NewDownCounter(queryCount, cli),
		tmLock:       &sync.Mutex{},
		HttpStatusCh: make(chan int, queryCount),
		ErrCh:        make(chan error, cli),
	}
	return c
}

func (c *statCollector) next() bool {
	return c.downCounter.Borrow()
}

func (c *statCollector) collectError(e error) {
	c.ErrCh <- e
	c.downCounter.Payback()
}

func (c *statCollector) collectStatus(code int) {
	c.HttpStatusCh <- code
	c.downCounter.Payback()
}

func (c *statCollector) recordStart() {
	c.tmLock.Lock()
	defer c.tmLock.Unlock()
	t := time.Now()
	if t.Before(c.start) {
		c.start = t
	}
}

func (c *statCollector) recordEnd() {
	c.tmLock.Lock()
	defer c.tmLock.Unlock()
	t := time.Now()
	if t.After(c.end) {
		c.end = t
	}
}

func (c *statCollector) calcTime() {
	c.UsedTime = c.end.Sub(c.start)
}

func (c *statCollector) close() {
	close(c.HttpStatusCh)
	close(c.ErrCh)
	c.wg.Wait()
}

func (c *statCollector) collect() *Result {
	ret := NewResult()
	ret.TimeStat = c.UsedTime

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for _ = range c.ErrCh {
			ret.ErrStat++
		}

		for statusCode := range c.HttpStatusCh {
			if n, ok := ret.HttpStatusStat[statusCode]; ok {
				ret.HttpStatusStat[statusCode] = n + 1
			} else {
				ret.HttpStatusStat[statusCode] = 1
			}
		}
	}()
	return ret
}

func setHeaders(req *http.Request, Headers string) {
	if Headers != "" {
		if req.Header == nil {
			req.Header = make(http.Header)
		}

		heads := strings.Split(Headers, ";")
		for _, val := range heads {
			p := strings.Split(val, ":")
			k, v := p[0], ""
			if len(p) > 1 {
				v = p[1]
			}

			v = strings.TrimSpace(v)
			if strings.ToLower(k) == "host" {
				req.Host = v
			}
			req.Header.Set(k, v)
		}
	}
}

func runClient(conf *QueryConfig, collector *statCollector, wg *sync.WaitGroup) {
	rd := strings.NewReader(conf.Data)
	req, err := http.NewRequest(conf.Method, conf.DestUrl, rd)
	must(err)
	setHeaders(req, conf.Headers)

	cli := &http.Client{
		Timeout: time.Second * 10,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   60 * time.Second,
				KeepAlive: 60 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			MaxIdleConnsPerHost:   50000,
		},
	}

	sched := make(chan struct{})
	wg.Add(1)
	go func() {
		defer func() {
			collector.recordEnd()
			if e, ok := recover().(error); ok {
				fmt.Fprintf(os.Stderr, "[error]:%s", e.Error())
				collector.collectError(e)
			}
			wg.Done()
		}()
		close(sched)
		collector.recordStart()
		for collector.next() {
			resp, err := cli.Do(req)
			must(err)

			io.Copy(ioutil.Discard, resp.Body)
			resp.Body.Close()

			collector.collectStatus(resp.StatusCode)
			rd.Seek(0, 0)
		}
	}()
	<-sched
}

func runQuery(conf *QueryConfig) *Result {
	queryNum := conf.QueryCount
	collector := newCollector(conf.Client, queryNum)

	wg := &sync.WaitGroup{}
	for i := 0; i < conf.Client; i++ {
		runClient(conf, collector, wg)
	}
	wg.Wait()
	collector.calcTime()

	stats := collector.collect()
	collector.close()
	return stats
}

func runModeQuery() {
	stats := runQuery(&conf.queryConf)
	queryNum := conf.queryConf.QueryCount

	fmt.Println("##################")
	fmt.Printf("query count:%d\nerr count:%d\nhttp status:%v\n%0.3f times/second\n",
		queryNum, stats.ErrStat, stats.HttpStatusStat,
		float32(time.Duration(queryNum)*time.Second)/float32(stats.TimeStat))
}
