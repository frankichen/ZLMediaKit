# LensHub Vendor-Compatible P2P Fork

This fork is the LensHub adaptation workspace for turning ZLMediaKit into a deployable P2P service candidate for the SXT/LensHub provider-group model.

Upstream base: `ZLMediaKit/ZLMediaKit`

## Contract

The LensHub/SXT boundary is unchanged:

- SXT allocates and downlinks `provider_type`, `p2p_server_group_id`, `p2p_resource_key`, `p2pid`, server IPs, request URL and provider-owned configuration.
- Camera and APP consume those fields using the existing provider path. This fork must not require a new SXT downlink shape.
- Runtime P2P registration, lookup, NAT traversal and relay are handled by the standalone P2P group. Runtime packets must not call SXT for authorization or discovery.
- `device_id` remains the permanent LensHub identity. A PPPP DID/P2PID is a recyclable communication resource.

## B1 implementation status

The previous JSON-only UDP skeleton has been replaced by a clean-room PPPP/CS2 wire-protocol foundation on the UDP rendezvous port.

Implemented and locally tested:

- F1 packet framing: `magic + opcode + big-endian payload length + payload`.
- Canonical four-byte-prefix DID support, including the current `PPCS-xxxxxx-xxxxx` test family.
- 20-byte DID wire encoding/decoding used by legacy PPPP control packets.
- Legacy 16-byte IPv4 endpoint encoding used by `MSG_HELLO_ACK` / `MSG_PUNCH_TO`.
- Optional CS2-style packet obfuscation with the PSK supplied only through an environment variable.
- `MSG_HELLO -> MSG_HELLO_ACK` using the observed source endpoint.
- `MSG_DEV_LGN -> MSG_DEV_LGN_ACK` registration using the observed source endpoint; caller-supplied address bytes are not routing authority.
- `MSG_P2P_REQ -> MSG_P2P_REQ_ACK + MSG_PUNCH_TO` coordination to both peers.
- `MSG_LST_REQ -> empty MSG_LST_REQ_ACK` while PPPP relay is intentionally disabled.
- `MSG_ALIVE -> MSG_ALIVE_ACK`.
- TTL-based presence expiry and fail-closed unknown-device behavior.
- HTTP health/readiness and counters.
- TCP 12306/12308 are reserved for real CS2 TCP/DSLK fallback and are intentionally left unbound in B1.
- Development diagnostics bind only to `127.0.0.1:18181`.
- Unverified `MSG_DEV_LGN` registration fails closed by default; the synthetic gongshi-test smoke must opt in with `unsafe_allow_unverified_did_login_for_test=true`.

The deterministic smoke path simulates a device and controller and validates:

```text
HELLO
-> HELLO_ACK
DEV_LGN(PPCS DID)
-> DEV_LGN_ACK
P2P_REQ(PPCS DID)
-> P2P_REQ_ACK
-> PUNCH_TO(controller -> device endpoint)
-> PUNCH_TO(device -> controller endpoint)
```

## Deliberate compatibility gates

This branch is **not yet release-ready PPCS/CS2 compatibility**. It reports:

```text
pppp_f1_rendezvous_foundation_not_vendor_validated
```

The following still require real APP/camera traffic or a confirmed SDK contract before implementation can be called compatible:

1. Which `MSG_DEV_LGN*` variant the target firmware actually sends (`DEV_LGN`, CRC, KEY, DSK, or vendor extension).
2. The exact target `MSG_P2P_REQ` payload variant and any DSK/session fields.
3. The target init-string decoding and server-list semantics.
4. CRC/key/license checks used by this particular `PPCS` network.
5. CS2 TCP fallback framing on 12306 and DSLK behavior on 12308; B1 does not occupy these ports.
6. PPPP relay server login/allocation/data forwarding.
7. Real vendor DID generation/verification. The 100 SXT rows currently prepared in `gongshi-test` are disabled placeholders and are not wire-valid inventory.
8. Packet-level validation against the current production APP SDK and real camera firmware.

Until those gates pass, SXT provider region/group/node/inventory rows must remain `disabled/draft/drain`.

## Source and clean-room policy

The wire implementation uses public protocol research as behavioral reference, without importing third-party client/server source wholesale. Important references and licenses are recorded in `lenshub/THIRD_PARTY_NOTICES.md`.

Do not copy GPL implementations into this fork. Do not commit supplier SDK binaries, vendor keys, init secrets, CRC keys, P2P keys, wake-up keys, tokens, device passwords or captured private credentials.

## Layout

- `lenshub/compatserver/pppp/`: clean-room F1/DID/endpoint/obfuscation codecs.
- `lenshub/compatserver/rendezvous.go`: UDP directory/rendezvous state machine.
- `lenshub/compatserver/cmd/ppppprobe/`: deterministic device/controller protocol probe.
- `lenshub/config/`: `gongshi-test` provider-group example and activation gates.
- `lenshub/systemd/`: service template.
- `lenshub/scripts/`: build and smoke helpers.

## Next batch

B2 should be driven by a packet capture from a test camera and current APP configured to the test server. It will add only the `DEV_LGN/P2P_REQ` variants actually observed, then implement PPPP relay/TCP fallback if the direct path requires it. ZLMediaKit remains available as the media/network foundation for server-mediated media integration; it is not presented as native PPCS protocol support.
