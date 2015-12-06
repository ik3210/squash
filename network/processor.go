package network

type Processor interface {
	Route(msg interface{}, userData interface{}) error //路由
	Unmarshal(data []byte) (interface{}, error)        //解码
	Marshal(msg interface{}) ([][]byte, error)         //编码
}
