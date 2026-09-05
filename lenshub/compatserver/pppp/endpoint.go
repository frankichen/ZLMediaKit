package pppp

import (
	"encoding/binary"
	"fmt"
	"net"
)

const EndpointIPv4WireSize = 16

// EncodeEndpointIPv4 implements the 16-byte IPv4 endpoint layout observed in
// legacy PPPP MSG_HELLO_ACK / MSG_PUNCH_TO traffic: 00 02, little-endian
// port, little-endian IPv4 uint32, and eight zero bytes.
func EncodeEndpointIPv4(addr *net.UDPAddr) ([EndpointIPv4WireSize]byte, error) {
	var out [EndpointIPv4WireSize]byte
	if addr == nil || addr.Port < 1 || addr.Port > 65535 {
		return out, fmt.Errorf("invalid UDP endpoint")
	}
	ip4 := addr.IP.To4()
	if ip4 == nil {
		return out, fmt.Errorf("PPPP IPv4 endpoint required")
	}
	out[0], out[1] = 0x00, 0x02
	binary.LittleEndian.PutUint16(out[2:4], uint16(addr.Port))
	out[4], out[5], out[6], out[7] = ip4[3], ip4[2], ip4[1], ip4[0]
	return out, nil
}

func DecodeEndpointIPv4(data []byte) (*net.UDPAddr, error) {
	if len(data) < EndpointIPv4WireSize {
		return nil, fmt.Errorf("endpoint payload too short: %d", len(data))
	}
	if data[0] != 0x00 || data[1] != 0x02 {
		return nil, fmt.Errorf("unsupported endpoint family marker %02x%02x", data[0], data[1])
	}
	port := int(binary.LittleEndian.Uint16(data[2:4]))
	ip := net.IPv4(data[7], data[6], data[5], data[4])
	return &net.UDPAddr{IP: ip, Port: port}, nil
}
