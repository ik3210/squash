package chanrpc

import (
	"errors"
	"fmt"
	"runtime"
	"squash/conf"
)

//rpc服务器
type Server struct {
	functions map[interface{}]interface{} //id->func映射
	ChanCall  chan *CallInfo              //调用信息管道，用于传递调用信息
}

//调用信息
type CallInfo struct {
	f       interface{}   //函数
	args    []interface{} //参数
	chanRet chan *RetInfo //返回值管道，用于传输返回值
	cb      interface{}   //回调
}

//返回信息
type RetInfo struct {
	ret interface{} //返回值
	err error       //错误
	cb  interface{} //回调，用于异步调用
}

//rpc客户端
type Client struct {
	s               *Server       //rpc服务器引用
	chanSyncRet     chan *RetInfo //同步调用返回信息管道
	ChanAsynRet     chan *RetInfo //异步调用返回信息管道
	pendingAsynCall int           //待处理的异步调用
}

//创建服务器
func NewServer(l int) *Server {
	//创建服务器
	s := new(Server)
	//为id->func映射分配内存
	s.functions = make(map[interface{}]interface{})
	//为调用信息管道分配内存
	s.ChanCall = make(chan *CallInfo, l)

	return s
}

//服务器注册id->func映射，必须在Open和Go之前调用
func (s *Server) Register(id interface{}, f interface{}) {
	//判断func的类型，类型定义非法，抛出错误
	//允许类型：
	//1. 参数是切片，值任意，无返回值
	//2. 参数是切片，值任意，返回一个任意值
	//3. 参数是切片，返回值也是切片，值均为任意值
	switch f.(type) {
	case func([]interface{}):
	case func([]interface{}) interface{}:
	case func([]interface{}) []interface{}:
	default:
		panic(fmt.Sprintf("function id %v: definition of function is invalid", id))
	}

	//映射已存在，抛出错误
	if _, ok := s.functions[id]; ok {
		panic(fmt.Sprintf("function id %v: already registered", id))
	}

	//存储映射
	s.functions[id] = f
}

//返回
func (s *Server) ret(ci *CallInfo, ri *RetInfo) (err error) {
	//返回管道不能为空
	if ci.chanRet == nil {
		return
	}

	//延迟捕获异常
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	//将调用信息中的回调函数保存到返回信息中，只有异步调用才有
	ri.cb = ci.cb
	//将返回信息发送到返回值管道中
	ci.chanRet <- ri

	return
}

//执行rpc调用
func (s *Server) Exec(ci *CallInfo) (err error) {
	//延迟处理异常
	defer func() {
		if r := recover(); r != nil {
			if conf.LenStackBuf > 0 { //配置了调用栈踪迹缓冲长度，将当前goroutine的调用栈踪迹格式化后写入到buf中
				buf := make([]byte, conf.LenStackBuf)
				l := runtime.Stack(buf, false)
				err = fmt.Errorf("%v: %s", r, buf[:l])
			} else {
				err = fmt.Errorf("%v", r)
			}

			//将错误发送到调用信息的返回值管道中
			s.ret(ci, &RetInfo{err: fmt.Errorf("%v", r)})
		}
	}()

	//根据调用函数的类型，执行调用，得到返回值
	switch ci.f.(type) {
	case func([]interface{}): //无返回值
		ci.f.(func([]interface{}))(ci.args)
		return s.ret(ci, &RetInfo{})
	case func([]interface{}) interface{}: //一个返回值
		ret := ci.f.(func([]interface{}) interface{})(ci.args)
		return s.ret(ci, &RetInfo{ret: ret})
	case func([]interface{}) []interface{}: //多个返回值
		ret := ci.f.(func([]interface{}) []interface{})(ci.args)
		return s.ret(ci, &RetInfo{ret: ret})
	}

	//执行调用失败，抛出错误
	panic("bug")
}

//rpc服务器调用自己
func (s *Server) Go(id interface{}, args ...interface{}) {
	//根据id获取所映射的func
	f := s.functions[id]

	//func未注册
	if f == nil {
		return
	}

	//延迟处理异常（因为该方法没有返回error，所以简单recover就好）
	defer func() {
		recover()
	}()

	//将调用信息通过管道传输到rpc服务器
	s.ChanCall <- &CallInfo{f: f, args: args}
}

//关闭rpc服务器
func (s *Server) Close() {
	//关闭调用信息管道
	close(s.ChanCall)

	//遍历所有未处理完的调用信息，将"管道已关闭"错误发送到调用信息的返回值管道中
	for ci := range s.ChanCall {
		s.ret(ci, &RetInfo{err: errors.New("chanrpc server closed")})
	}
}

//打开一个rpc客户端
func (s *Server) Open(l int) *Client {
	//创建一个rpc客户端
	c := new(Client)
	//保存rpc服务器引用
	c.s = s
	//为同步调用返回信息管道分配内存，管道大小一定为1，因为调用以后需要阻塞来读取返回信息
	c.chanSyncRet = make(chan *RetInfo, 1)
	//为异步调用返回信息管道分配内存，管道大小为l
	c.ChanAsynRet = make(chan *RetInfo, l)

	return c
}

//根据id获取所映射的func
func (c *Client) f(id interface{}, n int) (f interface{}, err error) {
	f = c.s.functions[id]

	//func未注册
	if f == nil {
		err = fmt.Errorf("function id %v: function not registered", id)
		return
	}

	var ok bool

	//根据n的值判断func类型是否匹配
	switch n {
	case 0: //n为0，无返回值
		_, ok = f.(func([]interface{}))
	case 1: //n为1，一个返回值
		_, ok = f.(func([]interface{}) interface{})
	case 2: //n为2，多个返回值
		_, ok = f.(func([]interface{}) []interface{})
	default:
		panic("bug")
	}

	//类型不匹配
	if !ok {
		err = fmt.Errorf("function id %v: return type mismatch", id)
	}

	return
}

//发起调用
func (c *Client) call(ci *CallInfo, block bool) (err error) {
	//延迟处理异常
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	if block { //阻塞，将调用消息通过管道传输到rpc服务器
		c.s.ChanCall <- ci
	} else { //不阻塞，当管道满时，返回"管道已满"错误（利用default特性检测chan是否已满）
		select {
		case c.s.ChanCall <- ci:
		default:
			err = errors.New("chanrpc channel full")
		}
	}

	return
}

//调用0。参数是切片，值任意，无返回值
func (c *Client) Call0(id interface{}, args ...interface{}) error {
	//根据id获取所映射的func
	f, err := c.f(id, 0)

	//func未注册或func类型不匹配
	if err != nil {
		return err
	}

	//发起调用
	err = c.call(&CallInfo{
		f:       f,
		args:    args,
		chanRet: c.chanSyncRet, //同步调用返回管道
	}, true)

	//调用失败
	if err != nil {
		return err
	}

	//读取结果（阻塞）
	ri := <-c.chanSyncRet

	return ri.err
}

//调用1。参数是切片，值任意，返回一个任意值
func (c *Client) Call1(id interface{}, args ...interface{}) (interface{}, error) {
	//根据id获取所映射的func
	f, err := c.f(id, 1)

	//func未注册或func类型不匹配
	if err != nil {
		return nil, err
	}

	//发起调用
	err = c.call(&CallInfo{
		f:       f,
		args:    args,
		chanRet: c.chanSyncRet,
	}, true)

	//调用失败
	if err != nil {
		return nil, err
	}

	//读取结果（阻塞）
	ri := <-c.chanSyncRet

	return ri.ret, ri.err
}

//调用N。参数是切片，返回值也是切片，值均为任意
func (c *Client) CallN(id interface{}, args ...interface{}) ([]interface{}, error) {
	//根据id获取所映射的func
	f, err := c.f(id, 2)

	//func未注册或func类型不匹配
	if err != nil {
		return nil, err
	}

	//发起调用
	err = c.call(&CallInfo{
		f:       f,
		args:    args,
		chanRet: c.chanSyncRet,
	}, true)

	//调用失败
	if err != nil {
		return nil, err
	}

	//读取结果（阻塞）
	ri := <-c.chanSyncRet

	return ri.ret.([]interface{}), ri.err
}

//发起异步调用（内部）
func (c *Client) asynCall(id interface{}, args []interface{}, cb interface{}, n int) error {
	//根据id获取所映射的func
	f, err := c.f(id, n)

	//func未注册或func类型不匹配
	if err != nil {
		return err
	}

	//发起调用
	err = c.call(&CallInfo{
		f:       f,
		args:    args,
		chanRet: c.ChanAsynRet, //异步调用返回管道
		cb:      cb,
	}, false)

	//调用失败
	if err != nil {
		return err
	}

	//增加计数器（待处理的异步调用）
	c.pendingAsynCall++

	return nil
}

//发起异步调用（导出），需要自己写c.Cb(<-c.ChanAsynRet)来执行回调
func (c *Client) AsynCall(id interface{}, _args ...interface{}) {
	//未提供回调函数参数，抛出错误（_args最后一个元素是回调函数，前面的是rpc调用的参数）
	if len(_args) < 1 {
		panic("callback function not found")
	}

	var args []interface{}

	//获取rpc调用的参数
	if len(_args) > 1 {
		args = _args[:len(_args)-1]
	}

	//获取回调函数
	cb := _args[len(_args)-1]

	//根据回调函数的类型，执行回调
	switch cb.(type) {
	case func(error): //只接收一个错误
		//发起异步调用（内部）
		err := c.asynCall(id, args, cb, 0)

		//内部调用失败，直接调用回调
		if err != nil {
			cb.(func(error))(err)
		}
	case func(interface{}, error): //接收一个返回值和一个错误
		err := c.asynCall(id, args, cb, 1)

		if err != nil {
			cb.(func(interface{}, error))(nil, err)
		}
	case func([]interface{}, error): //接收多个返回值和一个错误
		err := c.asynCall(id, args, cb, 2)

		if err != nil {
			cb.(func([]interface{}, error))(nil, err)
		}
	default: //非法回调函数
		panic("definition of callback function is invalid")
	}
}

//执行回调
func (c *Client) Cb(ri *RetInfo) {
	//根据回调函数的类型，执行回调
	switch ri.cb.(type) {
	case func(error): //只接收一个错误
		ri.cb.(func(error))(ri.err)
	case func(interface{}, error): //接受一个返回值和一个错误
		ri.cb.(func(interface{}, error))(ri.ret, ri.err)
	case func([]interface{}, error): //接收多个返回值和一个错误
		ri.cb.(func([]interface{}, error))(ri.ret.([]interface{}), ri.err)
	default: //非法回调函数
		panic("bug")
	}

	//减少计数器
	c.pendingAsynCall--
}

//关闭rpc客户端
func (c *Client) Close() {
	//如果还有未处理的异步调用，取出异步返回值，执行回调
	for c.pendingAsynCall > 0 {
		c.Cb(<-c.ChanAsynRet)
	}
}
