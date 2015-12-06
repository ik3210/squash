package network

import (
	"net"
	"squash/log"
	"sync"
)

//连接集合，值为空结构体
type ConnSet map[net.Conn]struct{}

//tcp连接
type TCPConn struct {
	sync.Mutex             //互斥锁
	conn       net.Conn    //底层连接
	writeChan  chan []byte //发送缓冲
	closeFlag  bool        //关闭标志
	msgParser  *MsgParser  //消息解析器
}

//新建tcp连接
func newTCPConn(conn net.Conn, pendingWriteNum int, msgParser *MsgParser) *TCPConn {
	//创建一个tcp连接
	tcpConn := new(TCPConn)
	tcpConn.conn = conn
	tcpConn.writeChan = make(chan []byte, pendingWriteNum)
	tcpConn.msgParser = msgParser

	//在一个新的goroutine中发送数据
	go func() {
		//如果发送缓冲区被关闭，此循环会自动结束
		//如果发送缓冲区没有数据，会阻塞在这里
		for b := range tcpConn.writeChan {
			//收到的值为nil，而不是字节切片，中断循环
			if b == nil {
				break
			}

			//发送数据
			_, err := conn.Write(b)

			//发送失败
			if err != nil {
				break
			}
		}

		/*清理工作开始*/
		//关闭底层连接
		conn.Close()
		//加锁
		tcpConn.Lock()
		//设置关闭标志
		tcpConn.closeFlag = true
		//解锁
		tcpConn.Unlock()
		/*清理工作结束*/
	}()

	return tcpConn
}

//销毁操作
func (tcpConn *TCPConn) doDestroy() {
	//丢弃所有的数据
	tcpConn.conn.(*net.TCPConn).SetLinger(0)
	//关闭底层连接
	tcpConn.conn.Close()
	//关闭发送缓冲区（会导致发送goroutine中断）
	close(tcpConn.writeChan)
	//设置关闭标记
	tcpConn.closeFlag = true
}

//写操作
func (tcpConn *TCPConn) doWrite(b []byte) {
	//发送缓冲区长度等于最大容量，输出日志"管道已满"，做销毁操作
	if len(tcpConn.writeChan) == cap(tcpConn.writeChan) {
		log.Debug("close conn: channel full")
		tcpConn.doDestroy()
		return
	}

	//将待发数据发送到发送缓冲区
	tcpConn.writeChan <- b
}

//从缓冲区读取数据
func (tcpConn *TCPConn) Read(b []byte) (int, error) {
	return tcpConn.conn.Read(b)
}

//写数据到缓冲区
func (tcpConn *TCPConn) Write(b []byte) {
	//加锁
	tcpConn.Lock()
	//延迟解锁
	defer tcpConn.Unlock()

	//连接已关闭或者传入的b为空
	if tcpConn.closeFlag || b == nil {
		return
	}

	//写操作
	tcpConn.doWrite(b)
}

//读取消息
func (tcpConn *TCPConn) ReadMsg() ([]byte, error) {
	//使用消息解析器读取
	return tcpConn.msgParser.Read(tcpConn)
}

//发送消息
func (tcpConn *TCPConn) WriteMsg(args ...[]byte) error {
	//使用消息解析器发送
	return tcpConn.msgParser.Write(tcpConn, args...)
}

//返回本地地址
func (tcpConn *TCPConn) LocalAddr() net.Addr {
	return tcpConn.conn.LocalAddr()
}

//返回远程（客户端）地址
func (tcpConn *TCPConn) RemoteAddr() net.Addr {
	return tcpConn.conn.RemoteAddr()
}

//关闭连接
func (tcpConn *TCPConn) Close() {
	//加锁
	tcpConn.Lock()
	//延迟解锁
	defer tcpConn.Unlock()

	//已经设置了关闭标志
	if tcpConn.closeFlag {
		return
	}

	//发送一个nil到发送缓冲区，导致发送goroutine中断循环，做清理工作
	tcpConn.doWrite(nil)
	//设置关闭标志
	tcpConn.closeFlag = true
}

//销毁
func (tcpConn *TCPConn) Destroy() {
	//加锁
	tcpConn.Lock()
	//延迟解锁
	defer tcpConn.Unlock()

	//已经设置了关闭标志
	if tcpConn.closeFlag {
		return
	}

	//做具体的销毁操作
	tcpConn.doDestroy()
}
