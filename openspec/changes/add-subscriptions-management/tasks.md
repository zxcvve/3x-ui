## 1. Backend Subscription APIs

- [x] 1.1 Add subscription summary/detail response types for grouped `subId` data.
- [x] 1.2 Implement service queries that group non-empty `clients.sub_id` values and batch-load member clients, traffic rows, and inbound attachments.
- [x] 1.3 Add subscription summary endpoint under the authenticated panel API.
- [x] 1.4 Add subscription detail endpoint that returns generated raw/JSON/Clash URLs when enabled plus member profile rows.
- [x] 1.5 Add backend tests for existing shared `subId`, empty `subId` exclusion, aggregate traffic, and mixed limit/expiry summaries.

## 2. Routed Profile Workflow

- [x] 2.1 Audit client create/update/bulk/import paths that reject duplicate `subId` and adjust them so `subId` may be shared while email remains unique.
- [x] 2.2 Implement a routed-profile service command that creates a client with an existing `subId`, attaches selected inbounds, and validates the requested outbound tag.
- [x] 2.3 Implement routing-rule mutation that appends to a compatible simple `user` rule or creates a new enabled field rule for the selected outbound.
- [x] 2.4 Add an authenticated routed-profile API endpoint for subscription detail actions.
- [x] 2.5 Add tests for routed-profile creation, duplicate email rejection, compatible rule reuse, and new rule creation.

## 3. Frontend Data Layer

- [x] 3.1 Add Zod schemas and TypeScript types for subscription summaries, details, URL formats, member profiles, and routed-profile requests.
- [x] 3.2 Add TanStack Query hooks for subscription list/detail and routed-profile mutation.
- [x] 3.3 Add query invalidation so client, subscription, and Xray routing views refresh after routed-profile creation.

## 4. Frontend UI

- [x] 4.1 Add `/subscriptions` route and sidebar item without moving global subscription settings from Settings.
- [x] 4.2 Build Subscriptions page list with search/filter support, member counts, state, inbounds, route summary, and aggregate usage.
- [x] 4.3 Build subscription detail view with subscription URLs, copy/QR actions, member profile table, and links to related client/routing records.
- [x] 4.4 Build routed-profile modal with email, inbound selection, outbound selection, and validation feedback.
- [x] 4.5 Ensure responsive layout works on mobile and desktop without overflowing table/action controls.

## 5. Verification

- [x] 5.1 Add or update Go tests for backend services/controllers touched by subscription APIs and routing mutation.
- [x] 5.2 Add or update frontend unit/component tests for schemas, hooks, page rendering, and routed-profile modal behavior.
- [x] 5.3 Run targeted Go tests for subscription/client/routing services.
- [x] 5.4 Run targeted frontend tests for clients, routing, and subscriptions.
- [x] 5.5 Run OpenSpec validation/status checks and confirm the change is ready for implementation review.
