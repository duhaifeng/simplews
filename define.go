package simplews

import (
	"errors"
	"fmt"
	log "github.com/duhaifeng/loglet"
	"github.com/google/uuid"
	"golang.org/x/net/websocket"
	"sync"
)

/**
 * 定义响应新握手websocket请求的句柄函数格式
 * @author duhaifeng
 */
type WsHandlerFunc func(socket *WebSocket)

/**
 * 定义消息过滤器函数格式
 * @author duhaifeng
 */
type MsgSendFilterFunc func(msg interface{}) bool

/**
 * 定义接收到客户端新消息的处理函数格式
 * @author duhaifeng
 */
type MsgRecvHandlerFunc func(msg []byte)

/**
 * websocket请求响应Handler路由定义
 * @author duhaifeng
 */
type WebSocketHandler struct {
	Path       string
	HandleFunc WsHandlerFunc
}

/**
 * websocket连接包装类型定义
 * @author duhaifeng
 */
type WebSocket struct {
	logger         *log.Logger
	socketId       string
	socket         *websocket.Conn
	recvMsgChan    chan []byte //缓存接收消息的管道
	msgRecvHandler MsgRecvHandlerFunc
	sendMsgChan    chan interface{} //缓存发送消息的管道
	msgSendFilter  MsgSendFilterFunc
	wg             *sync.WaitGroup
	wgLock         *sync.Mutex
	wsPool         *WebSocketPool
}

/**
 * websocket包装连接内部资源初始化
 * @author duhaifeng
 */
func (this *WebSocket) Init(conn *websocket.Conn) {
	this.socketId = uuid.New().String()
	this.logger = log.NewLogger()
	this.socket = conn
	this.sendMsgChan = make(chan interface{}, 100000)
	this.wg = new(sync.WaitGroup)
	this.wg.Add(1)
	this.wgLock = new(sync.Mutex)
	//启动后台监听消息的worker协程
	go this.receiveMsgDaemon()
	//启动后台向客户端写消息的worker协程
	go this.writeMsgDaemon()
}

/**
 * 与自身所在的连接池进行关联，用于当前websocket失效时能及时通知连接池释放资源
 * @author duhaifeng
 */
func (this *WebSocket) AssociateWsPool(wsPool *WebSocketPool) {
	this.wsPool = wsPool
}

/**
 * 设置当前websocket管道的处理接收消息的句柄
 * @author duhaifeng
 */
func (this *WebSocket) SetMsgRecvHandler(msgRecvHandler MsgRecvHandlerFunc) {
	this.msgRecvHandler = msgRecvHandler
}

/**
 * 设置当前websocket管道的发送消息的过滤器
 * @author duhaifeng
 */
func (this *WebSocket) SetMsgSendFilter(sendMsgFilter MsgSendFilterFunc) {
	this.msgSendFilter = sendMsgFilter
}

/**
 * 获取当前websocket的标识ID
 * @author duhaifeng
 */
func (this *WebSocket) GetSocketId() string {
	return this.socketId
}

/**
 * 获取本地IP地址
 * @author duhaifeng
 */
func (this *WebSocket) GetLocalIp() string {
	return this.socket.LocalAddr().String()
}

/**
 * 获取客户端IP地址
 * @author duhaifeng
 */
func (this *WebSocket) GetClientIp() string {
	return this.socket.Request().RemoteAddr
}

/**
 * 获取客户端请求的URL参数
 * @author duhaifeng
 */
func (this *WebSocket) GetUrlParam(key string) string {
	return this.socket.Request().URL.Query().Get(key)
}

/**
 * 从websocket监听消息，并触发对应回调句柄的后台协程
 * @author duhaifeng
 */
func (this *WebSocket) receiveMsgDaemon() {
	defer func() {
		//停止阻塞外部的调用方
		this.waitGroupDone()
		err := recover()
		if err != nil {
			this.logger.Error("web socket receive msg unexpected error: %v", err)
		}
	}()
	for {
		var msg []byte
		err := websocket.Message.Receive(this.socket, &msg)
		if err != nil {
			this.logger.Error("web socket read message err: %s <%s> %s", this.GetClientIp(), this.GetSocketId(), err.Error())
			this.Close()
			return
		}
		if this.msgRecvHandler != nil {
			this.msgRecvHandler(msg)
		}
	}
}

/**
 * 向websocket连接异步写入消息（先写入管道）
 * @author duhaifeng
 */
func (this *WebSocket) WriteMsgAsync(msg interface{}) error {
	if this.sendMsgChan == nil {
		return fmt.Errorf("web socket was not initialized or was closed: %s <%s>", this.GetClientIp(), this.GetSocketId())
	}
	this.sendMsgChan <- msg
	return nil
}

/**
 * 向客户端写入消息的后台协程
 * @author duhaifeng
 */
func (this *WebSocket) writeMsgDaemon() {
	defer func() {
		//停止阻塞外部的调用方
		this.waitGroupDone()
		err := recover()
		if err != nil {
			this.logger.Error("web socket write msg unexpected error: %v", err)
		}
	}()
	for {
		msg := <-this.sendMsgChan
		//如果传入的消息不符合客户端接收的过滤条件，则丢弃消息
		if this.msgSendFilter != nil && !this.msgSendFilter(msg) {
			continue
		}
		err := this.writeMsgToClient(msg)
		if err != nil {
			this.logger.Error("web socket write message err: %v <%s> %s", msg, this.GetSocketId(), err.Error())
			this.Close()
			return
		}
	}
}

/**
 * 向底层websocket连接写入数据
 * @author duhaifeng
 */
func (this *WebSocket) writeMsgToClient(msg interface{}) error {
	if this.socket == nil {
		return errors.New("web socket was not initialized")
	}
	//以json形式向客户端发送数据
	return websocket.JSON.Send(this.socket, msg)
}

/**
 * 向底层websocket连接写入Json数据
 * @author jinbinwu
 */
func (this *WebSocket) SendJsonMsgDirectly(msg interface{}) error {
	if this.socket == nil {
		return errors.New("web socket was not initialized")
	}
	return websocket.JSON.Send(this.socket, msg)
}

/**
 * 保持在当前websocket封装对象上阻塞
 * @author duhaifeng
 */
func (this *WebSocket) waitGroupDone() {
	this.wgLock.Lock()
	defer this.wgLock.Unlock()
	if this.wg == nil {
		return
	}
	this.wg.Done()
	this.wg = nil
}

/**
 * 保持在当前websocket封装对象上阻塞
 * @author duhaifeng
 */
func (this *WebSocket) Wait() {
	this.wg.Wait()
}

/**
 * 释放封装websocket中的管道资源
 * @author duhaifeng
 */
func (this *WebSocket) releaseMsgChan() {
	if this.sendMsgChan == nil {
		return
	}
	close(this.sendMsgChan)
	this.sendMsgChan = nil
}

/**
 * 关闭当前socket连接，并通知连接池释放资源
 * @author duhaifeng
 */
func (this *WebSocket) Close() {
	if this.wsPool != nil {
		//将当前websocket连接从连接池中解除注册
		this.wsPool.UnregisterWebSocket(this)
		//连接池属于长久对象，避免与连接池关联导致自身无法释放
		this.wsPool = nil
	}
	//释放管道资源
	this.releaseMsgChan()
	//仍然尝试关闭底层socket连接
	err := this.socket.Close()
	if err != nil {
		this.logger.Error("web socket close err: %s <%s> %s", this.GetClientIp(), this.GetSocketId(), err.Error())
	}
}

/**
 * websocket连接池对象定义
 * @author duhaifeng
 */
type WebSocketPool struct {
	lock       *sync.RWMutex
	socketPool map[string]*WebSocket
}

func (this *WebSocketPool) Init() {
	this.lock = new(sync.RWMutex)
	this.socketPool = make(map[string]*WebSocket)
}

/**
 * 向连接池中注册一个websocket连接
 * @author duhaifeng
 */
func (this *WebSocketPool) RegisterWebSocket(socket *WebSocket) {
	this.lock.Lock()
	defer func() {
		this.lock.Unlock()
	}()
	if this.socketPool == nil {
		this.Init()
	}
	socket.AssociateWsPool(this)
	this.socketPool[socket.GetSocketId()] = socket
}

/**
 * 向连接池中解除注册一个websocket连接
 * @author duhaifeng
 */
func (this *WebSocketPool) UnregisterWebSocket(socket *WebSocket) {
	this.lock.Lock()
	defer func() {
		this.lock.Unlock()
	}()
	if this.socketPool == nil {
		return
	}
	delete(this.socketPool, socket.GetSocketId())
}

/**
 * 向连接池中所有websocket连接广播消息
 * @author duhaifeng
 */
func (this *WebSocketPool) BroadcastMsg(msg interface{}) {
	this.lock.RLock()
	defer func() {
		this.lock.RUnlock()
	}()
	if this.socketPool == nil {
		return
	}
	for _, socket := range this.socketPool {
		//fmt.Printf("send msg to socketId %s",socket.socketId) //测试是否正常关闭
		err := socket.WriteMsgAsync(msg)
		if err != nil {
			socket.logger.Error("web socket async write err: %s <%s> %s", socket.GetClientIp(), socket.GetSocketId(), err.Error())
			this.UnregisterWebSocket(socket)
		}
	}
}

/**
 * 清空连接池中所有的websocket连接
 * @author duhaifeng
 */
func (this *WebSocketPool) CloseAll() {
	if this.socketPool == nil {
		return
	}
	for _, socket := range this.socketPool {
		socket.Close()
		this.UnregisterWebSocket(socket)
	}
}
