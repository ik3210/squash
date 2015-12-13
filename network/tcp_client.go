package network

import (
	"net"
	"squash/log"
	"sync"
	"time"
)

//tcp客户端类型定义
type TCPClient struct {
	sync.Mutex                           //互斥锁
	Addr            string               //地址
	ConnNum         int                  //连接数
	ConnectInterval time.Duration        //连接间隔
	PendingWriteNum int                  //发送缓冲区长度
	NewAgent        func(*TCPConn) Agent //创建代理函数
	conns           ConnSet              //连接集合
	wg              sync.WaitGroup       //等待组
	closeFlag       bool                 //关闭标志
	LenMsgLen       int                  //存储消息长度信息所占用的字节数
	MinMsgLen       uint32               //最小消息长度
	MaxMsgLen       uint32               //最大消息长度
	LittleEndian    bool                 //是否小端
	msgParser       *MsgParser           //消息解析器
}

//启动tcp客户端
func (client *TCPClient) Start() {
	//初始化
	client.init()

	for i := 0; i < client.ConnNum; i++ {
		//等待组+1
		client.wg.Add(1)
		//在goroutine里创建tcp客户端连接
		go client.connect()
	}
}

//初始化tcp客户端
func (client *TCPClient) init() {
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

	//代理函数为空，输出致命错误日志，结束tcp客户端进程
	if client.NewAgent == nil {
		log.Fatal("NewAgent must not be nil")
	}

	//连接集合不为空，输出致命错误日志，结束tcp客户端进程
	if client.conns != nil {
		log.Fatal("client is running")
	}

	//创建连接集合
	client.conns = make(ConnSet)
	//取消关闭标记
	client.closeFlag = false
	//创建消息解析器
	msgParser := NewMsgParser()
	//设置消息长度
	msgParser.SetMsgLen(client.LenMsgLen, client.MinMsgLen, client.MaxMsgLen)
	//设置字节序
	msgParser.SetByteOrder(client.LittleEndian)
	//保存消息解析器
	client.msgParser = msgParser
}

//拨号连接
func (client *TCPClient) dial() net.Conn {
	for {
		//创建一个tcp连接
		conn, err := net.Dial("tcp", client.Addr)

		//连接成功或设置了关闭标记，返回对象并结束循环
		//因为即使设置了关闭标记，但是连接还是建立的，这时候要让后面的流程（connect()函数里）来把这个连接关闭掉，这样对方才知道连接断开了
		if err == nil || client.closeFlag {
			return conn
		}

		//连接失败，输出日志，在连接间隔后重新尝试连接
		log.Release("connect to %v error: %v", client.Addr, err)
		time.Sleep(client.ConnectInterval)

		continue
	}
}

//创建一个tcp客户端连接
func (client *TCPClient) connect() {
	//延迟 等待组-1
	defer client.wg.Done()

	//拨号连接
	conn := client.dial()

	//连接失败
	if conn == nil {
		return
	}

	//加锁
	//因为会从不同的goroutine中访问client.conns
	//比如从外部goroutine中调用client.Close
	//或者在新的goroutine中运行代理执行清理工作
	client.Lock()

	//设置了关闭标志，解锁，取消连接
	if client.closeFlag {
		client.Unlock()
		conn.Close()
		return
	}

	//将新来的连接添加到连接集合
	client.conns[conn] = struct{}{} //struct{}为类型，第二个{}为初始化，只不过是空值而已
	//解锁
	client.Unlock()

	//创建一个tcp连接
	tcpConn := newTCPConn(conn, client.PendingWriteNum, client.msgParser)
	//创建代理
	agent := client.NewAgent(tcpConn)
	//运行代理
	agent.Run()

	/*清理工作开始*/
	//关闭连接
	tcpConn.Close()
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

//关闭tcp客户端
func (client *TCPClient) Close() {
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
