package pppp

import (
	"bytes"
	"encoding/hex"
	"net"
	"testing"
)

func TestPacketRoundTrip(t *testing.T) {
	want := []byte{0xF1, MsgP2PReqAck, 0x00, 0x04, 0, 0, 0, 0}
	pkt, err := ParsePacket(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pkt.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x want %x", got, want)
	}
}

func TestDIDRoundTrip(t *testing.T) {
	did, err := ParseDID("PPCS-020070-BNRLZ")
	if err != nil {
		t.Fatal(err)
	}
	wire, err := did.Wire20()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ParseDIDWire(wire[:])
	if err != nil {
		t.Fatal(err)
	}
	if decoded.String() != "PPCS-020070-BNRLZ" {
		t.Fatalf("decoded %s", decoded.String())
	}
}

func TestEndpointMatchesPublishedPunchSample(t *testing.T) {
	addr := &net.UDPAddr{IP: net.ParseIP("166.199.184.97"), Port: 4396}
	wire, err := EncodeEndpointIPv4(addr)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := hex.DecodeString("00022c1161b8c7a60000000000000000")
	if !bytes.Equal(wire[:], want) {
		t.Fatalf("got %x want %x", wire, want)
	}
	decoded, err := DecodeEndpointIPv4(wire[:])
	if err != nil {
		t.Fatal(err)
	}
	if decoded.String() != addr.String() {
		t.Fatalf("decoded %s want %s", decoded, addr)
	}
}

func TestObfuscationRoundTrip(t *testing.T) {
	clear := []byte{0xF1, MsgHello, 0, 0, 1, 2, 3, 4}
	enc := Obfuscate("test-psk", clear)
	if bytes.Equal(enc, clear) {
		t.Fatal("obfuscation did not change payload")
	}
	got := Deobfuscate("test-psk", enc)
	if !bytes.Equal(got, clear) {
		t.Fatalf("got %x want %x", got, clear)
	}
}
