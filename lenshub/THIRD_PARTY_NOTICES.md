# LensHub PPPP compatibility research notices

This LensHub compatibility layer was implemented from public protocol documentation and independently written tests.

## magicus/pppp-dissector

- Project: `https://github.com/magicus/pppp-dissector`
- License: MIT
- Use in this fork: protocol opcode/framing documentation and the 256-byte legacy PPPP shuffle table used by the optional obfuscation codec.
- The upstream repository identifies Magnus Ihse Bursie (`magicus`) as its author/maintainer. The copied shuffle table retains the upstream MIT notice below.

### MIT notice for `magicus/pppp-dissector` material

Copyright 2023 Magnus Ihse Bursie <mag@icus.se>

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

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
