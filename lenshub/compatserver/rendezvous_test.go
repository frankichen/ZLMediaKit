package main

import (
	"net"
	"testing"
	"time"

	"github.com/frankichen/ZLMediaKit/lenshub/compatserver/pppp"
)

func testDID(t *testing.T) pppp.DID {
	t.Helper()
	did, err := pppp.ParseDID("PPCS-020070-BNRLZ")
	if err != nil {
		t.Fatal(err)
	}
	return did
}

func TestHelloReturnsObservedEndpoint(t *testing.T) {
	cfg := defaultConfig()
	reg := newRegistry()
	remote := &net.UDPAddr{IP: net.ParseIP("203.0.113.8"), Port: 45678}
	out := handleWirePacket(cfg, reg, remote, decodedDatagram{packet: pppp.Packet{Type: pppp.MsgHello}})
	if len(out) != 1 || out[0].packet.Type != pppp.MsgHelloAck {
		t.Fatalf("unexpected %#v", out)
	}
	decoded, err := pppp.DecodeEndpointIPv4(out[0].packet.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.String() != remote.String() {
		t.Fatalf("got %s want %s", decoded, remote)
	}
}

func TestDeviceLoginAndP2PRequestProducePunchPair(t *testing.T) {
	cfg := defaultConfig()
	cfg.AllowedDIDPrefixes = []string{"PPCS"}
	reg := newRegistry()
	did := testDID(t)
	wire, _ := did.Wire20()
	device := &net.UDPAddr{IP: net.ParseIP("198.51.100.20"), Port: 23001}
	controller := &net.UDPAddr{IP: net.ParseIP("203.0.113.44"), Port: 34002}

	loginOut := handleWirePacket(cfg, reg, device, decodedDatagram{packet: pppp.Packet{Type: pppp.MsgDevLogin, Payload: append(wire[:], make([]byte, 20)...)}})
	if len(loginOut) != 1 || loginOut[0].packet.Type != pppp.MsgDevLoginAck || loginOut[0].packet.Payload[0] != 0 {
		t.Fatalf("login failed %#v", loginOut)
	}

	reqPayload := append(wire[:], make([]byte, 16)...)
	out := handleWirePacket(cfg, reg, controller, decodedDatagram{packet: pppp.Packet{Type: pppp.MsgP2PReq, Payload: reqPayload}})
	if len(out) != 3 {
		t.Fatalf("got %d outbound packets", len(out))
	}
	if out[0].packet.Type != pppp.MsgP2PReqAck || out[1].packet.Type != pppp.MsgPunchTo || out[2].packet.Type != pppp.MsgPunchTo {
		t.Fatalf("unexpected message sequence")
	}
	toController, _ := pppp.DecodeEndpointIPv4(out[1].packet.Payload)
	toDevice, _ := pppp.DecodeEndpointIPv4(out[2].packet.Payload)
	if toController.String() != device.String() {
		t.Fatalf("controller target got %s want %s", toController, device)
	}
	if toDevice.String() != controller.String() {
		t.Fatalf("device target got %s want %s", toDevice, controller)
	}
	if out[2].to.String() != device.String() {
		t.Fatalf("server sent device punch instruction to %s", out[2].to)
	}
}

func TestP2PRequestUnknownDeviceFailsClosed(t *testing.T) {
	cfg := defaultConfig()
	reg := newRegistry()
	did := testDID(t)
	wire, _ := did.Wire20()
	controller := &net.UDPAddr{IP: net.ParseIP("203.0.113.44"), Port: 34002}
	out := handleWirePacket(cfg, reg, controller, decodedDatagram{packet: pppp.Packet{Type: pppp.MsgP2PReq, Payload: append(wire[:], make([]byte, 16)...)}})
	if len(out) != 1 || out[0].packet.Type != pppp.MsgP2PReqAck || out[0].packet.Payload[0] == 0 {
		t.Fatalf("unknown device did not fail closed")
	}
}

func TestPresenceExpires(t *testing.T) {
	cfg := defaultConfig()
	cfg.PresenceTTLSeconds = 1
	reg := newRegistry()
	did := testDID(t)
	reg.devices[did.String()] = DevicePresence{DID: did.String(), RemoteAddr: "198.51.100.20:23001", LastSeenUTC: time.Now().Add(-2 * time.Second), Obfuscated: false}
	wire, _ := did.Wire20()
	controller := &net.UDPAddr{IP: net.ParseIP("203.0.113.44"), Port: 34002}
	out := handleWirePacket(cfg, reg, controller, decodedDatagram{packet: pppp.Packet{Type: pppp.MsgP2PReq, Payload: append(wire[:], make([]byte, 16)...)}})
	if len(out) != 1 || out[0].packet.Payload[0] == 0 {
		t.Fatalf("expired presence should fail")
	}
}
