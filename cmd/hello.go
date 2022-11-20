package main

import (
	"fmt"
	"log"
	"nitrohsu.com/futu/api/qotcommon"
	"nitrohsu.com/futu/api/qotgetsecuritysnapshot"
	"nitrohsu.com/futu/protocol"
	"nitrohsu.com/futu/quote"
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
	market := int32(qotcommon.QotMarket_QotMarket_CNSH_Security)
	code := "600000"
	msgBody := qotgetsecuritysnapshot.Request{C2S: &qotgetsecuritysnapshot.C2S{SecurityList: []*qotcommon.Security{
		{Market: &market, Code: &code},
	}}}

	err = quoteContext.SendMsgWithCallBack(protocol.P_Qot_GetSecuritySnapshot, &msgBody, func(resp interface{}) {
		if res, ok := resp.(*qotgetsecuritysnapshot.Response); ok {
			for _, snap := range res.GetS2C().GetSnapshotList() {
				fmt.Println(snap)
			}
		}
	})
	if err != nil {
		fmt.Println(err.Error())
	}
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
