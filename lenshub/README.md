# LensHub Vendor-Compatible P2P Fork

This fork is the LensHub adaptation workspace for turning ZLMediaKit into a deployable P2P service candidate for the SXT/LensHub provider-group model.

Upstream base: `ZLMediaKit/ZLMediaKit`

## Current status

This directory is an adaptation layer, not a release-ready PPCS/CS2-compatible server yet.

The current LensHub/SXT deployment model remains:

- SXT allocates and downlinks `provider_type`, `p2p_server_group_id`, `p2p_resource_key`, `p2pid`, server IPs, request URL and provider config.
- Camera and APP consume the downlinked provider data and connect through the selected P2P group.
- Runtime P2P traffic must not depend on SXT callbacks.

## Why this fork exists

ZLMediaKit gives us an actively maintained C++ media/network foundation with WebRTC, STUN/TURN, REST API, hook points and deployable server components. LensHub still needs a compatibility layer for its own provider-resource lifecycle:

- `p2pid` inventory and recyclable resource identity.
- `p2p_server_group_id` as the device-visible group identity.
- `p2p_resource_key + p2pid + p2p_server_group_id` validation.
- provider config and node list compatible with the existing SXT downlink model.
- health, capacity, drain and audit inputs for SXT routing.

## Non-goals for this first branch

- Do not mark SXT resource rows active merely because this branch exists.
- Do not pretend a JSON/TCP probe is PPCS/CS2 protocol compatibility.
- Do not put supplier secrets, SDK keys, tokens, or plaintext credentials in this repository.
- Do not change APP or device downlink fields as part of this bootstrap.

## Bootstrap contents

- `lenshub/compatserver/`: minimal runnable rendezvous/control skeleton for local development.
- `lenshub/config/`: gongshi-test provider-group example matching the prepared SXT test data.
- `lenshub/systemd/`: service template for the compatibility process.
- `lenshub/scripts/`: build and smoke-test helpers.

## Next implementation milestones

1. Replace the skeleton transport with the selected compatible protocol path.
2. Map LensHub `p2pid` and group identity to the chosen runtime registry.
3. Add offline-verifiable service credentials; no runtime SXT callback.
4. Expose health and capacity output for SXT scheduler ingestion.
5. Add protocol-level camera/APP integration tests before changing SXT rows from `disabled/draft` to `active/available`.
