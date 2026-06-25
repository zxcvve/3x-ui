## ADDED Requirements

### Requirement: Subscription groups are visible
The system SHALL provide an authenticated panel view that lists subscriptions grouped by non-empty client `subId`.

#### Scenario: Existing shared subId appears as one subscription
- **WHEN** two or more client records have the same non-empty `subId`
- **THEN** the subscription list shows one subscription row for that `subId`
- **AND** the row shows the number of member clients

#### Scenario: Empty subId clients are excluded
- **WHEN** a client record has an empty `subId`
- **THEN** it is not shown as a subscription group

### Requirement: Subscription summaries include operational state
The system SHALL show subscription-level summary data derived from member clients and their attachments.

#### Scenario: Summary shows member state
- **WHEN** an operator opens the subscription list
- **THEN** each subscription row shows enabled and disabled member counts
- **AND** shows aggregate traffic usage for member clients
- **AND** shows the inbounds used by member clients

#### Scenario: Mixed limits are represented explicitly
- **WHEN** member clients have different traffic limits or expiry times
- **THEN** the summary indicates that limits or expiry are mixed instead of displaying a single misleading limit

### Requirement: Subscription details expose URLs and member profiles
The system SHALL provide a detail view for a selected subscription.

#### Scenario: Detail view shows generated URLs
- **WHEN** an operator opens a subscription detail view
- **THEN** the system shows the available subscription URLs for that `subId` according to enabled subscription formats
- **AND** provides copy and QR actions for available URLs

#### Scenario: Detail view shows member profiles
- **WHEN** an operator opens a subscription detail view
- **THEN** the system lists every client profile with that `subId`
- **AND** shows each profile's email, enabled state, traffic, attached inbounds, and inferred route destination when available

### Requirement: Routed profiles can be created from a subscription
The system SHALL allow an operator to create a routeable profile inside an existing subscription.

#### Scenario: Create routed profile
- **WHEN** an operator selects a subscription and submits a profile email, inbound selection, and outbound tag
- **THEN** the system creates a client profile using the selected subscription's `subId`
- **AND** attaches the profile to the selected inbound or inbounds
- **AND** ensures Xray routing sends that profile email to the selected outbound tag using the `user` matcher

#### Scenario: Duplicate email is rejected
- **WHEN** an operator submits a routed profile using an email that already belongs to another client
- **THEN** the system rejects the request
- **AND** no routing rule is created for the rejected profile

### Requirement: Routed profiles use Xray user routing
The system SHALL represent profile-specific route destinations using Xray field routing rules with `user` values matching client emails.

#### Scenario: Existing compatible route is reused
- **WHEN** a simple enabled field rule already routes users to the requested outbound tag
- **THEN** the system may add the profile email to that rule's `user` list
- **AND** the resulting rule remains visible and editable in the existing Routing UI

#### Scenario: No compatible route exists
- **WHEN** no compatible route exists for the requested outbound tag
- **THEN** the system creates an enabled field rule with the profile email in `user` and the requested outbound tag

### Requirement: Global subscription settings remain separate
The system SHALL keep global subscription server configuration under Settings and use the Subscriptions page only for operational subscription/profile management.

#### Scenario: Operator changes subscription server settings
- **WHEN** an operator needs to change subscription domain, paths, format enablement, or templates
- **THEN** those controls remain available under Settings
- **AND** the Subscriptions page links to or reflects those settings without duplicating them as editable controls
