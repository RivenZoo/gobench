package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
)

const (
	ModeQuery   = "query"
	ModeAgent   = "agent"
	ModeCommand = "cmd"
)

const (
	DataBase64       = iota
	defaultAgentAddr = ":9012"
)

var (
	conf config
)

type config struct {
	Mode      string
	Addrs     string
	queryConf QueryConfig
	agentConf agentConfig
	cmdConf   commandConfig
}

func (c config) String() string {
	s, err := json.Marshal(&c)
	if err != nil {
		return "{}"
	}
	return string(s)
}

type QueryConfig struct {
	DestUrl     string
	Client      int
	QueryCount  int
	Method      string
	Data        string
	DataType    string
	Headers     string
}

func (qc *QueryConfig) check() {
	u, err := url.ParseRequestURI(qc.DestUrl)
	must(err)
	if u.Scheme != "http" {
		must(errors.New("need right http url"))
	}
	m := strings.ToUpper(qc.Method)
	if m != "GET" && m != "POST" {
		must(errors.New("method not support"))
	}
	if qc.Data != "" {
		qc.Method = "POST"
	}
	if qc.DataType != "" && qc.DataType != "base64" {
		must(errors.New("post data encode type not support"))
	}

	if qc.DataType == "base64" {
		d, err := base64.StdEncoding.DecodeString(qc.Data)
		must(err)
		qc.Data = string(d)
	}
}

type agentConfig struct {
	AgentAddr string
}

func (ac *agentConfig) check() {
	if ac.AgentAddr == "" {
		ac.AgentAddr = defaultAgentAddr
	}
	parts := strings.Split(ac.AgentAddr, ":")
	if len(parts) == 0 {
		must(errors.New("agent need ip and port"))
	}
}

type commandConfig struct {
	CmdRecvAddrs string // split by ';'
	SendCmd      string
}

func (cc *commandConfig) check() {
	cmd := strings.ToLower(cc.SendCmd)
	if cmd != CmdQuery && cmd != CmdExit {
		must(errors.New("unsupported command"))
	}
	cc.SendCmd = cmd

	addrs := strings.Split(cc.CmdRecvAddrs, ";")
	if len(addrs) == 0 {
		must(errors.New("command need receive address"))
	}
}

func checkParam() {
	if conf.Mode == "" {
		conf.Mode = ModeQuery
	}

	switch conf.Mode {
	case ModeQuery:
		conf.queryConf.check()
	case ModeAgent:
		conf.agentConf.AgentAddr = conf.Addrs
		conf.agentConf.check()
	case ModeCommand:
		conf.cmdConf.CmdRecvAddrs = conf.Addrs
		conf.cmdConf.check()
		if conf.cmdConf.SendCmd == CmdQuery {
			conf.queryConf.check()
		}
	}
}

func initConfig() {
	usage := flag.Usage
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "gobench use three modes, default query, run query test\n"+
			"mode agent, receive command and report result, set addrs to agent address\n"+
			"mode command, send command to agent, set addrs to receive address\n")
		usage()
	}

	flag.StringVar(&conf.queryConf.DestUrl, "url", "", "url to query, support http")
	flag.IntVar(&conf.queryConf.Client, "cli", 1, "query client number")
	flag.IntVar(&conf.queryConf.QueryCount, "N", 1, "query count")
	flag.StringVar(&conf.queryConf.Method, "m", "GET", "query method, GET or POST")
	flag.StringVar(&conf.queryConf.Data, "d", "", "post data to url")
	flag.StringVar(&conf.queryConf.DataType, "en", "", "use to decode param -d as post data, support:base64")
	flag.StringVar(&conf.queryConf.Headers, "H", "", `set request header, multi split by';', eg. "Host:abc.com;Agent:Firefox"`)

	flag.StringVar(&conf.Mode, "mode", "query", "mode:[agent|cmd|query]")
	flag.StringVar(&conf.Addrs, "addrs", "", "address 'ip:port' concatenated by ';'")

	flag.StringVar(&conf.cmdConf.SendCmd, "cmd", "", "send cmd to agent, support command:[query|exit]")

	flag.Parse()

	checkParam()
}
