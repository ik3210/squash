package network

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
)

//消息解析器
// --------------
// | len | data |
// --------------
type MsgParser struct {
	lenMsgLen    int    //存储消息长度信息所占用的字节数
	minMsgLen    uint32 //最小消息长度
	maxMsgLen    uint32 //最大消息长度
	littleEndian bool   //是否小端
}

//创建消息解析器
func NewMsgParser() *MsgParser {
	p := new(MsgParser)
	p.lenMsgLen = 2
	p.minMsgLen = 1
	p.maxMsgLen = 4096
	p.littleEndian = false

	return p
}

//设置消息长度
func (p *MsgParser) SetMsgLen(lenMsgLen int, minMsgLen uint32, maxMsgLen uint32) {
	//存储消息长度信息所占用的字节数有效值为1、2、4
	if lenMsgLen == 1 || lenMsgLen == 2 || lenMsgLen == 4 {
		p.lenMsgLen = lenMsgLen
	}

	//最小消息长度不为0
	if minMsgLen != 0 {
		p.minMsgLen = minMsgLen
	}

	//最大消息长度不为0
	if maxMsgLen != 0 {
		p.maxMsgLen = maxMsgLen
	}

	var max uint32

	//根据存储消息长度信息所占用的字节数计算data最大长度
	switch p.lenMsgLen {
	case 1:
		max = math.MaxUint8
	case 2:
		max = math.MaxUint16
	case 4:
		max = math.MaxUint32
	}

	//最小消息长度不大于data最大长度
	if p.minMsgLen > max {
		p.minMsgLen = max
	}

	//最大消息长度不大于data最大长度
	if p.maxMsgLen > max {
		p.maxMsgLen = max
	}
}

//设置字节序是否小端
func (p *MsgParser) SetByteOrder(littleEndian bool) {
	p.littleEndian = littleEndian
}

//读取消息
func (p *MsgParser) Read(conn *TCPConn) ([]byte, error) {
	var b [4]byte
	//根据存储消息长度信息所占用的字节数获取对应长度的字符切片
	bufMsgLen := b[:p.lenMsgLen]

	//读取消息长度
	if _, err := io.ReadFull(conn, bufMsgLen); err != nil {
		return nil, err
	}

	var msgLen uint32

	//解析长度，原理见"发送消息"中的"写入长度"
	switch p.lenMsgLen {
	case 1:
		msgLen = uint32(bufMsgLen[0])
	case 2:
		if p.littleEndian {
			msgLen = uint32(binary.LittleEndian.Uint16(bufMsgLen))
		} else {
			msgLen = uint32(binary.BigEndian.Uint16(bufMsgLen))
		}
	case 4:
		if p.littleEndian {
			msgLen = binary.LittleEndian.Uint32(bufMsgLen)
		} else {
			msgLen = binary.BigEndian.Uint32(bufMsgLen)
		}
	}

	//检查长度是否合法
	if msgLen > p.maxMsgLen {
		return nil, errors.New("message too long")
	} else if msgLen < p.minMsgLen {
		return nil, errors.New("message too short")
	}

	//创建对应长度的字节切片
	msgData := make([]byte, msgLen)

	//读取数据
	if _, err := io.ReadFull(conn, msgData); err != nil {
		return nil, err
	}

	return msgData, nil
}

//发送消息
func (p *MsgParser) Write(conn *TCPConn, args ...[]byte) error {
	var msgLen uint32

	//计算消息长度
	for i := 0; i < len(args); i++ {
		msgLen += uint32(len(args[i]))
	}

	//检查长度是否合法
	if msgLen > p.maxMsgLen {
		return errors.New("message too long")
	} else if msgLen < p.minMsgLen {
		return errors.New("message too short")
	}

	//创建(lenMsgLen+msgLen)长度的字节切片
	msg := make([]byte, uint32(p.lenMsgLen)+msgLen)

	//写入长度
	switch p.lenMsgLen {
	case 1: //将uin8类型的长度信息转化为ASCII码到msg
		msg[0] = byte(msgLen)
	case 2: //根据字节序将uint16类型的长度信息序列化到msg
		if p.littleEndian {
			binary.LittleEndian.PutUint16(msg, uint16(msgLen))
		} else {
			binary.BigEndian.PutUint16(msg, uint16(msgLen))
		}
	case 4: //根据字节序将uint16类型的长度信息序列化到msg
		if p.littleEndian {
			binary.LittleEndian.PutUint32(msg, msgLen)
		} else {
			binary.BigEndian.PutUint32(msg, msgLen)
		}
	}

	l := p.lenMsgLen

	//遍历所有字节切片，复制数据
	for i := 0; i < len(args); i++ {
		copy(msg[l:], args[i])
		l += len(args[i])
	}

	//发送数据
	conn.Write(msg)

	return nil
}
