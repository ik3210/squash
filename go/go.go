package g

import (
	"container/list"
	"runtime"
	"squash/conf"
	"squash/log"
	"sync"
)

//Go类型定义
type Go struct {
	ChanCb    chan func() //回调管道，用于传输回调函数
	pendingGo int         //待处理回调函数计数器
}

//线性（串行）Go类型定义
type LinearGo struct {
	f  func() //执行函数
	cb func() //回调函数
}

//线性上下文类型定义
type LinearContext struct {
	g              *Go        //一个Go
	linearGo       *list.List //链表
	mutexLinearGo  sync.Mutex //链表互斥锁
	mutexExecution sync.Mutex //执行互斥锁
}

//创建Go
func New(l int) *Go {
	//创建一个Go
	g := new(Go)
	//创建回调管道
	g.ChanCb = make(chan func(), l)

	return g
}

//一般的Go，执行一个比较耗时的操作，执行完成后将回调函数通过回调管道发送回原goroutine执行
func (g *Go) Go(f func(), cb func()) {
	//增加待处理回调函数计数器
	g.pendingGo++

	//在一个新的goroutine中执行
	go func() {
		//延迟执行
		defer func() {
			//当f执行完成后，将回调发送到回调管道中
			g.ChanCb <- cb

			//处理异常
			if r := recover(); r != nil {
				if conf.LenStackBuf > 0 { //配置了调用栈踪迹缓冲长度，将当前goroutine的调用栈踪迹格式化后写入到buf中
					buf := make([]byte, conf.LenStackBuf)
					l := runtime.Stack(buf, false)
					log.Error("%v: %s", r, buf[:l])
				} else {
					log.Error("%v", r)
				}
			}
		}()

		//在新的gouroutine中执行f（一个耗时的操作）
		f()
	}()
}

//执行回调
func (g *Go) Cb(cb func()) {
	defer func() {
		//减少待处理回调函数计数器
		g.pendingGo--

		//处理异常
		if r := recover(); r != nil {
			if conf.LenStackBuf > 0 { //配置了调用栈踪迹缓冲长度，将当前goroutine的调用栈踪迹格式化后写入到buf中
				buf := make([]byte, conf.LenStackBuf)
				l := runtime.Stack(buf, false)
				log.Error("%v: %s", r, buf[:l])
			} else {
				log.Error("%v", r)
			}
		}
	}()

	//回调函数不为空，执行回调函数
	if cb != nil {
		cb()
	}
}

//关闭Go
func (g *Go) Close() {
	//有待处理的回调函数，从管道中读出来执行
	for g.pendingGo > 0 {
		g.Cb(<-g.ChanCb)
	}
}

//创建线性上下文
func (g *Go) NewLinearContext() *LinearContext {
	//创建一个线性上下文
	c := new(LinearContext)
	//引用Go
	c.g = g
	//创建一个链表
	c.linearGo = list.New()

	return c
}

//线性上下文的Go（串行）
func (c *LinearContext) Go(f func(), cb func()) {
	//增加待处理回调函数计数器
	c.g.pendingGo++

	//链表加锁
	c.mutexLinearGo.Lock()
	//向链表添加元素
	c.linearGo.PushBack(&LinearGo{f: f, cb: cb})
	//链表解锁
	c.mutexLinearGo.Unlock()

	//在新的goroutine中执行
	go func() {
		//加锁，后来的Go将会阻塞在这里，直到该Go执行完成
		c.mutexExecution.Lock()
		//延迟解锁
		defer c.mutexExecution.Unlock()

		//链表加锁
		c.mutexLinearGo.Lock()
		//从表头移出一个元素
		e := c.linearGo.Remove(c.linearGo.Front()).(*LinearGo)
		//链表解锁
		c.mutexLinearGo.Unlock()

		defer func() {
			//当f执行完成后，将回调发送到回调管道中
			c.g.ChanCb <- e.cb

			//处理异常
			if r := recover(); r != nil {
				if conf.LenStackBuf > 0 { //配置了调用栈踪迹缓冲长度，将当前goroutine的调用栈踪迹格式化后写入到buf中
					buf := make([]byte, conf.LenStackBuf)
					l := runtime.Stack(buf, false)
					log.Error("%v: %s", r, buf[:l])
				} else {
					log.Error("%v", r)
				}
			}
		}()

		//执行函数
		e.f()
	}()
}
