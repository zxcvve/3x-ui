## 1. Data Model and API

- [x] 1.1 Add persistence for node outbound bridge selection without changing existing node `outboundTag` semantics.
- [x] 1.2 Extend node request/response validation schemas for bridge mode, selected inbound tags, and generated outbound metadata.
- [x] 1.3 Add backend APIs or extend existing node APIs so the UI can save bridge selections and fetch bridge candidate availability.
- [x] 1.4 Add validation that disabled nodes, missing inbounds, unsupported protocols, and tag collisions are reported explicitly.

## 2. Node Inbound to Outbound Generation

- [x] 2.1 Implement a backend converter from supported node inbound records to Xray outbound objects.
- [x] 2.2 Add protocol-specific conversion for the first supported protocol set, including VLESS, VMess, Trojan, and Shadowsocks where required credentials are present.
- [x] 2.3 Reject unsupported or incomplete inbound configurations with stable unavailable reasons instead of generating partial outbounds.
- [x] 2.4 Implement stable node outbound tag generation and collision checks against manual outbounds, subscription outbounds, balancers, and other node-derived outbounds.

## 3. Xray Config Integration

- [x] 3.1 Merge active node-derived outbounds into generated Xray configs without mutating the saved template.
- [x] 3.2 Expose `nodeOutbounds` and `nodeOutboundTags` from the Xray settings read API.
- [x] 3.3 Ensure Xray apply/restart paths refresh generated node outbounds after node bridge settings, node enablement, or selected inbound changes.
- [x] 3.4 Keep existing panel-to-node API egress through node `outboundTag` working independently of inbound outbound bridging.

## 4. Frontend Workflow

- [x] 4.1 Add bridge controls to the node form so operators can enable outbound bridging and select eligible inbounds.
- [x] 4.2 Show unavailable bridge candidates with protocol, missing-data, disabled-node, or tag-collision reasons.
- [x] 4.3 Add read-only node-derived outbound rows to the Xray Outbounds view with source node and source inbound details.
- [x] 4.4 Include node-derived outbound tags in routing, balancer, routed-profile, node egress, and other existing outbound selectors.

## 5. Verification

- [x] 5.1 Add Go unit tests for tag generation, collision validation, unsupported inbound rejection, and protocol conversion fixtures.
- [x] 5.2 Add Go tests for generated Xray config injection and preservation of existing node `outboundTag` behavior.
- [x] 5.3 Add frontend tests for node bridge selection, read-only generated outbound display, and outbound tag selector inclusion.
- [x] 5.4 Run targeted Go and frontend test suites for node, Xray settings, outbound, routing, and subscription-related paths.
