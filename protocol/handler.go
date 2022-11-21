package protocol

import (
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/wuqtao/futu/api/common"
	"github.com/wuqtao/futu/api/initconnect"
	"github.com/wuqtao/futu/api/keepalive"
	"github.com/wuqtao/futu/api/qotgetsubinfo"
	"github.com/wuqtao/futu/api/trdunlocktrade"
	"log"
	"time"
)

type Handler struct {
	wakeup    chan bool
	quit      chan bool
	Resp      chan *Message
	msgWriter chan *Message
	msgReader chan *Message
}

// 统一发送消息入口
func (handler *Handler) SendMsg(msg *Message) {
	handler.msgWriter <- msg
}

func (handler *Handler) Init(writer chan *Message, reader chan *Message, pwd string) error {
	if writer == nil || reader == nil {
		return errors.New("not init netManager")
	}

	handler.msgWriter = writer
	handler.msgReader = reader

	handler.wakeup = make(chan bool, 1)
	handler.quit = make(chan bool, 1)
	handler.Resp = make(chan *Message, 100)

	// init connect
	handler.connectRequest("123", 300, true, 0, common.PacketEncAlgo_PacketEncAlgo_None)
	// response
	go handler.response(pwd)
	return nil
}

func (handler *Handler) Close() {
	handler.quit <- true
	close(handler.Resp)
	time.Sleep(10 * time.Second)
}

func (handler *Handler) connectRequest(clientId string, clientVersion int32, notify bool, format int32, encodeAlgo common.PacketEncAlgo) {
	msg := NewMsg(P_InitConnect, &initconnect.Request{
		C2S: &initconnect.C2S{
			ClientID:      proto.String(clientId),
			ClientVer:     proto.Int32(clientVersion),
			RecvNotify:    proto.Bool(notify),
			PushProtoFmt:  proto.Int32(format),
			PacketEncAlgo: proto.Int32(int32(encodeAlgo)),
		},
	})
	handler.SendMsg(msg)
}

func (handler *Handler) keepAliveFunction(intervalSeconds int32) {
	go func() {
		for {
			select {
			case <-time.After(time.Duration(intervalSeconds-2) * time.Second):
				go func() {
					handler.wakeup <- true
				}()
			case <-handler.quit:
				log.Printf("keepAlive quit")
			case <-handler.wakeup:
				msg := NewMsg(P_KeepAlive, &keepalive.Request{
					C2S: &keepalive.C2S{
						Time: proto.Int64(time.Now().Unix()),
					},
				})
				handler.SendMsg(msg)
			}
		}
	}()
}

func (handler *Handler) response(pwd string) {
	for {
		select {
		case msg := <-handler.msgReader:
			if msg == nil {
				log.Printf("response is nil")
				return
			}

			switch msg.ProtoID {
			case P_InitConnect:
				{
					body, ok := msg.Body.(*initconnect.Response)
					if !ok || *body.ErrCode != 0 {
						log.Printf("init connect resp failed, err=%s", *body.RetMsg)
						return
					}
					// request basic info
					handler.keepAliveFunction(*body.S2C.KeepAliveInterval)
					handler.unlockAccount(pwd)
					//handler.getSubInfo(writer)
				}
			case P_KeepAlive:
				{
					body, ok := msg.Body.(*keepalive.Response)
					log.Printf("read content, retErr=%d, retType=%d, retMsg=%s", *body.ErrCode, *body.RetType, *body.RetMsg)
					if !ok || *body.ErrCode != 0 {
						log.Printf("keepAlive resp failed, err=%s", *body.RetMsg)
						return
					}
				}
			default:
				handler.Resp <- msg
			}
		}
	}
}

func (handler *Handler) unlockAccount(pwd string) {
	msg := NewMsg(P_Trd_UnlockTrade, &trdunlocktrade.Request{
		C2S: &trdunlocktrade.C2S{
			Unlock: proto.Bool(true),
			PwdMD5: proto.String(pwd),
		},
	})
	handler.SendMsg(msg)
}

func (handler *Handler) getSubInfo() {
	msg := NewMsg(P_Qot_GetSubInfo, &qotgetsubinfo.Request{
		C2S: &qotgetsubinfo.C2S{
			IsReqAllConn: proto.Bool(false),
		},
	})
	handler.SendMsg(msg)
}
