## ADDED Requirements

### Requirement: Node inbounds can be selected for outbound bridging
The system SHALL allow an authenticated operator to enable outbound bridging for selected inbounds on an enabled remote node.

#### Scenario: Operator selects node inbounds
- **WHEN** an operator edits a node and selects one or more bridgeable remote inbound tags for outbound bridging
- **THEN** the system persists the selection with the node configuration
- **AND** the selected inbounds are candidates for generated main-node outbounds

#### Scenario: Disabled node does not export outbounds
- **WHEN** a node is disabled
- **THEN** the system does not include that node's bridged inbounds in the generated main-node outbounds

### Requirement: Node-derived outbounds are generated from bridgeable inbounds
The system SHALL generate Xray outbound objects from selected node inbounds when the inbound protocol and settings can be represented as an outbound.

#### Scenario: Bridgeable inbound produces outbound
- **WHEN** a selected node inbound has a supported protocol, required client credentials, and valid stream settings
- **THEN** the generated Xray configuration includes an outbound that dials the node inbound
- **AND** the outbound uses the selected inbound's protocol-compatible client settings

#### Scenario: Unsupported inbound is reported unavailable
- **WHEN** a selected node inbound uses a protocol or settings shape that cannot be converted to an outbound
- **THEN** the system does not generate an outbound for that inbound
- **AND** the UI or API reports the inbound as unavailable with a reason

### Requirement: Node-derived outbound tags are stable and unique
The system SHALL assign stable, unique tags to generated node outbounds and reject collisions with existing route targets.

#### Scenario: Stable tag is reused
- **WHEN** the same node inbound remains selected across Xray config refreshes
- **THEN** the generated outbound keeps the same tag

#### Scenario: Tag collision blocks generation
- **WHEN** a generated node outbound tag conflicts with a manual outbound, subscription outbound, balancer, or another node-derived outbound
- **THEN** the system does not silently rename the generated outbound
- **AND** the system reports a validation error or unavailable candidate for the conflict

### Requirement: Node-derived outbounds are visible as read-only generated outbounds
The system SHALL expose generated node outbounds separately from manually editable template outbounds.

#### Scenario: Xray settings response includes node outbounds
- **WHEN** the frontend loads Xray settings
- **THEN** the response includes generated node outbounds and their tags separately from the editable `xraySetting.outbounds`

#### Scenario: Outbounds view shows node source
- **WHEN** generated node outbounds exist
- **THEN** the Outbounds view shows them as read-only generated outbounds
- **AND** each row identifies the source node and source inbound

### Requirement: Node-derived tags can be used by routing and balancers
The system SHALL include node-derived outbound tags in existing outbound target selectors.

#### Scenario: Routing rule selects node outbound
- **WHEN** an operator creates or edits an Xray routing rule
- **THEN** node-derived outbound tags are available as concrete outbound targets

#### Scenario: Balancer selects node outbound
- **WHEN** an operator creates or edits a balancer
- **THEN** node-derived outbound tags are available as selectable balancer members

### Requirement: Existing node egress behavior is preserved
The system SHALL keep the current node `outboundTag` behavior separate from node inbound outbound bridging.

#### Scenario: Node API traffic uses configured outbound
- **WHEN** a node has `outboundTag` configured for panel-to-node connectivity
- **THEN** the panel continues to route node API calls through that outbound as before
- **AND** this setting does not by itself export any remote inbounds as routeable user-traffic outbounds
