# LensHub PPPP compatibility research notices

This LensHub compatibility layer was implemented from public protocol documentation and independently written tests.

## magicus/pppp-dissector

- Project: `https://github.com/magicus/pppp-dissector`
- License: MIT
- Use in this fork: protocol opcode/framing documentation and the 256-byte legacy PPPP shuffle table used by the optional obfuscation codec.
- The upstream repository identifies Magnus Ihse Bursie (`magicus`) as its author/maintainer. Preserve the upstream MIT license when redistributing material derived from it.

## elastic/camera-hacks

- Project: `https://github.com/elastic/camera-hacks`
- License: MIT
- Use in this fork: behavioral reference for the publicly documented `HELLO -> P2P_REQ -> PUNCH_TO -> PUNCH_PKT -> P2P_RDY` sequence and a published legacy endpoint byte example.
- No exploit commands or camera application commands are copied into the LensHub server.

## devbis/aiopppp

- Project: `https://github.com/devbis/aiopppp`
- License: Apache-2.0
- Use in this fork: independent cross-check of F1 packet framing, 20-byte DID representation, DRW framing and public protocol terminology.

## Licensing boundary

Some other PPPP reverse-engineering projects are GPL-licensed. They may be consulted only as external documentation where legally appropriate; their implementation code must not be copied into this fork without a separate licensing decision.
