package network

import (
	"net"
)

type Conn interface {
	ReadMsg() ([]byte, error)      //读取消息
	WriteMsg(args ...[]byte) error //发送消息
	LocalAddr() net.Addr           //返回本地地址
	RemoteAddr() net.Addr          //返回远程（客户端）地址
	Close()                        //关闭连接
	Destroy()                      //销毁
}
