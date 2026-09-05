package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/frankichen/ZLMediaKit/lenshub/compatserver/pppp"
)

type DevicePresence struct {
	DID          string    `json:"did"`
	RemoteAddr   string    `json:"remote_addr"`
	LastSeenUTC  time.Time `json:"last_seen_utc"`
	LoginMessage byte      `json:"login_message"`
	Obfuscated   bool      `json:"obfuscated"`
}

type WireStats struct {
	PacketsReceived uint64 `json:"packets_received"`
	PacketsSent     uint64 `json:"packets_sent"`
	ParseErrors     uint64 `json:"parse_errors"`
	Unsupported     uint64 `json:"unsupported_messages"`
	DeviceLogins    uint64 `json:"device_logins"`
	P2PRequests     uint64 `json:"p2p_requests"`
	PunchPairs      uint64 `json:"punch_pairs"`
}

type Registry struct {
	mu      sync.RWMutex
	devices map[string]DevicePresence
	stats   WireStats
}

func newRegistry() *Registry {
	return &Registry{devices: make(map[string]DevicePresence)}
}

func (r *Registry) register(did pppp.DID, remote *net.UDPAddr, msgType byte, obfuscated bool) DevicePresence {
	p := DevicePresence{DID: did.String(), RemoteAddr: remote.String(), LastSeenUTC: time.Now().UTC(), LoginMessage: msgType, Obfuscated: obfuscated}
	r.mu.Lock()
	r.devices[p.DID] = p
	r.stats.DeviceLogins++
	r.mu.Unlock()
	return p
}

func (r *Registry) lookup(did pppp.DID) (DevicePresence, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.devices[did.String()]
	return p, ok
}

func (r *Registry) snapshot() (int, WireStats) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.devices), r.stats
}

func (r *Registry) noteReceive()      { r.mu.Lock(); r.stats.PacketsReceived++; r.mu.Unlock() }
func (r *Registry) noteSent(n uint64) { r.mu.Lock(); r.stats.PacketsSent += n; r.mu.Unlock() }
func (r *Registry) noteParseError()   { r.mu.Lock(); r.stats.ParseErrors++; r.mu.Unlock() }
func (r *Registry) noteUnsupported()  { r.mu.Lock(); r.stats.Unsupported++; r.mu.Unlock() }
func (r *Registry) noteP2PRequest()   { r.mu.Lock(); r.stats.P2PRequests++; r.mu.Unlock() }
func (r *Registry) notePunchPair()    { r.mu.Lock(); r.stats.PunchPairs++; r.mu.Unlock() }

type outboundDatagram struct {
	to        *net.UDPAddr
	packet    pppp.Packet
	obfuscate bool
}

type decodedDatagram struct {
	packet     pppp.Packet
	obfuscated bool
}

func decodeWireDatagram(data []byte, psk string) (decodedDatagram, error) {
	pkt, err := pppp.ParsePacket(data)
	if err == nil {
		return decodedDatagram{packet: pkt}, nil
	}
	if psk == "" {
		return decodedDatagram{}, err
	}
	clear := pppp.Deobfuscate(psk, data)
	pkt, decErr := pppp.ParsePacket(clear)
	if decErr != nil {
		return decodedDatagram{}, fmt.Errorf("plain parse: %v; obfuscated parse: %w", err, decErr)
	}
	return decodedDatagram{packet: pkt, obfuscated: true}, nil
}

func allowedDID(cfg Config, did pppp.DID) bool {
	if len(cfg.AllowedDIDPrefixes) == 0 {
		return true
	}
	for _, prefix := range cfg.AllowedDIDPrefixes {
		if strings.EqualFold(strings.TrimSpace(prefix), did.Prefix) {
			return true
		}
	}
	return false
}

func didFromPayload(payload []byte) (pppp.DID, error) {
	if len(payload) < pppp.DIDWireSize {
		return pppp.DID{}, fmt.Errorf("DID payload requires at least %d bytes, got %d", pppp.DIDWireSize, len(payload))
	}
	return pppp.ParseDIDWire(payload[:pppp.DIDWireSize])
}

func handleWirePacket(cfg Config, reg *Registry, remote *net.UDPAddr, decoded decodedDatagram) []outboundDatagram {
	pkt := decoded.packet
	obfuscate := decoded.obfuscated
	toRemote := func(typ byte, payload []byte) outboundDatagram {
		return outboundDatagram{to: remote, packet: pppp.Packet{Type: typ, Payload: payload}, obfuscate: obfuscate}
	}

	switch pkt.Type {
	case pppp.MsgHello:
		endpoint, err := pppp.EncodeEndpointIPv4(remote)
		if err != nil {
			return nil
		}
		return []outboundDatagram{toRemote(pppp.MsgHelloAck, endpoint[:])}

	case pppp.MsgDevLogin:
		if !cfg.UnsafeAllowUnverifiedDIDLoginForTest {
			// Until the target CS2 license/CRC/device-login variant is proven,
			// accepting an unauthenticated DID registration would allow presence
			// hijacking. Production-like mode therefore fails closed.
			return []outboundDatagram{toRemote(pppp.MsgDevLoginAck, pppp.StatusPayload(0xFC))}
		}
		did, err := didFromPayload(pkt.Payload)
		if err != nil || !allowedDID(cfg, did) {
			return []outboundDatagram{toRemote(pppp.MsgDevLoginAck, pppp.StatusPayload(0xFC))}
		}
		reg.register(did, remote, pkt.Type, obfuscate)
		return []outboundDatagram{toRemote(pppp.MsgDevLoginAck, pppp.StatusPayload(0))}

	case pppp.MsgP2PReq:
		reg.noteP2PRequest()
		did, err := didFromPayload(pkt.Payload)
		if err != nil || !allowedDID(cfg, did) {
			return []outboundDatagram{toRemote(pppp.MsgP2PReqAck, pppp.StatusPayload(0xFC))}
		}
		presence, ok := reg.lookup(did)
		if !ok || time.Since(presence.LastSeenUTC) > cfg.PresenceTTL() {
			return []outboundDatagram{toRemote(pppp.MsgP2PReqAck, pppp.StatusPayload(0xFC))}
		}
		deviceAddr, err := net.ResolveUDPAddr("udp", presence.RemoteAddr)
		if err != nil {
			return []outboundDatagram{toRemote(pppp.MsgP2PReqAck, pppp.StatusPayload(0xFC))}
		}
		deviceEP, err := pppp.EncodeEndpointIPv4(deviceAddr)
		if err != nil {
			return []outboundDatagram{toRemote(pppp.MsgP2PReqAck, pppp.StatusPayload(0xFC))}
		}
		controllerEP, err := pppp.EncodeEndpointIPv4(remote)
		if err != nil {
			return []outboundDatagram{toRemote(pppp.MsgP2PReqAck, pppp.StatusPayload(0xFC))}
		}
		reg.notePunchPair()
		return []outboundDatagram{
			toRemote(pppp.MsgP2PReqAck, pppp.StatusPayload(0)),
			toRemote(pppp.MsgPunchTo, deviceEP[:]),
			{to: deviceAddr, packet: pppp.Packet{Type: pppp.MsgPunchTo, Payload: controllerEP[:]}, obfuscate: presence.Obfuscated},
		}

	case pppp.MsgListReq:
		// No relay node is advertised until the opaque PPPP relay transport is
		// implemented and validated. A zero-count list keeps the direct path
		// explicit instead of pretending that ZLMediaKit TURN is a PPPP relay.
		return []outboundDatagram{toRemote(pppp.MsgListReqAck, []byte{0, 0, 0, 0})}

	case pppp.MsgAlive:
		return []outboundDatagram{toRemote(pppp.MsgAliveAck, nil)}

	case pppp.MsgPunchPkt, pppp.MsgP2PReady, pppp.MsgDRW, pppp.MsgDRWAck, pppp.MsgClose:
		// These are peer-to-peer session messages. The rendezvous server must
		// not terminate or reinterpret the direct media/data path.
		return nil
	default:
		reg.noteUnsupported()
		return nil
	}
}
