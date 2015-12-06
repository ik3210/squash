package network

//代理接口
type Agent interface {
	Run()     //运行
	OnClose() //关闭
}
