package main

import (
	"net"
	"net/http"
	"net/rpc"
	"os"
	"os/signal"
	"syscall"
)

type Agent struct {
	conf *agentConfig
	l    net.Listener
}

func NewAgent(conf *agentConfig) *Agent {
	a := &Agent{conf: conf}
	l, e := net.Listen("tcp", conf.AgentAddr)
	must(e)
	a.l = l
	return a
}

func (a *Agent) Query(conf *QueryConfig, reply *Result) error {
	ret := runQuery(conf)

	reply.HttpStatusStat = ret.HttpStatusStat
	reply.TimeStat = ret.TimeStat
	reply.ErrStat = ret.ErrStat

	if e, ok := recover().(error); ok {
		return e
	}
	return nil
}

func (a *Agent) Exit(code int, reply *int) error {
	a.Close()
	os.Exit(code)
	return nil
}

// start rpc service and keep on handling request
func (a *Agent) StartRPC() {
	err := rpc.Register(a)
	must(err)
	rpc.HandleHTTP()

	http.Serve(a.l, nil)
}

func (a *Agent) Close() {
	a.l.Close()
}

func handleSignal(a *Agent) {
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
		for {
			sig := <-ch
			switch sig {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM:
				a.Close()
				os.Exit(0)
			}
		}
	}()
}
func runModeAgent() {
	a := NewAgent(&conf.agentConf)
	handleSignal(a)
	a.StartRPC()
}
