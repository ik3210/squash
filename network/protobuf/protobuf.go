package protobuf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"math"
	"reflect"
	"squash/chanrpc"
	"squash/log"
)

//处理器
// -------------------------
// | id | protobuf message |
// -------------------------
type Processor struct {
	littleEndian bool                    //是否小端
	msgInfo      []*MsgInfo              //消息信息切片
	msgID        map[reflect.Type]uint16 //消息ID映射
}

//消息信息
type MsgInfo struct {
	msgType    reflect.Type    //消息类型
	msgRouter  *chanrpc.Server //处理消息的rpc服务器
	msgHandler MsgHandler      //消息处理函数
}

//消息处理函数
type MsgHandler func([]interface{})

//创建一个处理器
func NewProcessor() *Processor {
	//创建处理器
	p := new(Processor)
	//字节序默认大端
	p.littleEndian = false
	//创建消息ID映射
	p.msgID = make(map[reflect.Type]uint16)

	return p
}

//设置字节序是否小端
func (p *Processor) SetByteOrder(littleEndian bool) {
	p.littleEndian = littleEndian
}

//注册消息
func (p *Processor) Register(msg proto.Message) {
	//获取消息类型
	msgType := reflect.TypeOf(msg)

	//判断消息的合法性（不能为空，需要是指针）
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("protobuf message pointer required")
	}

	//消息已注册
	if _, ok := p.msgID[msgType]; ok {
		log.Fatal("message %s is already registered", msgType)
	}

	//消息切片已满
	if len(p.msgInfo) >= math.MaxUint16 {
		log.Fatal("too many protobuf messages (max = %v)", math.MaxUint16)
	}

	//新建一个消息信息
	i := new(MsgInfo)
	//保存消息类型
	i.msgType = msgType
	//保存消息信息到切片中
	p.msgInfo = append(p.msgInfo, i)
	//保存消息ID到映射中
	p.msgID[msgType] = uint16(len(p.msgInfo) - 1)
}

//设置路由
func (p *Processor) SetRouter(msg proto.Message, msgRouter *chanrpc.Server) {
	//获取消息类型
	msgType := reflect.TypeOf(msg)
	//获取消息ID
	id, ok := p.msgID[msgType]

	//消息未注册
	if !ok {
		log.Fatal("message %s not registered", msgType)
	}

	//保存rpc服务器引用
	p.msgInfo[id].msgRouter = msgRouter
}

//设置消息处理函数
func (p *Processor) SetHandler(msg proto.Message, msgHandler MsgHandler) {
	//获取消息的类型
	msgType := reflect.TypeOf(msg)
	//获取消息ID
	id, ok := p.msgID[msgType]

	//消息未注册
	if !ok {
		log.Fatal("message %s not registered", msgType)
	}

	//保存消息处理函数
	p.msgInfo[id].msgHandler = msgHandler
}

//路由
func (p *Processor) Route(msg interface{}, userData interface{}) error {
	//获取消息类型
	msgType := reflect.TypeOf(msg)
	//获取消息ID
	id, ok := p.msgID[msgType]

	//判断消息是否已经注册
	if !ok {
		return fmt.Errorf("message %s not registered", msgType)
	}

	//消息未注册
	i := p.msgInfo[id]

	//调用消息处理函数
	if i.msgHandler != nil {
		i.msgHandler([]interface{}{msg, userData})
	}

	//rpc服务器自己发起调用
	if i.msgRouter != nil {
		i.msgRouter.Go(msgType, msg, userData)
	}

	return nil
}

//解码消息
func (p *Processor) Unmarshal(data []byte) (interface{}, error) {
	//消息过短（[][]byte{id, data}为2字节）
	if len(data) < 2 {
		return nil, errors.New("protobuf data too short")
	}

	var id uint16

	//获取消息ID
	if p.littleEndian {
		id = binary.LittleEndian.Uint16(data)
	} else {
		id = binary.BigEndian.Uint16(data)
	}

	//ID超出消息切片长度
	if id >= uint16(len(p.msgInfo)) {
		return nil, fmt.Errorf("message id %v not registered", id)
	}

	//用于存储解码数据
	msg := reflect.New(p.msgInfo[id].msgType.Elem()).Interface()

	//解码data
	return msg, proto.UnmarshalMerge(data[2:], msg.(proto.Message))
}

//编码消息
func (p *Processor) Marshal(msg interface{}) ([][]byte, error) {
	//获取消息类型
	msgType := reflect.TypeOf(msg)
	//获取消息ID
	_id, ok := p.msgID[msgType]

	//消息未注册
	if !ok {
		err := fmt.Errorf("message %s not registered", msgType)
		return nil, err
	}

	//创建消息ID对应的字节切片
	id := make([]byte, 2)

	//根据字节序将_id序列化到id字节切片上
	if p.littleEndian {
		binary.LittleEndian.PutUint16(id, _id)
	} else {
		binary.BigEndian.PutUint16(id, _id)
	}

	//编码
	data, err := proto.Marshal(msg.(proto.Message))

	return [][]byte{id, data}, err
}

//对所有消息应用函数
func (p *Processor) Range(f func(id uint16, t reflect.Type)) {
	for id, i := range p.msgInfo {
		f(uint16(id), i.msgType)
	}
}
