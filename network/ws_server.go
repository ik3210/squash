package network

import (
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"squash/log"
	"sync"
	"time"
)

//ws服务器
type WSServer struct {
	Addr            string              //地址
	MaxConnNum      int                 //最大连接数
	PendingWriteNum int                 //发送缓冲区长度
	MaxMsgLen       uint32              //最大消息长度
	HTTPTimeout     time.Duration       //http连接超时时限
	NewAgent        func(*WSConn) Agent //创建代理函数
	ln              net.Listener        //监听连接器
	handler         *WSHandler          //调用的处理器
}

type WSHandler struct {
	maxConnNum      int                 //最大连接数
	pendingWriteNum int                 //发送缓冲区长度
	maxMsgLen       uint32              //最大消息长度
	newAgent        func(*WSConn) Agent //创建代理函数
	upgrader        websocket.Upgrader  //升级器，将http连接升级为ws连接
	conns           WebsocketConnSet    //连接集合
	mutexConns      sync.Mutex          //互斥锁
	wg              sync.WaitGroup      //等待组
}

//运行http服务器
func (handler *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//http请求使用GET方法，回复错误信息和状态码
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	//升级http连接到ws协议
	conn, err := handler.upgrader.Upgrade(w, r, nil)

	//升级失败
	if err != nil {
		log.Debug("upgrade error: %v", err)
		return
	}

	//设置消息最大读取长度
	conn.SetReadLimit(int64(handler.maxMsgLen))
	//等待组+1
	handler.wg.Add(1)
	//延迟 等待组-1
	defer handler.wg.Done()
	//加锁
	handler.mutexConns.Lock()

	//连接集合为空，解锁，关闭新来的连接
	if handler.conns == nil {
		handler.mutexConns.Unlock()
		conn.Close()
		return
	}

	//当前连接数超过上限，解锁，关闭新来的连接，输出日志
	if len(handler.conns) >= handler.maxConnNum {
		handler.mutexConns.Unlock()
		conn.Close()
		log.Debug("too many connections")
		return
	}

	//将新来的连接添加到连接集合
	handler.conns[conn] = struct{}{}
	//解锁
	handler.mutexConns.Unlock()
	//创建一个ws连接
	wsConn := newWSConn(conn, handler.pendingWriteNum, handler.maxMsgLen)
	//创建代理
	agent := handler.newAgent(wsConn)
	//在一个新的goroutine中运行代理，一个客户端一个agent
	agent.Run()

	/*清理工作开始*/
	//关闭连接
	wsConn.Close()
	//加锁
	handler.mutexConns.Lock()
	//从连接集合中删除连接
	delete(handler.conns, conn)
	//解锁
	handler.mutexConns.Unlock()
	//关闭代理
	agent.OnClose()
	/*清理工作结束*/
}

//启动ws服务器
func (server *WSServer) Start() {
	//监听tcp连接
	ln, err := net.Listen("tcp", server.Addr)

	//监听失败
	if err != nil {
		log.Fatal("%v", err)
	}

	//最大连接数小于0，重置到100
	if server.MaxConnNum <= 0 {
		server.MaxConnNum = 100
		log.Release("invalid MaxConnNum, reset to %v", server.MaxConnNum)
	}

	//发送缓冲区长度小于0，重置到100
	if server.PendingWriteNum <= 0 {
		server.PendingWriteNum = 100
		log.Release("invalid PendingWriteNum, reset to %v", server.PendingWriteNum)
	}

	//最大消息长度小于0，重置到4096
	if server.MaxMsgLen <= 0 {
		server.MaxMsgLen = 4096
		log.Release("invalid MaxMsgLen, reset to %v", server.MaxMsgLen)
	}

	//http连接超时时限小于0，重置到10
	if server.HTTPTimeout <= 0 {
		server.HTTPTimeout = 10 * time.Second
		log.Release("invalid HTTPTimeout, reset to %v", server.HTTPTimeout)
	}

	//代理函数为空，输出致命错误日志，结束tcp服务器进程
	if server.NewAgent == nil {
		log.Fatal("NewAgent must not be nil")
	}

	//保存监听连接器
	server.ln = ln

	//设置调用的处理器
	server.handler = &WSHandler{
		maxConnNum:      server.MaxConnNum,      //最大连接数
		pendingWriteNum: server.PendingWriteNum, //发送缓冲区长度
		maxMsgLen:       server.MaxMsgLen,       //最大消息长度
		newAgent:        server.NewAgent,        //创建代理函数
		conns:           make(WebsocketConnSet), //连接集合
		upgrader: websocket.Upgrader{ //升级器，将http连接升级为ws连接
			HandshakeTimeout: server.HTTPTimeout,
			CheckOrigin:      func(_ *http.Request) bool { return true },
		},
	}

	//设置http服务器
	httpServer := &http.Server{
		Addr:           server.Addr,        //监听的TCP地址
		Handler:        server.handler,     //调用的处理器
		ReadTimeout:    server.HTTPTimeout, //读取操作超时时限
		WriteTimeout:   server.HTTPTimeout, //写入操作超时时限
		MaxHeaderBytes: 1024,               //请求头最大长度
	}

	//运行http服务器
	go httpServer.Serve(ln)
}

//关闭ws服务器
func (server *WSServer) Close() {
	//关闭监听器（会导致再Accept时出错）
	server.ln.Close()
	//加锁
	server.handler.mutexConns.Lock()

	//关闭所有现有连接（会导致所有agent循环读取数据时异常，退出循环）
	for conn := range server.handler.conns {
		conn.Close()
	}

	//重置连接集合
	server.handler.conns = nil
	//解锁
	server.handler.mutexConns.Unlock()
	//等待所有连接的goroutine退出
	server.handler.wg.Wait()
}
