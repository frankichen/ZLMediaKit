package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/frankichen/ZLMediaKit/lenshub/compatserver/pppp"
)

func main() {
	addrFlag := flag.String("addr", "127.0.0.1:12305", "PPPP rendezvous UDP address")
	didFlag := flag.String("did", "PPCS-020070-BNRLZ", "synthetic test DID")
	flag.Parse()
	if err := run(*addrFlag, *didFlag); err != nil {
		fmt.Fprintln(os.Stderr, "pppp probe:", err)
		os.Exit(1)
	}
	fmt.Println("pppp probe: OK")
}

func run(serverString, didString string) error {
	server, err := net.ResolveUDPAddr("udp4", serverString)
	if err != nil {
		return err
	}
	did, err := pppp.ParseDID(didString)
	if err != nil {
		return err
	}
	wireDID, err := did.Wire20()
	if err != nil {
		return err
	}

	device, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return err
	}
	defer device.Close()
	controller, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return err
	}
	defer controller.Close()
	deadline := time.Now().Add(3 * time.Second)
	_ = device.SetDeadline(deadline)
	_ = controller.SetDeadline(deadline)

	if err := send(controller, server, pppp.Packet{Type: pppp.MsgHello}); err != nil {
		return err
	}
	hello, _, err := recv(controller)
	if err != nil {
		return err
	}
	if hello.Type != pppp.MsgHelloAck {
		return fmt.Errorf("hello ack type 0x%02x", hello.Type)
	}

	loginPayload := append(append([]byte(nil), wireDID[:]...), make([]byte, 20)...)
	if err := send(device, server, pppp.Packet{Type: pppp.MsgDevLogin, Payload: loginPayload}); err != nil {
		return err
	}
	loginAck, _, err := recv(device)
	if err != nil {
		return err
	}
	if loginAck.Type != pppp.MsgDevLoginAck || len(loginAck.Payload) != 4 || loginAck.Payload[0] != 0 {
		return fmt.Errorf("device login rejected: type=0x%02x payload=%x", loginAck.Type, loginAck.Payload)
	}

	reqPayload := append(append([]byte(nil), wireDID[:]...), make([]byte, 16)...)
	if err := send(controller, server, pppp.Packet{Type: pppp.MsgP2PReq, Payload: reqPayload}); err != nil {
		return err
	}

	var gotAck, gotControllerPunch bool
	for i := 0; i < 2; i++ {
		pkt, _, err := recv(controller)
		if err != nil {
			return err
		}
		switch pkt.Type {
		case pppp.MsgP2PReqAck:
			gotAck = len(pkt.Payload) == 4 && pkt.Payload[0] == 0
		case pppp.MsgPunchTo:
			gotControllerPunch = true
		}
	}
	if !gotAck || !gotControllerPunch {
		return fmt.Errorf("controller missing ack/punch ack=%v punch=%v", gotAck, gotControllerPunch)
	}
	devicePunch, _, err := recv(device)
	if err != nil {
		return err
	}
	if devicePunch.Type != pppp.MsgPunchTo {
		return fmt.Errorf("device expected PUNCH_TO, got 0x%02x", devicePunch.Type)
	}
	return nil
}

func send(conn *net.UDPConn, to *net.UDPAddr, pkt pppp.Packet) error {
	b, err := pkt.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = conn.WriteToUDP(b, to)
	return err
}

func recv(conn *net.UDPConn) (pppp.Packet, *net.UDPAddr, error) {
	buf := make([]byte, 4096)
	n, from, err := conn.ReadFromUDP(buf)
	if err != nil {
		return pppp.Packet{}, nil, err
	}
	pkt, err := pppp.ParsePacket(buf[:n])
	return pkt, from, err
}
