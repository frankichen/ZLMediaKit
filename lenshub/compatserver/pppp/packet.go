package pppp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	MagicF1 byte = 0xF1

	MsgHello        byte = 0x00
	MsgHelloAck     byte = 0x01
	MsgDevLogin     byte = 0x10
	MsgDevLoginAck  byte = 0x11
	MsgDevOnlineReq byte = 0x18
	MsgP2PReq       byte = 0x20
	MsgP2PReqAck    byte = 0x21
	MsgPunchTo      byte = 0x40
	MsgPunchPkt     byte = 0x41
	MsgP2PReady     byte = 0x42
	MsgListReq      byte = 0x67
	MsgListReqAck   byte = 0x69
	MsgDRW          byte = 0xD0
	MsgDRWAck       byte = 0xD1
	MsgAlive        byte = 0xE0
	MsgAliveAck     byte = 0xE1
	MsgClose        byte = 0xF0
)

var (
	ErrPacketTooShort = errors.New("pppp packet too short")
	ErrBadMagic       = errors.New("pppp unsupported magic")
	ErrLengthMismatch = errors.New("pppp payload length mismatch")
)

type Packet struct {
	Magic   byte
	Type    byte
	Payload []byte
}

func ParsePacket(data []byte) (Packet, error) {
	if len(data) < 4 {
		return Packet{}, ErrPacketTooShort
	}
	if data[0] != MagicF1 {
		return Packet{}, fmt.Errorf("%w: 0x%02x", ErrBadMagic, data[0])
	}
	payloadLen := int(binary.BigEndian.Uint16(data[2:4]))
	if len(data) != 4+payloadLen {
		return Packet{}, fmt.Errorf("%w: header=%d actual=%d", ErrLengthMismatch, payloadLen, len(data)-4)
	}
	payload := append([]byte(nil), data[4:]...)
	return Packet{Magic: data[0], Type: data[1], Payload: payload}, nil
}

func (p Packet) MarshalBinary() ([]byte, error) {
	magic := p.Magic
	if magic == 0 {
		magic = MagicF1
	}
	if magic != MagicF1 {
		return nil, fmt.Errorf("%w: 0x%02x", ErrBadMagic, magic)
	}
	if len(p.Payload) > 0xFFFF {
		return nil, fmt.Errorf("pppp payload too large: %d", len(p.Payload))
	}
	out := make([]byte, 4+len(p.Payload))
	out[0] = magic
	out[1] = p.Type
	binary.BigEndian.PutUint16(out[2:4], uint16(len(p.Payload)))
	copy(out[4:], p.Payload)
	return out, nil
}

func StatusPayload(code byte) []byte {
	return []byte{code, 0, 0, 0}
}
