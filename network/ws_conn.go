package network

import (
	"errors"
	"github.com/gorilla/websocket"
	"net"
	"squash/log"
	"sync"
)

//连接集合，值为空结构体
type WebsocketConnSet map[*websocket.Conn]struct{}

//ws连接
type WSConn struct {
	sync.Mutex                 //互斥锁
	conn       *websocket.Conn //底层连接
	writeChan  chan []byte     //发送缓冲
	maxMsgLen  uint32          //最大消息长度
	closeFlag  bool            //关闭标志
}

//新建ws连接
func newWSConn(conn *websocket.Conn, pendingWriteNum int, maxMsgLen uint32) *WSConn {
	//创建一个ws连接
	wsConn := new(WSConn)
	wsConn.conn = conn
	wsConn.writeChan = make(chan []byte, pendingWriteNum)
	wsConn.maxMsgLen = maxMsgLen

	//在一个新的goroutine中发送数据
	go func() {
		//如果发送缓冲区被关闭，此循环会自动结束
		//如果发送缓冲区没有数据，会阻塞在这里
		for b := range wsConn.writeChan {
			//收到的值为nil，而不是字节切片，中断循环
			if b == nil {
				break
			}

			//发送数据
			err := conn.WriteMessage(websocket.BinaryMessage, b)

			//发送失败
			if err != nil {
				break
			}
		}

		/*清理工作开始*/
		//关闭底层连接
		conn.Close()
		//加锁
		wsConn.Lock()
		//设置关闭标志
		wsConn.closeFlag = true
		//解锁
		wsConn.Unlock()
		/*清理工作结束*/
	}()

	return wsConn
}

//销毁操作
func (wsConn *WSConn) doDestroy() {
	//丢弃所有的数据
	wsConn.conn.UnderlyingConn().(*net.TCPConn).SetLinger(0)
	//关闭底层连接
	wsConn.conn.Close()
	//关闭发送缓冲区（会导致发送goroutine中断）
	close(wsConn.writeChan)
	//设置关闭标记
	wsConn.closeFlag = true
}

//写操作
func (wsConn *WSConn) doWrite(b []byte) {
	//发送缓冲区长度等于最大容量，输出日志"管道已满"，做销毁操作
	if len(wsConn.writeChan) == cap(wsConn.writeChan) {
		log.Debug("close conn: channel full")
		wsConn.doDestroy()
		return
	}

	//将待发数据发送到发送缓冲区
	wsConn.writeChan <- b
}

//读取消息
func (wsConn *WSConn) ReadMsg() ([]byte, error) {
	_, b, err := wsConn.conn.ReadMessage()
	return b, err
}

//发送消息
func (wsConn *WSConn) WriteMsg(args ...[]byte) error {
	//加锁
	wsConn.Lock()
	//延迟解锁
	defer wsConn.Unlock()

	//已经设置了关闭标志
	if wsConn.closeFlag {
		return nil
	}

	var msgLen uint32

	//获取消息长度
	for i := 0; i < len(args); i++ {
		msgLen += uint32(len(args[i]))
	}

	if msgLen > wsConn.maxMsgLen { //长度大于最大容量
		return errors.New("message too long")
	} else if msgLen < 1 { //长度小于1
		return errors.New("message too short")
	}

	//只有一条消息
	if len(args) == 1 {
		wsConn.doWrite(args[0])
		return nil
	}

	//有多条消息，合并
	msg := make([]byte, msgLen)
	l := 0

	for i := 0; i < len(args); i++ {
		copy(msg[l:], args[i])
		l += len(args[i])
	}

	//写操作
	wsConn.doWrite(msg)

	return nil
}

//返回本地地址
func (wsConn *WSConn) LocalAddr() net.Addr {
	return wsConn.conn.LocalAddr()
}

//返回远程（客户端）地址
func (wsConn *WSConn) RemoteAddr() net.Addr {
	return wsConn.conn.RemoteAddr()
}

//关闭连接
func (wsConn *WSConn) Close() {
	//加锁
	wsConn.Lock()
	//延迟解锁
	defer wsConn.Unlock()

	//已经设置了关闭标志
	if wsConn.closeFlag {
		return
	}

	//发送一个nil到发送缓冲区，导致发送goroutine中断循环，做清理工作
	wsConn.doWrite(nil)
	//设置关闭标志
	wsConn.closeFlag = true
}

//销毁
func (wsConn *WSConn) Destroy() {
	//加锁
	wsConn.Lock()
	//延迟解锁
	defer wsConn.Unlock()

	//已经设置了关闭标志
	if wsConn.closeFlag {
		return
	}

	//做具体的销毁操作
	wsConn.doDestroy()
}
