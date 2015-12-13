package network

//消息处理器接口
type Processor interface {
	Route(msg interface{}, userData interface{}) error //路由
	Unmarshal(data []byte) (interface{}, error)        //解码
	Marshal(msg interface{}) ([][]byte, error)         //编码
}
