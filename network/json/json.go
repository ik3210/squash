package json

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"squash/chanrpc"
	"squash/log"
)

//处理器
type Processor struct {
	msgInfo map[string]*MsgInfo //消息信息映射
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
	//创建消息信息映射
	p.msgInfo = make(map[string]*MsgInfo)

	return p
}

//注册消息
func (p *Processor) Register(msg interface{}) {
	//获取消息类型
	msgType := reflect.TypeOf(msg)

	//判断消息的合法性（不能为空，需要是指针）
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("json message pointer required")
	}

	//获取消息本身（不是指针）的名字，作为消息ID
	msgID := msgType.Elem().Name()

	//获取失败
	if msgID == "" {
		log.Fatal("unnamed json message")
	}

	//消息已注册
	if _, ok := p.msgInfo[msgID]; ok {
		log.Fatal("message %v is already registered", msgID)
	}

	//新建一个消息信息
	i := new(MsgInfo)
	//保存消息类型
	i.msgType = msgType
	//保存消息信息到映射中
	p.msgInfo[msgID] = i
}

//设置路由
func (p *Processor) SetRouter(msg interface{}, msgRouter *chanrpc.Server) {
	//获取消息类型
	msgType := reflect.TypeOf(msg)

	//判断消息的合法性（不能为空，需要是指针）
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("json message pointer required")
	}

	//获取消息本身（不是指针）的名字，作为消息ID
	msgID := msgType.Elem().Name()
	//根据消息ID获取消息信息
	i, ok := p.msgInfo[msgID]

	//获取消息信息失败
	if !ok {
		log.Fatal("message %v not registered", msgID)
	}

	//保存rpc服务器引用
	i.msgRouter = msgRouter
}

//设置消息处理函数
func (p *Processor) SetHandler(msg interface{}, msgHandler MsgHandler) {
	//获取消息类型
	msgType := reflect.TypeOf(msg)

	//判断消息的合法性（不能为空，需要是指针）
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("json message pointer required")
	}

	//获取消息本身（不是指针）的名字，作为消息ID
	msgID := msgType.Elem().Name()
	//根据消息ID获取消息信息
	i, ok := p.msgInfo[msgID]

	//获取消息信息失败
	if !ok {
		log.Fatal("message %v not registered", msgID)
	}

	//保存消息处理函数
	i.msgHandler = msgHandler
}

//路由
func (p *Processor) Route(msg interface{}, userData interface{}) error {
	//获取消息类型
	msgType := reflect.TypeOf(msg)

	//判断消息的合法性（不能为空，需要是指针）
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		return errors.New("json message pointer required")
	}

	//获取消息本身（不是指针）的名字，作为消息ID
	msgID := msgType.Elem().Name()
	//根据消息ID获取消息信息
	i, ok := p.msgInfo[msgID]

	//获取消息信息失败
	if !ok {
		return fmt.Errorf("message %v not registered", msgID)
	}

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
	//用于存储解码数据
	var m map[string]json.RawMessage
	//解码
	err := json.Unmarshal(data, &m)

	//解码失败
	if err != nil {
		return nil, err
	}

	//m的长度必为1，也就是只有一个键值对：msgID和未解码的data（data是原生json对象）
	if len(m) != 1 {
		return nil, errors.New("invalid json data")
	}

	//取出msgID和未解码的data
	for msgID, data := range m {
		//根据消息ID获取消息信息
		i, ok := p.msgInfo[msgID]

		//获取失败
		if !ok {
			return nil, fmt.Errorf("message %v not registered", msgID)
		}

		//用于存储解码数据
		msg := reflect.New(i.msgType.Elem()).Interface()

		//解码data
		return msg, json.Unmarshal(data, msg)
	}

	panic("bug")
}

//编码消息
func (p *Processor) Marshal(msg interface{}) ([]byte, error) {
	//获取消息类型
	msgType := reflect.TypeOf(msg)

	//判断消息的合法性（不能为空，需要是指针）
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		return nil, errors.New("json message pointer required")
	}

	//获取消息本身（不是指针）的名字，作为消息ID
	msgID := msgType.Elem().Name()

	//获取消息信息失败
	if _, ok := p.msgInfo[msgID]; !ok {
		return nil, fmt.Errorf("message %v not registered", msgID)
	}

	//创建消息ID映射
	m := map[string]interface{}{msgID: msg}

	//编码
	return json.Marshal(m)
}
