package module

import (
	"runtime"
	"squash/conf"
	"squash/log"
	"sync"
)

//模块接口
type Module interface {
	OnInit()                //初始化
	OnDestroy()             //销毁
	Run(closeSig chan bool) //运行
}

//模块
type module struct {
	mi       Module         //实现了模块接口的某对象
	closeSig chan bool      //传输关闭信号的管道
	wg       sync.WaitGroup //等待组
}

//模块数组，用于保存注册的模块
var mods []*module

//运行模块
func run(m *module) {
	//等待组+1
	m.wg.Add(1)
	//调用模块的Run函数（skeleton内实现，一个死循环）
	m.mi.Run(m.closeSig)
	//等待组减1
	m.wg.Done()
}

//销毁模块
func destroy(m *module) {
	//延迟处理异常
	defer func() {
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

	//调用模块的销毁方法
	m.mi.OnDestroy()
}

//注册模块
func Register(mi Module) {
	//新建一个模块
	m := new(module)
	//保存实现了模块接口的某对象
	m.mi = mi
	//创建传输关闭信号的管道
	m.closeSig = make(chan bool, 1)
	//保存模块到模块数组中
	mods = append(mods, m)
}

//初始化已注册模块
func Init() {
	//遍历所有注册的模块（从前往后），调用各个模块的OnInit函数
	for i := 0; i < len(mods); i++ {
		mods[i].mi.OnInit()
	}

	//遍历所有注册的模块（从前往后），在一个新的goroutine中运行模块
	for i := 0; i < len(mods); i++ {
		go run(mods[i])
	}
}

//销毁已注册模块
func Destroy() {
	//遍历所有注册的模块（反序，从后往前）
	for i := len(mods) - 1; i >= 0; i-- {
		m := mods[i]
		//向模块发送关闭信号（导致Run内的死循环结束，继续执行到m.wg.Done()）
		m.closeSig <- true
		//等待模块所在goroutine执行完成
		m.wg.Wait()
		//销毁模块
		destroy(m)
	}
}
