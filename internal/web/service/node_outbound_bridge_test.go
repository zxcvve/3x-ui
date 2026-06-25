package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func bridgeNode() *model.Node {
	return &model.Node{
		Id:                   7,
		Name:                 "de-node",
		Address:              "node.example.com",
		Enable:               true,
		OutboundBridgeEnable: true,
		OutboundBridgeTags:   []string{"remote-vless"},
		OutboundTag:          "panel-api-egress",
	}
}

func bridgeInbound(protocol model.Protocol, tag, settings string) *model.Inbound {
	return &model.Inbound{
		Id:             12,
		NodeID:         bridgeIntPtr(7),
		Enable:         true,
		Port:           443,
		Protocol:       protocol,
		Tag:            tag,
		Settings:       settings,
		StreamSettings: `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"node.example.com"}}`,
	}
}

func bridgeIntPtr(v int) *int { return &v }

func TestNodeOutboundBridgeCandidates(t *testing.T) {
	t.Run("builds a stable outbound from a selected VLESS inbound", func(t *testing.T) {
		in := bridgeInbound(model.VLESS, "remote-vless", `{"clients":[{"id":"11111111-1111-1111-1111-111111111111","flow":"xtls-rprx-vision"}]}`)
		candidates := BuildNodeOutboundCandidates([]*model.Node{bridgeNode()}, []*model.Inbound{in}, NodeOutboundCollisionContext{})
		if len(candidates) != 1 {
			t.Fatalf("expected one candidate, got %d", len(candidates))
		}
		got := candidates[0]
		if !got.Available || got.UnavailableReason != "" {
			t.Fatalf("candidate should be available, got %+v", got)
		}
		if got.Tag != "node-7-remote-vless" {
			t.Fatalf("tag = %q, want node-7-remote-vless", got.Tag)
		}
		if got.NodeID != 7 || got.SourceInboundID != 12 || got.SourceInboundTag != "remote-vless" {
			t.Fatalf("source metadata not preserved: %+v", got)
		}
		out := got.Outbound
		if out["tag"] != "node-7-remote-vless" || out["protocol"] != "vless" {
			t.Fatalf("unexpected outbound identity: %+v", out)
		}
		settings := out["settings"].(map[string]any)
		vnext := settings["vnext"].([]any)[0].(map[string]any)
		if vnext["address"] != "node.example.com" || vnext["port"] != float64(443) {
			t.Fatalf("unexpected target: %+v", vnext)
		}
		users := vnext["users"].([]any)
		if users[0].(map[string]any)["id"] != "11111111-1111-1111-1111-111111111111" {
			t.Fatalf("credential not copied: %+v", users)
		}
	})

	t.Run("disabled node reports a stable unavailable reason", func(t *testing.T) {
		n := bridgeNode()
		n.Enable = false
		in := bridgeInbound(model.VLESS, "remote-vless", `{"clients":[{"id":"11111111-1111-1111-1111-111111111111"}]}`)
		got := BuildNodeOutboundCandidates([]*model.Node{n}, []*model.Inbound{in}, NodeOutboundCollisionContext{})[0]
		if got.Available || got.UnavailableReason != NodeOutboundUnavailableNodeDisabled {
			t.Fatalf("got %+v, want disabled-node reason", got)
		}
	})

	t.Run("unsupported protocol is unavailable instead of partially generated", func(t *testing.T) {
		n := bridgeNode()
		n.OutboundBridgeTags = []string{"remote-http"}
		in := bridgeInbound(model.HTTP, "remote-http", `{}`)
		got := BuildNodeOutboundCandidates([]*model.Node{n}, []*model.Inbound{in}, NodeOutboundCollisionContext{})[0]
		if got.Available || got.UnavailableReason != NodeOutboundUnavailableUnsupportedProtocol || got.Outbound != nil {
			t.Fatalf("got %+v, want unsupported without outbound", got)
		}
	})

	t.Run("tag collision blocks generation without renaming", func(t *testing.T) {
		in := bridgeInbound(model.VLESS, "remote-vless", `{"clients":[{"id":"11111111-1111-1111-1111-111111111111"}]}`)
		ctx := NodeOutboundCollisionContext{
			ManualOutboundTags: []string{"node-7-remote-vless"},
		}
		got := BuildNodeOutboundCandidates([]*model.Node{bridgeNode()}, []*model.Inbound{in}, ctx)[0]
		if got.Available || got.UnavailableReason != NodeOutboundUnavailableTagCollision || got.Tag != "node-7-remote-vless" {
			t.Fatalf("got %+v, want collision on stable tag", got)
		}
	})

	t.Run("selected remote tag resolves an imported inbound with the node prefix", func(t *testing.T) {
		in := bridgeInbound(model.VLESS, "n7-remote-vless", `{"clients":[{"id":"11111111-1111-1111-1111-111111111111"}]}`)
		candidates := BuildNodeOutboundCandidates([]*model.Node{bridgeNode()}, []*model.Inbound{in}, NodeOutboundCollisionContext{})
		if len(candidates) != 1 {
			t.Fatalf("expected one candidate, got %d", len(candidates))
		}
		got := candidates[0]
		if !got.Available || got.UnavailableReason != "" {
			t.Fatalf("candidate should be available, got %+v", got)
		}
		if got.SourceInboundID != 12 || got.SourceInboundTag != "remote-vless" {
			t.Fatalf("source metadata not preserved: %+v", got)
		}
		if got.Tag != "node-7-remote-vless" {
			t.Fatalf("tag = %q, want node-7-remote-vless", got.Tag)
		}
	})

	t.Run("custom share address is used as the dial target", func(t *testing.T) {
		n := bridgeNode()
		n.Address = "panel-only.example.com"
		in := bridgeInbound(model.VLESS, "remote-vless", `{"clients":[{"id":"11111111-1111-1111-1111-111111111111"}]}`)
		in.ShareAddrStrategy = "custom"
		in.ShareAddr = "traffic.example.com"
		got := BuildNodeOutboundCandidates([]*model.Node{n}, []*model.Inbound{in}, NodeOutboundCollisionContext{})[0]
		if !got.Available {
			t.Fatalf("candidate should be available, got %+v", got)
		}
		settings := got.Outbound["settings"].(map[string]any)
		vnext := settings["vnext"].([]any)[0].(map[string]any)
		if vnext["address"] != "traffic.example.com" {
			t.Fatalf("address = %v, want custom share address", vnext["address"])
		}
	})

	t.Run("reality stream settings are converted to client-side outbound settings", func(t *testing.T) {
		stream := `{
			"network":"tcp",
			"security":"reality",
			"tcpSettings":{"header":{"type":"none"}},
			"realitySettings":{
				"serverNames":["reality.example.com"],
				"shortIds":["ab12cd"],
				"privateKey":"server-private",
				"target":"www.example.com:443",
				"settings":{"publicKey":"PBKvalue","fingerprint":"firefox","spiderX":"/spider","mldsa65Verify":"PQV"}
			}
		}`
		in := bridgeInbound(model.VLESS, "remote-vless", `{"clients":[{"id":"11111111-1111-1111-1111-111111111111","flow":"xtls-rprx-vision"}]}`)
		in.StreamSettings = stream
		got := BuildNodeOutboundCandidates([]*model.Node{bridgeNode()}, []*model.Inbound{in}, NodeOutboundCollisionContext{})[0]
		if !got.Available {
			t.Fatalf("candidate should be available, got %+v", got)
		}
		outStream := got.Outbound["streamSettings"].(map[string]any)
		reality := outStream["realitySettings"].(map[string]any)
		if reality["publicKey"] != "PBKvalue" || reality["serverName"] != "reality.example.com" || reality["shortId"] != "ab12cd" {
			t.Fatalf("reality outbound settings = %+v", reality)
		}
		if _, ok := reality["privateKey"]; ok {
			t.Fatalf("client-side reality settings must not include server privateKey: %+v", reality)
		}
		if _, ok := reality["settings"]; ok {
			t.Fatalf("client-side reality settings must flatten nested settings: %+v", reality)
		}
	})
}

func TestMergeNodeOutbounds(t *testing.T) {
	cfg := &xray.Config{
		OutboundConfigs: json_util.RawMessage(`[{"tag":"direct","protocol":"freedom"}]`),
	}
	candidates := []NodeOutboundCandidate{
		{Available: true, Tag: "node-7-remote-vless", Outbound: map[string]any{"tag": "node-7-remote-vless", "protocol": "vless", "settings": map[string]any{}}},
		{Available: false, Tag: "node-7-bad", Outbound: map[string]any{"tag": "node-7-bad", "protocol": "vless"}},
	}
	if err := MergeNodeOutbounds(cfg, candidates); err != nil {
		t.Fatalf("MergeNodeOutbounds: %v", err)
	}
	var outbounds []map[string]any
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatal(err)
	}
	if len(outbounds) != 2 {
		t.Fatalf("expected manual + one available node outbound, got %+v", outbounds)
	}
	if outbounds[0]["tag"] != "direct" || outbounds[1]["tag"] != "node-7-remote-vless" {
		t.Fatalf("unexpected merge order: %+v", outbounds)
	}
}

func TestGetXrayConfigInjectsNodeOutboundsWithoutMutatingTemplate(t *testing.T) {
	setupSettingTestDB(t)
	template := `{"log":{},"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}],"routing":{"rules":[]}}`
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", template); err != nil {
		t.Fatalf("save template: %v", err)
	}
	node := bridgeNode()
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	inbound := bridgeInbound(model.VLESS, "remote-vless", `{"clients":[{"id":"11111111-1111-1111-1111-111111111111"}]}`)
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("create node inbound: %v", err)
	}

	cfg, err := (&XrayService{}).GetXrayConfig()
	if err != nil {
		t.Fatalf("GetXrayConfig: %v", err)
	}
	var outbounds []map[string]any
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatal(err)
	}
	if !hasOutboundTag(outbounds, "direct") || !hasOutboundTag(outbounds, "node-7-remote-vless") {
		t.Fatalf("generated config missing expected outbounds: %+v", outbounds)
	}
	stored, err := (&SettingService{}).GetXrayConfigTemplate()
	if err != nil {
		t.Fatalf("read stored template: %v", err)
	}
	if !equalJSON(t, stored, template) {
		t.Fatalf("stored template was mutated\nwant: %s\ngot:  %s", template, stored)
	}
}

func TestGetXrayConfigKeepsNodeOutboundTagSeparateFromBridgeSelection(t *testing.T) {
	setupSettingTestDB(t)
	template := `{"log":{},"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"},{"tag":"warp","protocol":"freedom"}],"routing":{"rules":[]}}`
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", template); err != nil {
		t.Fatalf("save template: %v", err)
	}
	node := bridgeNode()
	node.OutboundBridgeEnable = false
	node.OutboundBridgeTags = nil
	node.OutboundTag = "warp"
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	cfg, err := (&XrayService{}).GetXrayConfig()
	if err != nil {
		t.Fatalf("GetXrayConfig: %v", err)
	}
	var outbounds []map[string]any
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatal(err)
	}
	if hasOutboundTag(outbounds, "node-7-remote-vless") {
		t.Fatalf("node outboundTag alone must not export remote inbounds: %+v", outbounds)
	}
	if !hasNodeEgressInbound(cfg.InboundConfigs, 7) {
		t.Fatalf("node outboundTag should still inject panel-to-node egress inbound, got %+v", cfg.InboundConfigs)
	}
	if !hasNodeEgressRule(t, cfg.RouterConfig, 7, "warp") {
		t.Fatalf("node outboundTag should still inject egress routing rule, got %s", cfg.RouterConfig)
	}
}

func hasOutboundTag(outbounds []map[string]any, tag string) bool {
	for _, outbound := range outbounds {
		if outbound["tag"] == tag {
			return true
		}
	}
	return false
}

func hasNodeEgressInbound(inbounds []xray.InboundConfig, nodeID int) bool {
	want := NodeEgressInboundTag(nodeID)
	for _, inbound := range inbounds {
		if inbound.Tag == want && inbound.Protocol == "socks" {
			return true
		}
	}
	return false
}

func hasNodeEgressRule(t *testing.T, raw json_util.RawMessage, nodeID int, outboundTag string) bool {
	t.Helper()
	var routing struct {
		Rules []struct {
			InboundTag  []string `json:"inboundTag"`
			OutboundTag string   `json:"outboundTag"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(raw, &routing); err != nil {
		t.Fatalf("unmarshal routing: %v", err)
	}
	wantInbound := NodeEgressInboundTag(nodeID)
	for _, rule := range routing.Rules {
		if rule.OutboundTag != outboundTag {
			continue
		}
		for _, inboundTag := range rule.InboundTag {
			if inboundTag == wantInbound {
				return true
			}
		}
	}
	return false
}
