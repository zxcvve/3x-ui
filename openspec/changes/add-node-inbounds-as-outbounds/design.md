## Context

The panel already supports remote nodes and syncs selected node inbounds into the main database for operational visibility. It also has a separate node `outboundTag` setting that routes the main panel's API calls to a node through an existing outbound by injecting a loopback SOCKS inbound and routing rule. That setting solves panel-to-node connectivity, not user traffic relay through a node.

Outbound subscription support is the closest existing pattern for this change: generated outbounds are persisted or derived outside the editable Xray template, exposed as read-only rows, merged into the runtime Xray config, and surfaced as tags for routing and balancers.

## Goals / Non-Goals

**Goals:**
- Let operators enable selected remote node inbounds as routeable outbounds on the main node.
- Generate stable outbound tags that can be used by routing rules, balancers, client routed profiles, node egress, and other existing outbound selectors.
- Keep generated node outbounds read-only and separate from manually edited template outbounds.
- Support only protocols and transports that can be faithfully converted from an inbound definition to a client outbound.
- Provide enough status and validation to prevent silent broken routes when a node, inbound, tag, or protocol is unavailable.

**Non-Goals:**
- Do not replace node inbound sync or node provisioning.
- Do not change Xray routing semantics or create a new node-specific routing matcher.
- Do not export every node inbound automatically by default.
- Do not expose panel API tokens or reuse panel management credentials as proxy credentials.
- Do not support protocols that do not have a meaningful client outbound equivalent in Xray.

## Decisions

### Generate node outbounds outside the editable template

Node-derived outbounds will be generated during Xray config composition and returned by the Xray settings read API as a separate read-only collection, alongside subscription outbounds. The saved `xrayTemplateConfig` remains the source of truth for manual outbounds.

Alternative considered: write generated outbounds into the editable template. That would make tag cleanup, node deletion, and inbound refreshes mutate user-managed config and create difficult rollback behavior. Runtime generation matches subscription outbounds and keeps ownership clear.

### Store export intent on the node/inbound relationship

The selection model should live near node inbound sync state. A node can expose no inbounds, all synced inbounds, or an explicit list of inbound tags as outbound candidates. Each exported candidate should carry enough metadata for a stable tag prefix and optional display name.

Alternative considered: create a separate outbound subscription-like table for node exports. That could work later for richer labels and per-candidate overrides, but it duplicates node/inbound selection rules before the basic workflow is proven.

### Convert only bridgeable inbound protocols

The backend will implement a converter that takes a node, an inbound row, and its client/stream settings and emits an Xray outbound object. Initial bridgeable protocols should include VLESS, VMess, Trojan, Shadowsocks, and other protocols already supported by outbound schemas when their inbound data contains the necessary client credential. Unsupported protocols such as TUN, Tunnel/Dokodemo transparent forwarding, MTProto sidecars, and inbound-only modes must be listed as unavailable with a reason instead of generating invalid outbounds.

Alternative considered: generate SOCKS/HTTP outbounds to every node regardless of inbound type. That would require each node to run a dedicated relay inbound and would not use the existing user-facing inbounds the operator asked to route through.

### Use stable, collision-safe tags

Generated tags should use a deterministic prefix such as `node-<node-id>-` plus a slug of the source inbound tag, with collision checks against manual outbounds, subscription outbounds, balancers, and other node-derived outbounds. If a configured tag collides, the system must reject or mark that candidate unavailable rather than silently changing a route target.

Alternative considered: auto-suffix on collisions. Suffixing avoids immediate errors but can break existing routing rules after a node rename, inbound rename, or refresh. Explicit validation is safer for route targets.

### Surface node-derived tags everywhere existing generated outbounds are usable

The Xray config response should include `nodeOutbounds` and `nodeOutboundTags`. Existing hooks that collect outbound tags should add these tags in the same category as concrete outbounds, while balancers remain grouped separately. The Outbounds tab should show node-derived outbounds in a read-only section with test actions where possible.

Alternative considered: keep node outbounds visible only in the Nodes page. That hides them from the routing workflows where operators need to select them and makes troubleshooting harder.

### Reuse existing Xray reload and hot-apply paths

Changing node outbound export settings affects generated outbounds and routing target availability, so the backend should apply the resulting Xray config using the same save/restart or hot-apply mechanism already used for outbound subscription changes and Xray settings. Node deletion or disabling must remove the generated outbounds from the runtime config on the next apply.

Alternative considered: defer all changes until a manual Xray restart. That is operationally surprising because the UI would show route targets that are not present in the running config.

## Risks / Trade-offs

- Inbound-to-outbound conversion may miss protocol-specific fields -> implement converter tests from real inbound fixtures and reject incomplete candidates with clear reasons.
- Generated tags are route targets, so accidental tag changes can break routing -> derive tags from stable node id plus source inbound tag and validate collisions explicitly.
- Node availability does not guarantee the remote inbound is reachable from the main node -> keep probe/test actions and show node/inbound status, but do not treat status as proof of end-user reachability.
- Selected inbounds can disappear on the remote node -> keep stale selections visible as unavailable until the operator removes or refreshes them.
- Exporting credentials from inbound settings can expose sensitive data in the main node config -> never include panel API tokens, redact read APIs where appropriate, and treat generated Xray config as sensitive like the existing template.

## Migration Plan

No existing node exports are enabled by default. Existing nodes continue to behave as they do today, including the current `outboundTag` setting for panel-to-node API egress.

After deployment, operators can opt in per node or per selected inbound. Rolling back removes the UI/API and generated config injection; manually created routing rules that reference generated node tags will remain in the template but will no longer match an outbound until the operator removes or retargets them.

## Open Questions

- Should phase one allow one generated outbound per inbound only, or also one generated outbound per selected client credential inside a multi-client inbound?
- Should generated tags be configurable per candidate, or fixed from `node-<id>-<inbound-tag>` until a later metadata table exists?
- Which exact inbound protocols should ship in the first converter set beyond VLESS, VMess, Trojan, and Shadowsocks?
