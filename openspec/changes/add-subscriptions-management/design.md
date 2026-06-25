## Context

The panel already stores a subscription identifier on each client (`clients.sub_id`) and the subscription server resolves `/sub/{subId}` by finding enabled inbounds attached to clients with that value. Some installations already contain multiple client rows with the same `subId`, which is a useful model for one subscriber receiving multiple profiles in one subscription.

The current UI is client-centered. Operators must manually reuse `subId`, create separate client identities, attach inbounds, create outbounds, and add Xray routing rules using the `user` matcher. The Xray routing UI already supports `user`, and `user` maps to the client email. The missing piece is an operational view that makes shared-subscription relationships visible and provides a guided workflow.

## Goals / Non-Goals

**Goals:**
- Add a first-class Subscriptions page in the panel sidebar.
- Represent subscriptions as grouped client records by non-empty `subId`.
- Surface subscription URLs, member profiles, attached inbounds, route destinations, status, and aggregate usage.
- Provide a guided routed-profile workflow that creates or updates a client under a selected subscription and creates or updates a user-based routing rule for the selected outbound.
- Preserve existing installations where multiple clients already share a `subId`.

**Non-Goals:**
- Do not replace Settings > Subscription, which remains the global subscription server configuration area.
- Do not introduce a required `subscriptions` table in the first implementation.
- Do not change Xray's routing semantics or invent a new per-subscription matcher.
- Do not make two independent inbounds bind the same public port; routing simplification is handled by user rules or fallback designs outside this feature.

## Decisions

### Derive subscriptions from `clients.sub_id`

Phase one treats a subscription as an implicit group of client rows with the same non-empty `subId`.

Alternative considered: add a `subscriptions` table immediately. That would improve naming and metadata, but it would require migration and reconciliation rules before the basic workflow is useful. Deriving from existing data is compatible with current installs and keeps the first change focused.

### Add read APIs for subscription groups

Introduce backend endpoints that return grouped subscription summaries and details instead of making the frontend fetch every client page and group locally. The summary should include `subId`, client count, enabled/disabled counts, aggregate traffic, expiry summary, inferred inbounds, and generated subscription URLs when subscription settings are enabled.

Alternative considered: build the page entirely from existing `/clients/list/paged`. That endpoint is paginated and filter-oriented, so it is a poor source for complete grouping and aggregate traffic.

### Implement routed profile as a workflow over existing services

The routed-profile action should call existing client creation/update/attach code, then add or update a routing rule:

```json
{
  "type": "field",
  "user": ["alice-relay@example.com"],
  "outboundTag": "relay-node-out"
}
```

If a compatible rule for the same outbound already exists, the workflow can append the email to that rule's `user` list. If no compatible rule exists, it creates a new enabled rule. Rules created by this workflow should remain editable in the existing Routing UI.

Alternative considered: store route destination directly on the client row. That would create a second source of truth and still require generating Xray routing rules. Keeping routing in Xray settings preserves existing behavior.

### Keep profile identity email-based

The workflow must create distinct client emails for distinct routeable profiles. Xray `user` routing and runtime traffic/IP accounting are keyed by email, so duplicate emails cannot represent different route destinations.

Alternative considered: distinguish profiles by `subId` or display name. Xray does not route by `subId`, and display names are not runtime identities.

### Use a dedicated route and sidebar item

Add `/subscriptions` as a panel route with its own sidebar item. Global subscription settings remain under `/settings#subscription`.

Alternative considered: add another tab to Clients. That would hide the workflow inside an already dense table and would not give subscriptions equal operational weight.

## Risks / Trade-offs

- Existing duplicate `subId` validation may still block some UI/API flows -> audit create, update, bulk create, import, and routed-profile code paths so shared `subId` is allowed while email remains unique.
- Routing rule mutation can surprise operators if rules are appended incorrectly -> only append to simple enabled field rules with the same `outboundTag` and no non-user matchers; otherwise create a new rule.
- Aggregate traffic can be misleading when member profiles have different limits or expiry times -> show aggregate usage plus per-profile rows, and use explicit labels for mixed/unlimited limits.
- Large client tables can make grouping expensive -> implement server-side grouped queries and batch traffic/inbound lookups.
- Subscription labels are limited without a table -> use `subId` as the stable identifier in phase one and leave room for optional metadata later.

## Migration Plan

No required database migration for phase one. Existing clients with shared `subId` appear automatically on the Subscriptions page.

Rollback is straightforward: removing the UI and APIs does not alter the underlying client/subscription model. Routed-profile-created clients and routing rules are ordinary existing records and remain manageable through Clients and Routing.

## Open Questions

- Should the routed-profile workflow append users to an existing simple route by default, or always create one rule per profile for easier auditing?
- Should phase one include editable subscription display names stored in client comments/group, or defer labels until a real `subscriptions` table exists?
- Which subscription formats should be shown in the detail view when raw, JSON, and Clash are enabled?
