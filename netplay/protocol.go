// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package netplay

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// Magic 帧头魔数（0x4750 = "GP"）。
const Magic uint16 = 0x4750

// Version 协议版本：握手不匹配直接拒绝，避免新旧客户端互相误解帧。
const Version uint8 = 1

// headerSize 帧头长度：magic(2) + version(1) + type(1) + seq(4) + len(4)。
const headerSize = 12

// MsgType 消息类型。
type MsgType uint8

const (
	// MsgHandshake 客户端→服务端：版本与昵称。
	MsgHandshake MsgType = 1
	// MsgHandshakeAck 服务端→客户端：分配到的玩家 ID。
	MsgHandshakeAck MsgType = 2
	// MsgSnapshot 状态快照（定期广播）。
	MsgSnapshot MsgType = 3
	// MsgInput 帧同步输入。
	MsgInput MsgType = 4
	// MsgChat 文本消息。
	MsgChat MsgType = 5
	// MsgPing 心跳请求。
	MsgPing MsgType = 6
	// MsgPong 心跳应答。
	MsgPong MsgType = 7
	// MsgJoin 玩家加入公告。
	MsgJoin MsgType = 8
	// MsgLeave 玩家离开公告。
	MsgLeave MsgType = 9
)

// Message 一条已解码消息。
type Message struct {
	Type    MsgType
	Seq     uint32
	Payload []byte
}

// EncodeMessage 编码成帧（含 12 字节帧头）。
func EncodeMessage(m Message) ([]byte, error) {
	n := len(m.Payload)
	if n > MaxFrameSize {
		return nil, ErrFrameTooLarge
	}
	buf := make([]byte, headerSize+n)
	binary.BigEndian.PutUint16(buf[0:2], Magic)
	buf[2] = Version
	buf[3] = byte(m.Type)
	binary.BigEndian.PutUint32(buf[4:8], m.Seq)
	binary.BigEndian.PutUint32(buf[8:12], uint32(n))
	copy(buf[headerSize:], m.Payload)
	return buf, nil
}

// DecodeMessage 解码一帧（校验魔数 / 版本 / 长度）。
func DecodeMessage(b []byte) (Message, error) {
	if len(b) < headerSize {
		return Message{}, fmt.Errorf("netplay: 帧太短（%d 字节）", len(b))
	}
	if binary.BigEndian.Uint16(b[0:2]) != Magic {
		return Message{}, fmt.Errorf("netplay: 魔数不匹配")
	}
	if b[2] != Version {
		return Message{}, fmt.Errorf("netplay: 协议版本 %d 与服务版本 %d 不匹配", b[2], Version)
	}
	n := binary.BigEndian.Uint32(b[8:12])
	if int(n) > MaxFrameSize || int(n) != len(b)-headerSize {
		return Message{}, fmt.Errorf("netplay: 长度字段 %d 与实际 %d 不符", n, len(b)-headerSize)
	}
	return Message{
		Type:    MsgType(b[3]),
		Seq:     binary.BigEndian.Uint32(b[4:8]),
		Payload: b[headerSize:],
	}, nil
}

// Handshake 客户端握手数据。
type Handshake struct {
	Version uint8  `json:"version"`
	Name    string `json:"name"`
}

// HandshakeAck 服务端握手应答。
type HandshakeAck struct {
	Version uint8  `json:"version"`
	ID      uint64 `json:"id"`
	Message string `json:"message,omitempty"`
}

// EncodeJSON 把任意值编码为消息载荷（上层状态 / 输入用它就行）。
func EncodeJSON(v any) ([]byte, error) { return json.Marshal(v) }

// DecodeJSON 把载荷解回结构体。
func DecodeJSON(b []byte, v any) error { return json.Unmarshal(b, v) }
