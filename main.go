package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
)

/*
 * gobench run multi client to GET/POST data to a specify url
 */

func must(err error) {
	if err != nil {
		pc := make([]uintptr, 10)
		runtime.Callers(2, pc)
		f := runtime.FuncForPC(pc[0])
		file, line := f.FileLine(pc[0])
		s := fmt.Sprintf("%s:%d %s [error]:%s", file, line, f.Name(), err.Error())
		panic(errors.New(s))
	}
}

func cleanPanic() {
	if e, ok := recover().(error); ok {
		fmt.Printf("%s\n\n", e)
		flag.Usage()
		os.Exit(-1)
	}
}

func run() {
	defer cleanPanic()
	initConfig()

	switch conf.Mode {
	case ModeQuery:
		runModeQuery()
	case ModeAgent:
		runModeAgent()
	case ModeCommand:
		runCommand()
	}
}

func main() {
	run()
}
