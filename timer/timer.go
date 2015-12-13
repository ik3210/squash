package timer

import (
	"errors"
	"runtime"
	"squash/conf"
	"squash/log"
	"time"
)

//定时器
type Timer struct {
	t  *time.Timer //底层定时器
	cb func()      //回调
}

//计划任务
type Cron struct {
	t *Timer
}

//分发器
type Dispatcher struct {
	ChanTimer chan *Timer //用于传输定时器的管道
}

//创建分发器
func NewDispatcher(l int) *Dispatcher {
	//创建分发器
	disp := new(Dispatcher)
	//创建用于传输定时器的管道
	disp.ChanTimer = make(chan *Timer, l)
	return disp
}

//注册定时器
func (disp *Dispatcher) AfterFunc(d time.Duration, cb func()) *Timer {
	//创建定时器
	t := new(Timer)
	//设置回调
	t.cb = cb
	//等待时间段d过去之后，将定时器发送到分发器管道中
	t.t = time.AfterFunc(d, func() {
		disp.ChanTimer <- t
	})

	//返回定时器
	return t
}

//停止定时器
func (t *Timer) Stop() {
	//停止底层定时器
	t.t.Stop()
	//置空回调函数
	t.cb = nil
}

//调用定时器的回调函数
func (t *Timer) Cb() {
	//延迟执行
	defer func() {
		//置空回调
		t.cb = nil
		//捕获异常
		if r := recover(); r != nil {
			if conf.LenStackBuf > 0 { //配置了堆栈buf长度大于0，打印堆栈信息
				buf := make([]byte, conf.LenStackBuf)
				l := runtime.Stack(buf, false)
				log.Error("%v: %s", r, buf[:l])
			} else { //打印异常
				log.Error("%v", r)
			}
		}
	}()

	//回调不为空，调用回调
	if t.cb != nil {
		t.cb()
	}
}

//注册计划任务
func (disp *Dispatcher) CronFunc(expr string, _cb func()) (*Cron, error) {
	//创建一个cron表达式
	cronExpr, err := NewCronExpr(expr)
	//创建失败
	if err != nil {
		return nil, err
	}

	//获取当前时间
	now := time.Now()
	//获取下一个时间
	nextTime := cronExpr.Next(now)
	//下一个时间为零值，返回错误，不注册后续的计划任务
	if nextTime.IsZero() {
		return nil, errors.New("next time not found")
	}

	//创建一个计划任务
	cron := new(Cron)
	//定义回调
	var cb func()
	cb = func() {
		//延迟执行回调（注册完毕下一次调用回调的定时器，才执行本次回调）
		defer _cb()

		//获取当前时间
		now := time.Now()
		//获取下一个时间
		nextTime := cronExpr.Next(now)
		//下一个时间为零值，不注册后续的计划任务
		if nextTime.IsZero() {
			return
		}

		//计算时间差值，注册下一次调用回调的定时器
		cron.t = disp.AfterFunc(nextTime.Sub(now), cb)
	}

	//注册第一次计划任务
	cron.t = disp.AfterFunc(nextTime.Sub(now), cb)

	return cron, nil
}

//停止计划任务
func (c *Cron) Stop() {
	c.t.Stop()
}
