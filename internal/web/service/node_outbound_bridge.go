package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const (
	NodeOutboundUnavailableNodeDisabled        = "node_disabled"
	NodeOutboundUnavailableMissingInbound      = "missing_inbound"
	NodeOutboundUnavailableInboundDisabled     = "inbound_disabled"
	NodeOutboundUnavailableUnsupportedProtocol = "unsupported_protocol"
	NodeOutboundUnavailableMissingCredentials  = "missing_credentials"
	NodeOutboundUnavailableTagCollision        = "tag_collision"
)

type NodeOutboundCollisionContext struct {
	ManualOutboundTags       []string
	SubscriptionOutboundTags []string
	BalancerTags             []string
}

type NodeOutboundCandidate struct {
	NodeID            int            `json:"nodeId"`
	NodeName          string         `json:"nodeName"`
	SourceInboundID   int            `json:"sourceInboundId,omitempty"`
	SourceInboundTag  string         `json:"sourceInboundTag"`
	Protocol          string         `json:"protocol,omitempty"`
	Tag               string         `json:"tag"`
	Available         bool           `json:"available"`
	UnavailableReason string         `json:"unavailableReason,omitempty"`
	Outbound          map[string]any `json:"outbound,omitempty"`
}

func (s *NodeService) OutboundBridgeCandidates() ([]NodeOutboundCandidate, error) {
	template, err := (&SettingService{}).GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}
	var cfg xray.Config
	if err := json.Unmarshal([]byte(template), &cfg); err != nil {
		return nil, err
	}
	subTags, _ := (&OutboundSubscriptionService{}).AllActiveOutboundTags()
	ctx := NodeOutboundCollisionContext{
		ManualOutboundTags:       outboundTagsFromRaw(cfg.OutboundConfigs),
		SubscriptionOutboundTags: subTags,
		BalancerTags:             balancerTagsFromRaw(cfg.RouterConfig),
	}
	return s.outboundBridgeCandidates(ctx)
}

func (s *NodeService) OutboundBridgeCandidatesForConfig(cfg *xray.Config) ([]NodeOutboundCandidate, error) {
	ctx := NodeOutboundCollisionContext{
		ManualOutboundTags: outboundTagsFromRaw(cfg.OutboundConfigs),
		BalancerTags:       balancerTagsFromRaw(cfg.RouterConfig),
	}
	return s.outboundBridgeCandidates(ctx)
}

func (s *NodeService) OutboundBridgeCandidatesForNode(id int) ([]NodeOutboundCandidate, error) {
	candidates, err := s.OutboundBridgeCandidates()
	if err != nil {
		return nil, err
	}
	out := make([]NodeOutboundCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.NodeID == id {
			out = append(out, candidate)
		}
	}
	return out, nil
}

func (s *NodeService) outboundBridgeCandidates(ctx NodeOutboundCollisionContext) ([]NodeOutboundCandidate, error) {
	db := database.GetDB()
	var nodes []*model.Node
	if err := db.Model(&model.Node{}).Order("id asc").Find(&nodes).Error; err != nil {
		return nil, err
	}
	var inbounds []*model.Inbound
	if err := db.Model(&model.Inbound{}).Where("node_id IS NOT NULL").Order("id asc").Find(&inbounds).Error; err != nil {
		return nil, err
	}
	return BuildNodeOutboundCandidates(nodes, inbounds, ctx), nil
}

func (s *NodeService) AllActiveNodeOutboundTags() ([]string, error) {
	candidates, err := s.OutboundBridgeCandidates()
	if err != nil {
		return nil, err
	}
	tags := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Available && candidate.Tag != "" {
			tags = append(tags, candidate.Tag)
		}
	}
	return tags, nil
}

func (s *NodeService) AllActiveNodeOutbounds() ([]NodeOutboundCandidate, error) {
	candidates, err := s.OutboundBridgeCandidates()
	if err != nil {
		return nil, err
	}
	out := make([]NodeOutboundCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Available {
			out = append(out, candidate)
		}
	}
	return out, nil
}

func BuildNodeOutboundCandidates(nodes []*model.Node, inbounds []*model.Inbound, ctx NodeOutboundCollisionContext) []NodeOutboundCandidate {
	inboundsByNodeTag := make(map[int]map[string]*model.Inbound)
	for _, inbound := range inbounds {
		if inbound == nil || inbound.NodeID == nil {
			continue
		}
		byTag := inboundsByNodeTag[*inbound.NodeID]
		if byTag == nil {
			byTag = make(map[string]*model.Inbound)
			inboundsByNodeTag[*inbound.NodeID] = byTag
		}
		byTag[inbound.Tag] = inbound
		prefix := nodeTagPrefix(inbound.NodeID)
		if prefix != "" {
			if stripped, found := strings.CutPrefix(inbound.Tag, prefix); found {
				byTag[stripped] = inbound
			} else {
				byTag[prefix+inbound.Tag] = inbound
			}
		}
	}

	collisions := make(map[string]struct{})
	for _, tag := range ctx.ManualOutboundTags {
		if tag != "" {
			collisions[tag] = struct{}{}
		}
	}
	for _, tag := range ctx.SubscriptionOutboundTags {
		if tag != "" {
			collisions[tag] = struct{}{}
		}
	}
	for _, tag := range ctx.BalancerTags {
		if tag != "" {
			collisions[tag] = struct{}{}
		}
	}

	var candidates []NodeOutboundCandidate
	generatedTags := make(map[string]struct{})
	for _, node := range nodes {
		if node == nil || !node.OutboundBridgeEnable {
			continue
		}
		for _, inboundTag := range node.OutboundBridgeTags {
			inboundTag = strings.TrimSpace(inboundTag)
			if inboundTag == "" {
				continue
			}
			tag := nodeOutboundTag(node.Id, inboundTag)
			candidate := NodeOutboundCandidate{
				NodeID:           node.Id,
				NodeName:         node.Name,
				SourceInboundTag: inboundTag,
				Tag:              tag,
			}
			inbound := inboundsByNodeTag[node.Id][inboundTag]
			switch {
			case !node.Enable:
				candidate.UnavailableReason = NodeOutboundUnavailableNodeDisabled
			case inbound == nil:
				candidate.UnavailableReason = NodeOutboundUnavailableMissingInbound
			case !inbound.Enable:
				candidate.SourceInboundID = inbound.Id
				candidate.Protocol = string(inbound.Protocol)
				candidate.UnavailableReason = NodeOutboundUnavailableInboundDisabled
			default:
				candidate.SourceInboundID = inbound.Id
				candidate.Protocol = string(inbound.Protocol)
				outbound, reason := nodeInboundToOutbound(node, inbound, tag)
				if reason != "" {
					candidate.UnavailableReason = reason
				} else {
					candidate.Outbound = outbound
					candidate.Available = true
				}
			}
			if _, exists := collisions[tag]; exists {
				candidate.Available = false
				candidate.Outbound = nil
				candidate.UnavailableReason = NodeOutboundUnavailableTagCollision
			}
			if _, exists := generatedTags[tag]; exists {
				candidate.Available = false
				candidate.Outbound = nil
				candidate.UnavailableReason = NodeOutboundUnavailableTagCollision
			}
			generatedTags[tag] = struct{}{}
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func MergeNodeOutbounds(cfg *xray.Config, candidates []NodeOutboundCandidate) error {
	var outbounds []any
	if len(cfg.OutboundConfigs) > 0 {
		if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
			return err
		}
	}
	for _, candidate := range candidates {
		if candidate.Available && candidate.Outbound != nil {
			outbounds = append(outbounds, candidate.Outbound)
		}
	}
	raw, err := json.Marshal(outbounds)
	if err != nil {
		return err
	}
	cfg.OutboundConfigs = json_util.RawMessage(raw)
	return nil
}

func outboundTagsFromRaw(raw json_util.RawMessage) []string {
	var outbounds []any
	if len(raw) == 0 || json.Unmarshal(raw, &outbounds) != nil {
		return nil
	}
	tags := make([]string, 0, len(outbounds))
	for _, outbound := range outbounds {
		m, ok := outbound.(map[string]any)
		if !ok {
			continue
		}
		if tag, _ := m["tag"].(string); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func balancerTagsFromRaw(raw json_util.RawMessage) []string {
	var routing map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &routing) != nil {
		return nil
	}
	balancers, _ := routing["balancers"].([]any)
	tags := make([]string, 0, len(balancers))
	for _, balancer := range balancers {
		m, ok := balancer.(map[string]any)
		if !ok {
			continue
		}
		if tag, _ := m["tag"].(string); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

var nodeOutboundTagUnsafe = regexp.MustCompile(`[^a-z0-9-]+`)

func nodeOutboundTag(nodeID int, inboundTag string) string {
	slug := strings.ToLower(strings.TrimSpace(inboundTag))
	slug = strings.ReplaceAll(slug, "_", "-")
	slug = nodeOutboundTagUnsafe.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "inbound"
	}
	return fmt.Sprintf("node-%d-%s", nodeID, slug)
}

func nodeInboundToOutbound(node *model.Node, inbound *model.Inbound, tag string) (map[string]any, string) {
	settings := map[string]any{}
	if strings.TrimSpace(inbound.Settings) != "" {
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			return nil, NodeOutboundUnavailableMissingCredentials
		}
	}
	stream := map[string]any{}
	if strings.TrimSpace(inbound.StreamSettings) != "" {
		if err := json.Unmarshal([]byte(inbound.StreamSettings), &stream); err != nil {
			return nil, NodeOutboundUnavailableMissingCredentials
		}
	}

	base := map[string]any{
		"tag":      tag,
		"protocol": string(inbound.Protocol),
	}
	if len(stream) > 0 {
		base["streamSettings"] = outboundStreamSettings(stream)
	}
	address := nodeOutboundAddress(node, inbound)
	switch inbound.Protocol {
	case model.VLESS:
		client, ok := firstClient(settings)
		id, _ := client["id"].(string)
		if !ok || strings.TrimSpace(id) == "" {
			return nil, NodeOutboundUnavailableMissingCredentials
		}
		user := map[string]any{"id": id, "encryption": "none"}
		if flow, _ := client["flow"].(string); flow != "" {
			user["flow"] = flow
		}
		base["settings"] = map[string]any{"vnext": []any{serverWithUsers(address, inbound.Port, []any{user})}}
	case model.VMESS:
		client, ok := firstClient(settings)
		id, _ := client["id"].(string)
		if !ok || strings.TrimSpace(id) == "" {
			return nil, NodeOutboundUnavailableMissingCredentials
		}
		user := map[string]any{"id": id, "alterId": 0}
		if security, _ := client["security"].(string); security != "" {
			user["security"] = security
		} else {
			user["security"] = "auto"
		}
		base["settings"] = map[string]any{"vnext": []any{serverWithUsers(address, inbound.Port, []any{user})}}
	case model.Trojan:
		client, ok := firstClient(settings)
		password, _ := client["password"].(string)
		if !ok || strings.TrimSpace(password) == "" {
			return nil, NodeOutboundUnavailableMissingCredentials
		}
		server := map[string]any{"address": address, "port": float64(inbound.Port), "password": password}
		if flow, _ := client["flow"].(string); flow != "" {
			server["flow"] = flow
		}
		base["settings"] = map[string]any{"servers": []any{server}}
	case model.Shadowsocks:
		method, _ := settings["method"].(string)
		password, _ := settings["password"].(string)
		if password == "" {
			if client, ok := firstClient(settings); ok {
				password, _ = client["password"].(string)
				if m, _ := client["method"].(string); m != "" {
					method = m
				}
			}
		}
		if strings.TrimSpace(method) == "" || strings.TrimSpace(password) == "" {
			return nil, NodeOutboundUnavailableMissingCredentials
		}
		base["settings"] = map[string]any{"servers": []any{map[string]any{
			"address":  address,
			"port":     float64(inbound.Port),
			"method":   method,
			"password": password,
		}}}
	default:
		return nil, NodeOutboundUnavailableUnsupportedProtocol
	}
	return base, ""
}

func nodeOutboundAddress(node *model.Node, inbound *model.Inbound) string {
	nodeAddr := ""
	if node != nil {
		nodeAddr = strings.TrimSpace(node.Address)
	}
	listenAddr := shareableListenAddress(inbound)
	customAddr := strings.TrimSpace(inbound.ShareAddr)
	switch inbound.ShareAddrStrategy {
	case "listen":
		return firstNonEmpty(listenAddr, nodeAddr)
	case "custom":
		return firstNonEmpty(customAddr, nodeAddr, listenAddr)
	default:
		return firstNonEmpty(nodeAddr, listenAddr)
	}
}

func shareableListenAddress(inbound *model.Inbound) string {
	listen := strings.TrimSpace(inbound.Listen)
	if listen == "" || strings.HasPrefix(listen, "@") || strings.HasPrefix(listen, "/") {
		return ""
	}
	if listen == "0.0.0.0" || listen == "::" || listen == "[::]" || strings.EqualFold(listen, "localhost") ||
		strings.HasPrefix(listen, "127.") || listen == "::1" || listen == "[::1]" {
		return ""
	}
	return strings.Trim(listen, "[]")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func outboundStreamSettings(stream map[string]any) map[string]any {
	out := cloneStringAnyMap(stream)
	delete(out, "externalProxy")
	if tls, ok := out["tlsSettings"].(map[string]any); ok {
		out["tlsSettings"] = outboundTLSSettings(tls)
	}
	if reality, ok := out["realitySettings"].(map[string]any); ok {
		out["realitySettings"] = outboundRealitySettings(reality)
	}
	return out
}

func outboundTLSSettings(tls map[string]any) map[string]any {
	out := cloneStringAnyMap(tls)
	if settings, ok := tls["settings"].(map[string]any); ok {
		copyStringField(out, settings, "fingerprint")
		copyStringField(out, settings, "echConfigList")
		copyStringField(out, settings, "verifyPeerCertByName")
		if pins, ok := settings["pinnedPeerCertSha256"]; ok {
			out["pinnedPeerCertSha256"] = pins
		}
	}
	delete(out, "settings")
	delete(out, "certificates")
	delete(out, "rejectUnknownSni")
	delete(out, "disableSystemRoot")
	delete(out, "echServerKeys")
	delete(out, "echSockopt")
	return out
}

func outboundRealitySettings(reality map[string]any) map[string]any {
	out := map[string]any{}
	settings, _ := reality["settings"].(map[string]any)
	copyStringField(out, settings, "publicKey")
	copyStringField(out, settings, "fingerprint")
	copyStringField(out, settings, "spiderX")
	copyStringField(out, settings, "mldsa65Verify")
	if serverName, _ := settings["serverName"].(string); strings.TrimSpace(serverName) != "" {
		out["serverName"] = serverName
	} else if names := stringListFromAny(reality["serverNames"]); len(names) > 0 {
		out["serverName"] = names[0]
	}
	if shortIDs := stringListFromAny(reality["shortIds"]); len(shortIDs) > 0 {
		out["shortId"] = shortIDs[0]
	}
	return out
}

func cloneStringAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func copyStringField(dst map[string]any, src map[string]any, key string) {
	if value, _ := src[key].(string); strings.TrimSpace(value) != "" {
		dst[key] = value
	}
}

func stringListFromAny(raw any) []string {
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func serverWithUsers(address string, port int, users []any) map[string]any {
	return map[string]any{"address": address, "port": float64(port), "users": users}
}

func firstClient(settings map[string]any) (map[string]any, bool) {
	rawClients, ok := settings["clients"].([]any)
	if !ok || len(rawClients) == 0 {
		return nil, false
	}
	client, ok := rawClients[0].(map[string]any)
	return client, ok
}
