## Why

Operators can manage remote nodes from the main panel, but routing traffic through a node still requires manually creating compatible outbounds in the main Xray template. This makes multi-node relay topologies fragile because node inbound changes are not reflected as routeable destinations on the main node.

## What Changes

- Add a way to expose selected inbounds from enabled remote nodes as generated outbounds in the main node's Xray configuration.
- Surface node-derived outbound tags in routing, balancer, node egress, and other outbound selectors without making them editable as manual template outbounds.
- Keep generated tags stable across refreshes while preventing collisions with manual and subscription-derived outbounds.
- Show node-derived outbounds as read-only operational rows with source node, source inbound, protocol, address, and availability state.
- Preserve the existing node `outboundTag` behavior that routes panel-to-node API traffic through a selected outbound.

## Capabilities

### New Capabilities
- `node-outbound-bridging`: Covers generating routeable main-node outbounds from selected remote node inbounds and making those generated outbounds available to routing and balancers.

### Modified Capabilities

## Impact

- Backend: node service/controller APIs for selecting bridgeable node inbounds, generated outbound composition, tag stability, collision validation, and Xray config injection.
- Frontend: Nodes and Xray Outbounds UI for enabling bridge export, previewing generated node outbounds, and including node-derived tags in existing outbound selectors.
- Xray configuration: generated outbounds are injected into the runtime config, similar to outbound subscription outbounds, while the saved template remains manually editable.
- Security and networking: remote node connection details and credentials must be converted into client-side outbound objects carefully, without exposing panel API tokens or accepting unsupported inbound protocols.
