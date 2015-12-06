package network

import (
	"net"
	"squash/log"
	"sync"
	"time"
)

//tcp服务器
type TCPServer struct {
	Addr            string               //地址
	MaxConnNum      int                  //最大连接数
	PendingWriteNum int                  //发送缓冲区长度
	NewAgent        func(*TCPConn) Agent //创建代理函数
	ln              net.Listener         //监听连接器
	conns           ConnSet              //连接集合
	mutexConns      sync.Mutex           //互斥锁
	wgLn            sync.WaitGroup       //监听器等待组
	wgConns         sync.WaitGroup       //连接等待组

	//消息解析器
	LenMsgLen    int        //消息长度占用字节数
	MinMsgLen    uint32     //最小消息长度
	MaxMsgLen    uint32     //最大消息长度
	LittleEndian bool       //是否小端
	msgParser    *MsgParser //消息解析器
}

//启动tcp服务器
func (server *TCPServer) Start() {
	//初始化
	server.init()
	//在一个goroutine里运行tcp服务器
	go server.run()
}

//初始化tcp服务器
func (server *TCPServer) init() {
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

	//代理函数为空，输出致命错误日志，结束tcp服务器进程
	if server.NewAgent == nil {
		log.Fatal("NewAgent must not be nil")
	}

	//保存监听连接器
	server.ln = ln
	//创建连接集合
	server.conns = make(ConnSet)

	//创建消息解析器
	msgParser := NewMsgParser()
	//设置消息长度
	msgParser.SetMsgLen(server.LenMsgLen, server.MinMsgLen, server.MaxMsgLen)
	//设置字节序
	msgParser.SetByteOrder(server.LittleEndian)
	//保存消息解析器
	server.msgParser = msgParser
}

//运行tcp服务器
func (server *TCPServer) run() {
	//监听器等待组+1
	server.wgLn.Add(1)
	//延迟 监听器等待组-1
	defer server.wgLn.Done()

	//连接延时
	var tempDelay time.Duration

	for {
		//接受一个连接
		conn, err := server.ln.Accept()

		//接受失败（比如调用了server.Close再Accept就会失败）
		if err != nil {
			//错误为临时错误
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				//逐渐增大延时
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}

				//延时最大只能为1s
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}

				//输出日志
				log.Release("accept error: %v; retrying in %v", err, tempDelay)
				//执行延时
				time.Sleep(tempDelay)

				continue
			}

			//其他错误，直接退出
			return
		}

		//重置延时，以接受下一个连接
		tempDelay = 0

		//加锁
		//因为会从不同的goroutine中访问server.conns
		//比如从外部goroutine中调用server.Close
		//或者在新的goroutine中运行代理执行清理工作
		//或者当前for循环所在goroutine中增加连接记录
		server.mutexConns.Lock()

		//当前连接数超过上限，解锁，关闭新来的连接，输出日志，继续循环
		if len(server.conns) >= server.MaxConnNum {
			server.mutexConns.Unlock()
			conn.Close()
			log.Debug("too many connections")
			continue
		}

		//将新来的连接添加到连接集合
		server.conns[conn] = struct{}{} //struct{}为类型，第二个{}为初始化，只不过是空值而已
		//解锁
		server.mutexConns.Unlock()
		//连接等待组+1
		server.wgConns.Add(1)
		//创建一个tcp连接
		tcpConn := newTCPConn(conn, server.PendingWriteNum, server.msgParser)
		//创建代理
		agent := server.NewAgent(tcpConn)

		//在一个新的goroutine中运行代理，一个客户端一个agent
		go func() {
			//启动代理
			agent.Run()

			/*清理工作开始*/
			//关闭连接
			tcpConn.Close()
			//加锁
			server.mutexConns.Lock()
			//从连接集合中删除连接
			delete(server.conns, conn)
			//解锁
			server.mutexConns.Unlock()
			//关闭代理
			agent.OnClose()
			//连接等待组-1
			server.wgConns.Done()
			/*清理工作结束*/
		}()
	}
}

//关闭tcp服务器
func (server *TCPServer) Close() {
	//关闭监听器（会导致再Accept时出错）
	server.ln.Close()
	//等待所有监听器的goroutine退出
	server.wgLn.Wait()
	//加锁
	server.mutexConns.Lock()

	//关闭所有现有连接（会导致所有agent循环读取数据时异常，退出循环）
	for conn, _ := range server.conns {
		conn.Close()
	}

	//重置连接集合
	server.conns = make(ConnSet)
	//解锁
	server.mutexConns.Unlock()
	//等待所有连接的goroutine退出
	server.wgConns.Wait()
}
