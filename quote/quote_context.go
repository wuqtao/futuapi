package quote

import (
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/wuqtao/futuapi/api/qotcommon"
	"github.com/wuqtao/futuapi/api/qotgetorderbook"
	"github.com/wuqtao/futuapi/api/qotgetsubinfo"
	"github.com/wuqtao/futuapi/api/qotsub"
	"github.com/wuqtao/futuapi/api/trdcommon"
	"github.com/wuqtao/futuapi/api/trdgethistoryorderlist"
	"github.com/wuqtao/futuapi/api/trdunlocktrade"
	"github.com/wuqtao/futuapi/conf"
	"github.com/wuqtao/futuapi/netmanager"
	"github.com/wuqtao/futuapi/protocol"
	"log"
	"sync"
)

var defaultQuote *Quote
var once sync.Once

type Quote struct {
	Config    conf.Config
	NetMgr    *netmanager.NetManager
	futuState *protocol.Handler
	FuncCall  map[uint32]func(resp interface{})
	funcLock  *sync.Mutex
	SubStock  []*qotcommon.Security
	account   map[trdcommon.TrdMarket]*trdcommon.TrdHeader
	password  string
	unlock    bool
}

func NewQuote(host, port string) (*Quote, error) {
	quote := &Quote{
		Config: conf.Config{
			Host: host,
			Port: port,
		},
	}
	err := quote.Start()
	if err != nil {
		return nil, err
	}
	once.Do(func() {
		defaultQuote = quote
	})
	return quote, nil
}

func GetDefaultQuote() (*Quote, error) {
	if defaultQuote != nil {
		return defaultQuote, nil
	}
	return nil, errors.New("default is not initialized , call new quote first please")
}

// 设置了callback的消息的回复不再传入统一的Resp通道，而是直接调用callback返回消息内容
func (quote *Quote) SendMsgWithCallBack(protoId int, body interface{}, f func(resp interface{})) error {
	msg := protocol.NewMsg(protoId, body)
	if msg.SerialNo == 0 {
		errors.New("the filed serialNo of msg must not be 0")
	}
	quote.funcLock.Lock()
	quote.FuncCall[msg.SerialNo] = f
	quote.funcLock.Unlock()
	quote.futuState.SendMsg(msg)
	return nil
}

func (quote *Quote) SendMsg(protoId int, body interface{}) {
	msg := protocol.NewMsg(protoId, body)
	quote.futuState.SendMsg(msg)
}

func (quote *Quote) Start() (err error) {
	//
	quote.NetMgr = &netmanager.NetManager{
		Host: quote.Config.Host,
		Port: quote.Config.Port,
	}
	err = quote.NetMgr.Connect()
	if err != nil {
		return
	}

	quote.funcLock = &sync.Mutex{}
	quote.FuncCall = make(map[uint32]func(resp interface{}))

	quote.account = make(map[trdcommon.TrdMarket]*trdcommon.TrdHeader)
	//quote.account[trdcommon.TrdMarket_TrdMarket_US] = &trdcommon.TrdHeader{
	//	TrdEnv:    proto.Int32(int32(trdcommon.TrdEnv_TrdEnv_Real)),
	//	AccID:     proto.Uint64(0),
	//	TrdMarket: proto.Int32(int32(trdcommon.TrdMarket_TrdMarket_US)),
	//}
	//quote.account[trdcommon.TrdMarket_TrdMarket_HK] = &trdcommon.TrdHeader{
	//	TrdEnv:    proto.Int32(int32(trdcommon.TrdEnv_TrdEnv_Real)),
	//	AccID:     proto.Uint64(0),
	//	TrdMarket: proto.Int32(int32(trdcommon.TrdMarket_TrdMarket_HK)),
	//}
	//quote.password = "MD5 YOUR PASSWORD"
	quote.futuState = &protocol.Handler{}
	//
	if futuErr := quote.futuState.Init(quote.NetMgr.WriteBuffer, quote.NetMgr.ReadBuffer, quote.password); futuErr != nil {
		return futuErr
	}
	//
	go func() {
		for {
			select {
			case msg := <-quote.futuState.Resp:
				quote.funcLock.Lock()
				if fn, ok := quote.FuncCall[msg.SerialNo]; ok {
					go fn(msg.Body)
					continue
				}
				quote.funcLock.Unlock()

				switch msg.ProtoID {
				case protocol.P_Qot_Sub:
					quote.subResponse(msg.Body.(*qotsub.Response))
				case protocol.P_Qot_GetSubInfo:
					quote.subInfoListResponse(msg.Body.(*qotgetsubinfo.Response))
				case protocol.P_Trd_GetOrderList:
					quote.realOrderListResponse(msg.Body.(*qotgetorderbook.Response))
				case protocol.P_Trd_GetHistoryOrderList:
					quote.historyOrderListResponse(msg.Body.(*trdgethistoryorderlist.Response))
				case protocol.P_Trd_UnlockTrade:
					quote.unlockTradeResponse(msg.Body.(*trdunlocktrade.Response))
				}
			}
		}
	}()
	//time.Sleep(time.Duration(10) * time.Second)
	//quote.realOrderListRequest(quote.NetMgr.WriteBuffer)
	//quote.historyOrderListRequest(nil)

	return
}

func (quote *Quote) realOrderListRequest(writer chan *protocol.Message) {

	stock := &qotcommon.Security{
		Market: proto.Int32(int32(qotcommon.QotMarket_QotMarket_US_Security)),
		Code:   proto.String("BA"),
	}
	exist := false
	for _, item := range quote.SubStock {
		if stock.GetCode() == item.GetCode() && stock.GetMarket() == item.GetMarket() {
			exist = true
		}
	}
	if !exist {
		quote.subRequest(stock)
	} else {
		log.Printf("subed stock, %d=%s", stock.GetMarket(), stock.GetCode())
	}
	msg := protocol.NewMsg(protocol.P_Qot_GetOrderBook, &qotgetorderbook.Request{
		C2S: &qotgetorderbook.C2S{
			Num:      proto.Int32(10),
			Security: stock,
		}})
	quote.futuState.SendMsg(msg)
}

func (quote *Quote) historyOrderListRequest(markets []trdcommon.TrdMarket) {

	if markets == nil {
		markets = make([]trdcommon.TrdMarket, 2)
		markets[0] = trdcommon.TrdMarket_TrdMarket_US
		markets[1] = trdcommon.TrdMarket_TrdMarket_HK
	}
	for _, market := range markets {
		msg := protocol.NewMsg(protocol.P_Trd_GetHistoryOrderList, &trdgethistoryorderlist.Request{
			C2S: &trdgethistoryorderlist.C2S{
				Header: quote.account[market],
				FilterConditions: &trdcommon.TrdFilterConditions{
					CodeList: []string{},
					IdList:   []uint64{},
					//YYYY-MM-DD HH:MM:SS
					BeginTime: proto.String("2020-01-01 00:00:00"),
					EndTime:   proto.String("2020-03-15 00:00:00"),
				},
			}})
		quote.futuState.SendMsg(msg)
	}
}
func (quote *Quote) realOrderListResponse(msg *qotgetorderbook.Response) {
	if msg.GetErrCode() == 0 {
		orderList := msg.S2C.OrderBookAskList
		log.Printf("%x", orderList)
	} else {
		log.Printf("orderList err. err=%s", *msg.RetMsg)
	}
}

func (quote *Quote) historyOrderListResponse(msg *trdgethistoryorderlist.Response) {
	if msg.GetErrCode() == 0 {
		header := msg.S2C.GetHeader()
		log.Printf("accId=%d, market=%s, env=%s", header.GetAccID(), trdcommon.TrdMarket_name[header.GetTrdMarket()], trdcommon.TrdEnv_name[header.GetTrdEnv()])
		orderList := msg.S2C.GetOrderList()
		for _, order := range orderList {
			log.Printf("%s,%s,%s,%d,%s,%s,%s,%s,%f,%f",
				order.GetCreateTime(),
				trdcommon.TrdMarket_name[header.GetTrdMarket()],
				order.GetUpdateTime(),
				order.GetOrderID(),
				trdcommon.OrderStatus_name[order.GetOrderStatus()],
				trdcommon.TrdSide_name[order.GetTrdSide()],
				order.GetCode(),
				order.GetName(),
				order.GetPrice(),
				order.GetQty())
		}
	} else {
		log.Printf("orderList err. err=%s", *msg.RetMsg)
	}
}

func (quote *Quote) subRequest(stock *qotcommon.Security) {
	msg := protocol.NewMsg(protocol.P_Qot_Sub, &qotsub.Request{
		C2S: &qotsub.C2S{
			IsFirstPush:  proto.Bool(false),
			IsSubOrUnSub: proto.Bool(true),
			SecurityList: []*qotcommon.Security{stock},
			SubTypeList: []int32{
				int32(qotcommon.SubType_SubType_OrderBook),
			},
		}})
	quote.futuState.SendMsg(msg)
}

func (quote *Quote) subResponse(msg *qotsub.Response) {
	if *msg.ErrCode == 0 {
		log.Printf("sub succes")
	} else {
		log.Printf("sub err. err=%s", *msg.RetMsg)
	}
}

func (quote *Quote) subInfoListResponse(msg *qotgetsubinfo.Response) {
	if *msg.ErrCode == 0 {
		connSubInfoList := msg.S2C.ConnSubInfoList
		for _, connSubInfo := range connSubInfoList {
			for _, subInfo := range connSubInfo.SubInfoList {
				for _, stock := range subInfo.SecurityList {
					quote.SubStock = append(quote.SubStock, stock)
				}
			}
			log.Printf("getSubInfoList used=%d", connSubInfo.GetUsedQuota())
		}
	} else {
		log.Printf("getSubInfoList err. err=%s", *msg.RetMsg)
	}
}

func (quote *Quote) unlockTradeResponse(msg *trdunlocktrade.Response) {
	if *msg.ErrCode == 0 {
		quote.unlock = true
		log.Printf("unlockTrade success")
	} else {
		log.Printf("unlockTrade err. err=%s", *msg.RetMsg)
	}
}

func (quote *Quote) Close() {
	quote.NetMgr.Close()
	quote.futuState.Close()
}
