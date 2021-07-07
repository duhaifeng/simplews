package simplews

import (
	"fmt"
	log "github.com/duhaifeng/loglet"
	"golang.org/x/net/websocket"
	"sync"
	"testing"
	"time"
)

var logger = log.NewLogger()

func TestWebSocketServer(t *testing.T) {
	wsServer := new(WebSocketServer)
	wsPool := new(WebSocketPool)
	wsPool.Init()

	wsServer.HandRequest("/ws/test1", func(socket *WebSocket) {
		logger.Debug("param p1 = %s", socket.GetUrlParam("p1"))
		socket.SetMsgRecvHandler(func(msg []byte) {
			logger.Debug(">> server received: %s from %s", string(msg), socket.GetClientIp())
		})
		socket.SetMsgSendFilter(func(msg interface{}) bool {
			//msgStr, ok := msg.(string)
			//if !ok {
			//	return false
			//}
			//if strings.Contains(string(msgStr), "1") {
			//	return false
			//}
			return true
		})
		wsPool.RegisterWebSocket(socket)
		socket.Wait()
		logger.Debug("client disconnect: %s <%s>", socket.GetClientIp(), socket.GetSocketId())
	})

	//broadcaster
	var count = 0
	go func() {
		for i := 0; ; i++ {
			time.Sleep(time.Millisecond * 200)
			msg := fmt.Sprintf("{count : %d}", count)
			for j := 0; j < 150; j++ {
				msg += "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\r\n"
			}
			//logger.Debug(msg)
			wsPool.BroadcastMsg(msg)
			count++
		}
		wsPool.CloseAll()
	}()
	go startWsClient()

	//启动websocket监听服务
	wsServer.StartListen("0.0.0.0", "1213")
}

func TestWebSocketClient(t *testing.T) {
	go startWsClient()
	go startWsClient()
	go startWsClient()
	go startWsClient()
	go startWsClient()
	time.Sleep(time.Second * 5)
}

func startWsClient() {
	conn, err := websocket.Dial("ws://127.0.0.1:1213/ws/test1?p1=v1", "http", "test://1111111")
	if err != nil {
		logger.Error("client err: dial failed: %s", err.Error())
		return
	}
	wg := new(sync.WaitGroup)
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			websocket.Message.Send(conn, fmt.Sprintf("++ client.%d", i))
			time.Sleep(time.Millisecond * 200)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			var msg []byte
			err := websocket.Message.Receive(conn, &msg)
			if err != nil {
				logger.Error("client err: %s", err.Error())
				return
			}
			logger.Debug(">> client received: %s", string(msg))
		}
	}()
	wg.Wait()
	logger.Debug("client finish.")
	conn.Close()
}
