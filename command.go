package main

import (
	"errors"
	"fmt"
	"net/rpc"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	CmdExit  = "exit"
	CmdQuery = "query"

	AgentExit  = "Agent.Exit"
	AgentQuery = "Agent.Query"
)

type Command struct {
	Cmd string
}

func report(results []*Result, costTime time.Duration) {
	statusCount := make(CodeCounter)
	errCount := 0
	queryCount := 0
	t := time.Duration(0)
	sz := len(results)
	for _, r := range results {
		for k, n := range r.HttpStatusStat {
			if c, ok := statusCount[k]; ok {
				statusCount[k] = c + n
			} else {
				statusCount[k] = n
			}
			queryCount += n
		}
		errCount += r.ErrStat
		t += r.TimeStat
		fmt.Println(r)
	}

	fmt.Println("##################")
	fmt.Printf("time cost since send command:%s\nquery count:%d\nerr count:%d\nagent average cost time:%s\nhttp status:%v\n",
		costTime, queryCount, errCount, t/time.Duration(sz), statusCount)
	fmt.Printf("%0.3f times/second\n", float32(time.Duration(queryCount)*time.Second)/float32(costTime))
}

func callQuery(addrs []string, conf *QueryConfig) {
	wg := &sync.WaitGroup{}

	results := make([]*Result, 0)
	start := time.Now()
	for _, addr := range addrs {
		cli, err := rpc.DialHTTP("tcp", addr)
		must(err)
		reply := NewResult()
		doneCall := cli.Go(AgentQuery, conf, reply, nil)

		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			replyCall := <-doneCall.Done
			if replyCall.Error != nil {
				fmt.Fprintf(os.Stderr, "send query cmd to addr:%s error:%s", addr, replyCall.Error.Error())
			} else {
				results = append(results, replyCall.Reply.(*Result))
			}
		}(addr)
	}
	wg.Wait()

	report(results, time.Since(start))
}

func callExit(addrs []string) {
	for _, addr := range addrs {
		cli, err := rpc.DialHTTP("tcp", addr)
		must(err)
		reply := new(int)
		cli.Go(AgentExit, 1, reply, nil)
	}
}

func runCommand() {
	addrs := strings.Split(conf.cmdConf.CmdRecvAddrs, ";")
	switch conf.cmdConf.SendCmd {
	case CmdExit:
		callExit(addrs)
	case CmdQuery:
		callQuery(addrs, &conf.queryConf)
	default:
		must(errors.New("unsupport command"))
	}
}
