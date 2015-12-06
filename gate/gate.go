package gate

import (
	"reflect"
	"squash/chanrpc"
	"squash/log"
	"squash/network"
	"time"
)

//网关服务器
type Gate struct {
	MaxConnNum      int               //最大连接数
	PendingWriteNum int               //发送缓冲区长度
	MaxMsgLen       uint32            //最大消息长度
	Processor       network.Processor //消息解析器
	AgentChanRPC    *chanrpc.Server   //rpc服务器

	//websocket
	WSAddr      string        //ws地址
	HTTPTimeout time.Duration //超时时限

	//tcp
	TCPAddr      string //tcp地址
	LenMsgLen    int    //消息长度占用字节数
	LittleEndian bool   //大小端标志
}

//代理
type agent struct {
	conn     network.Conn //连接
	gate     *Gate        //网关
	userData interface{}  //用户数据
}

//实现module.Module接口的Run方法
func (gate *Gate) Run(closeSig chan bool) {
	//创建ws服务器
	var wsServer *network.WSServer

	//设置ws服务器相关参数
	if gate.WSAddr != "" {
		wsServer = new(network.WSServer)
		wsServer.Addr = gate.WSAddr                                    //地址
		wsServer.MaxConnNum = gate.MaxConnNum                          //最大连接数
		wsServer.PendingWriteNum = gate.PendingWriteNum                //发送缓冲区长度
		wsServer.MaxMsgLen = gate.MaxMsgLen                            //最大消息长度
		wsServer.HTTPTimeout = gate.HTTPTimeout                        //http连接超时时限
		wsServer.NewAgent = func(conn *network.WSConn) network.Agent { //创建代理函数
			a := &agent{conn: conn, gate: gate}

			//代理rpc服务器，用于接受NewAgent和CloseAgentRPC调用
			if gate.AgentChanRPC != nil {
				gate.AgentChanRPC.Go("NewAgent", a)
			}

			return a
		}
	}

	//创建tcp服务器
	var tcpServer *network.TCPServer

	//设置tcp服务器相关参数
	if gate.TCPAddr != "" {
		tcpServer = new(network.TCPServer)
		tcpServer.Addr = gate.TCPAddr                                    //地址
		tcpServer.MaxConnNum = gate.MaxConnNum                           //最大连接数
		tcpServer.PendingWriteNum = gate.PendingWriteNum                 //发送缓冲区长度
		tcpServer.LenMsgLen = gate.LenMsgLen                             //消息长度占用字节数
		tcpServer.MaxMsgLen = gate.MaxMsgLen                             //最大消息长度
		tcpServer.LittleEndian = gate.LittleEndian                       //大小端
		tcpServer.NewAgent = func(conn *network.TCPConn) network.Agent { //创建代理函数
			a := &agent{conn: conn, gate: gate}

			//代理rpc服务器，用于接受NewAgent和CloseAgentRPC调用
			if gate.AgentChanRPC != nil {
				gate.AgentChanRPC.Go("NewAgent", a)
			}

			return a
		}
	}

	//启动ws服务器
	if wsServer != nil {
		wsServer.Start()
	}

	//启动tcp服务器
	if tcpServer != nil {
		tcpServer.Start()
	}

	//等待关闭信号
	<-closeSig

	//关闭ws服务器
	if wsServer != nil {
		wsServer.Close()
	}

	//关闭tcp服务器
	if tcpServer != nil {
		tcpServer.Close()
	}
}

//实现module.Module接口的OnDestroy方法
func (gate *Gate) OnDestroy() {}

//实现network.Agent接口的Run方法
func (a *agent) Run() {
	for {
		//读取一条完整的消息
		data, err := a.conn.ReadMsg()

		//读取失败
		if err != nil {
			log.Debug("read message: %v", err)
			break
		}

		//消息处理器不为空，解码消息
		if a.gate.Processor != nil {
			//解码
			msg, err := a.gate.Processor.Unmarshal(data)

			//解码失败
			if err != nil {
				log.Debug("unmarshal message error: %v", err)
				break
			}

			//路由，分发数据
			err = a.gate.Processor.Route(msg, a)

			//路由失败
			if err != nil {
				log.Debug("route message error: %v", err)
				break
			}
		}
	}
}

//实现network.Agent接口的OnClose方法
func (a *agent) OnClose() {
	//rpc服务器不为空，打开一个rpc客户端，同步调用CloseAgent方法
	if a.gate.AgentChanRPC != nil {
		err := a.gate.AgentChanRPC.Open(0).Call0("CloseAgent", a)

		if err != nil {
			log.Error("chanrpc error: %v", err)
		}
	}
}

//实现gate.Agent接口的WriteMsg方法
func (a *agent) WriteMsg(msg interface{}) {
	//消息处理器不为空，编码消息
	if a.gate.Processor != nil {
		//编码
		data, err := a.gate.Processor.Marshal(msg)

		//编码失败
		if err != nil {
			log.Error("marshal message %v error: %v", reflect.TypeOf(msg), err)
			return
		}

		//发送消息
		a.conn.WriteMsg(data...)
	}
}

//实现gate.Agent接口的Close方法
func (a *agent) Close() {
	a.conn.Close()
}

//实现gate.Agent接口的UserData方法
func (a *agent) UserData() interface{} {
	return a.userData
}

//实现gate.Agent接口的SetUserData方法
func (a *agent) SetUserData(data interface{}) {
	a.userData = data
}
