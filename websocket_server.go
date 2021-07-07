package simplews

import (
	log "github.com/duhaifeng/loglet"
	"golang.org/x/net/websocket"
	"net/http"
	"reflect"
	"runtime"
)

/**
 * websocket服务器封装定义
 * @author duhaifeng
 */
type WebSocketServer struct {
	logger     *log.Logger
	wsHandlers []*WebSocketHandler
}

/**
 * 初试化websocket服务器
 * @author duhaifeng
 */
func (this *WebSocketServer) init() {
	this.logger = log.NewLogger()
}

/**
 * 启动一个websocket监听端口
 * @author duhaifeng
 */
func (this *WebSocketServer) StartListen(addr, port string) {
	this.init()
	this.registerRoute()
	listen := addr + ":" + port
	this.logger.Info("start websocket listen %s", listen)

	err := http.ListenAndServe(listen, nil)
	if err != nil {
		this.logger.Error("can not listen websocket %s. for the reason: %s", listen, err.Error())
	}
}

/**
 * 注册websocket服务的路由（一个api对应一个url，一个url对应一个handler）
 * @author duhaifeng
 */
func (this *WebSocketServer) HandRequest(path string, handler WsHandlerFunc) {
	this.wsHandlers = append(this.wsHandlers, &WebSocketHandler{Path: path, HandleFunc: handler})
}

/**
 * 将用户注册的路由，注册为底层的websocket handler
 * @author duhaifeng
 */
func (this *WebSocketServer) registerRoute() {
	for i := 0; i < len(this.wsHandlers); i++ {
		handler := this.wsHandlers[i]
		handleFunc := func(conn *websocket.Conn) {
			wrappedSocket := new(WebSocket)
			wrappedSocket.Init(conn)
			handler.HandleFunc(wrappedSocket)
		}
		this.logger.Debug("register websocket api: <%d> %s %s", i, handler.Path, runtime.FuncForPC(reflect.ValueOf(handler.HandleFunc).Pointer()).Name())
		http.Handle(handler.Path, websocket.Handler(handleFunc))
	}
}
