package network

import (
	"github.com/gorilla/websocket"
	"squash/log"
	"sync"
	"time"
)

//ws客户端
type WSClient struct {
	sync.Mutex                           //互斥锁
	Addr             string              //地址
	ConnNum          int                 //连接数
	ConnectInterval  time.Duration       //连接间隔
	PendingWriteNum  int                 //发送缓冲区长度
	MaxMsgLen        uint32              //最大消息长度
	HandshakeTimeout time.Duration       //握手超时时限
	NewAgent         func(*WSConn) Agent //创建代理函数
	dialer           websocket.Dialer    //拨号器
	conns            WebsocketConnSet    //连接集合
	wg               sync.WaitGroup      //等待组
	closeFlag        bool                //关闭标志
}

//启动ws客户端
func (client *WSClient) Start() {
	//初始化
	client.init()

	for i := 0; i < client.ConnNum; i++ {
		//等待组+1
		client.wg.Add(1)
		//在goroutine里创建ws客户端连接
		go client.connect()
	}
}

//初始化ws客户端
func (client *WSClient) init() {
	//加锁
	client.Lock()
	//延迟解锁
	defer client.Unlock()

	//连接数小于0，重置到1
	if client.ConnNum <= 0 {
		client.ConnNum = 1
		log.Release("invalid ConnNum, reset to %v", client.ConnNum)
	}

	//连接间隔小于0，重置到3
	if client.ConnectInterval <= 0 {
		client.ConnectInterval = 3 * time.Second
		log.Release("invalid ConnectInterval, reset to %v", client.ConnectInterval)
	}

	//发送缓冲区长度小于0，重置到100
	if client.PendingWriteNum <= 0 {
		client.PendingWriteNum = 100
		log.Release("invalid PendingWriteNum, reset to %v", client.PendingWriteNum)
	}

	//消息最大长度小于0，重置到4096
	if client.MaxMsgLen <= 0 {
		client.MaxMsgLen = 4096
		log.Release("invalid MaxMsgLen, reset to %v", client.MaxMsgLen)
	}

	//握手超时时间小于0，重置到10
	if client.HandshakeTimeout <= 0 {
		client.HandshakeTimeout = 10 * time.Second
		log.Release("invalid HandshakeTimeout, reset to %v", client.HandshakeTimeout)
	}

	//代理函数为空，输出致命错误日志，结束ws客户端进程
	if client.NewAgent == nil {
		log.Fatal("NewAgent must not be nil")
	}

	//连接集合不为空，输出致命错误日志，结束ws客户端进程
	if client.conns != nil {
		log.Fatal("client is running")
	}

	//创建连接集合
	client.conns = make(WebsocketConnSet)
	//关闭标记
	client.closeFlag = false
	//设置拨号器
	client.dialer = websocket.Dialer{
		HandshakeTimeout: client.HandshakeTimeout,
	}
}

//创建一个ws客户端连接
func (client *WSClient) connect() {
	//延迟 等待组-1
	defer client.wg.Done()

	//拨号连接
	conn := client.dial()
	//连接失败
	if conn == nil {
		return
	}

	//设置读取消息的最大长度
	conn.SetReadLimit(int64(client.MaxMsgLen))

	//加锁，避免其他goroutine访问client.conns
	client.Lock()
	//设置了关闭标志，解锁，取消连接
	if client.closeFlag {
		client.Unlock()
		conn.Close()
		return
	}
	//将新连接添加到连接集合
	client.conns[conn] = struct{}{} //struct{}为类型，第二个{}为初始化，只不过是空值而已
	//解锁
	client.Unlock()

	//创建一个ws连接
	wsConn := newWSConn(conn, client.PendingWriteNum, client.MaxMsgLen)
	//创建代理
	agent := client.NewAgent(wsConn)
	//运行代理
	agent.Run()

	/*清理工作开始*/
	//关闭连接
	wsConn.Close()
	//加锁
	client.Lock()
	//从连接集合中删除连接
	delete(client.conns, conn)
	//解锁
	client.Unlock()
	//关闭代理
	agent.OnClose()
	/*清理工作结束*/
}

//拨号连接
func (client *WSClient) dial() *websocket.Conn {
	for {
		//创建一个ws连接
		conn, _, err := client.dialer.Dial(client.Addr, nil)
		//连接成功或设置了关闭标记，返回对象并结束循环（即使设置了关闭标记，连接还是建立的，要在后面的connect()里把这个连接关闭掉，这样对方才知道连接断开了）
		if err == nil || client.closeFlag {
			return conn
		}

		//连接失败，输出日志，在连接间隔后重新尝试连接
		log.Release("connect to %v error: %v", client.Addr, err)
		time.Sleep(client.ConnectInterval)

		continue
	}
}

//关闭ws客户端
func (client *WSClient) Close() {
	//加锁
	client.Lock()
	//设置关闭标记
	client.closeFlag = true

	//关闭所有现有连接
	for conn := range client.conns {
		conn.Close()
	}

	//重置连接集合
	client.conns = nil
	//解锁
	client.Unlock()
	//等待所有goroutine退出
	client.wg.Wait()
}
