package main

import (
	"fmt"
	"github.com/wuqtao/futuapi/query"
	"github.com/wuqtao/futuapi/quote"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	quoteContext, err := quote.NewQuote("127.0.0.1", "11111")
	if err != nil {
		panic(err)
	}
	defer quoteContext.Close()

	snap, err := query.GetSecuritySnapshot([]string{"SZ300001"})
	fmt.Println(snap)
	// signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		log.Printf("get a signal %s", s.String())
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			log.Printf("exit")
			return
		case syscall.SIGHUP:
		default:
			return
		}
	}
}
