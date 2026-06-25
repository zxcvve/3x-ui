## Why

Operators can already create multiple clients that share a subscription URL by reusing a `subId`, but the panel presents those clients as unrelated rows. Workflows such as "one person gets direct and relayed profiles in the same subscription" require manual coordination across Clients, Inbounds, Outbounds, and Routing rules.

## What Changes

- Add an operational Subscriptions page to the panel sidebar that groups clients by shared `subId`.
- Show subscription-level details including generated URLs, member clients, attached inbounds, traffic summary, and route destinations.
- Add a guided "routed profile" workflow that creates or updates a client under an existing subscription and configures user-based routing to the selected outbound.
- Keep global subscription server settings under Settings; the new page manages subscriber/profile relationships, not server configuration.
- Preserve compatibility with existing clients that already share a `subId`.

## Capabilities

### New Capabilities
- `subscription-management`: Covers listing subscriptions derived from `subId`, inspecting member clients, copying subscription URLs, and creating routed profiles backed by client rows and Xray user routing.

### Modified Capabilities

## Impact

- Frontend: new panel route and sidebar item, subscription list/detail UI, routed-profile modal, and client/routing data composition.
- Backend: new read APIs for grouped subscriptions and optional command API for routed-profile creation; may reuse existing client CRUD and Xray setting services internally.
- Xray configuration: generated routing rules use the existing `user` matcher with client email values and existing outbound tags.
- Data model: phase one should not require a new table; subscriptions are derived from `clients.sub_id`. A later migration may add a `subscriptions` table for labels and notes.
