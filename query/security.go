package query

import (
	"errors"
	"nitrohsu.com/futu/api/qotcommon"
	"nitrohsu.com/futu/api/qotgetsecuritysnapshot"
	"nitrohsu.com/futu/protocol"
	"nitrohsu.com/futu/quote"
	"strings"
	"sync"
)

var marketSH = int32(qotcommon.QotMarket_QotMarket_CNSH_Security)
var marketSZ = int32(qotcommon.QotMarket_QotMarket_CNSZ_Security)

func GetSecuritySnapshot(symbols []string) ([]*qotgetsecuritysnapshot.Snapshot, error) {
	var resList []*qotgetsecuritysnapshot.Snapshot
	var err error
	lock := sync.Mutex{}
	if len(symbols) <= 400 {
		res, err := getSecuritySnapshot(symbols)
		if err != nil {
			return nil, err
		}
		resList = append(resList, res.GetS2C().SnapshotList...)
	} else {
		wg := sync.WaitGroup{}
		for i := 0; i < len(symbols); i = i + 400 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, cerr := getSecuritySnapshot(symbols)
				lock.Lock()
				if cerr != nil {
					err = cerr
				} else {
					resList = append(resList, res.GetS2C().SnapshotList...)
				}
				lock.Unlock()
			}()
		}
		wg.Wait()
	}
	return resList, err
}

func getSecuritySnapshot(symbols []string) (*qotgetsecuritysnapshot.Response, error) {
	var err error
	quoteCtx, err := quote.GetDefaultQuote()
	if err != nil {
		return nil, err
	}
	list := buildSecurityQuerys(symbols)
	wg := sync.WaitGroup{}
	wg.Add(1)
	var res *qotgetsecuritysnapshot.Response

	quoteCtx.SendMsgWithCallBack(protocol.P_Qot_GetSecuritySnapshot, list, func(resp interface{}) {
		defer wg.Done()
		if cRes, ok := resp.(*qotgetsecuritysnapshot.Response); ok {
			res = cRes
		} else {
			err = errors.New("resp is not a instance of qotgetsecuritysnapshot.Response")
		}
	})
	wg.Wait()
	return res, err
}

func buildSecurityQuerys(symbols []string) []*qotcommon.Security {
	list := []*qotcommon.Security{}
	for _, symbol := range symbols {
		market := symbol[0:2]
		code := symbol[2:]
		switch strings.ToUpper(market) {
		case "SH":
			list = append(list, &qotcommon.Security{
				Market: &marketSH,
				Code:   &code,
			})
		case "SZ":
			list = append(list, &qotcommon.Security{
				Market: &marketSZ,
				Code:   &code,
			})
		}
	}
	return list
}
