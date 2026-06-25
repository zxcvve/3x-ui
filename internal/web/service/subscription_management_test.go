package service

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func seedSubscriptionClient(t *testing.T, c model.ClientRecord, inboundIDs ...int) {
	t.Helper()
	if err := database.GetDB().Create(&c).Error; err != nil {
		t.Fatalf("create client %q: %v", c.Email, err)
	}
	for _, inboundID := range inboundIDs {
		if err := database.GetDB().Create(&model.ClientInbound{ClientId: c.Id, InboundId: inboundID}).Error; err != nil {
			t.Fatalf("attach client %q to inbound %d: %v", c.Email, inboundID, err)
		}
	}
}

func TestSubscriptionServiceListSummariesGroupsSharedSubId(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	ib1 := mkInbound(t, 24001, model.VLESS, `{"clients":[]}`)
	ib1.Remark = "primary"
	ib1.Tag = "in-primary"
	if err := db.Save(ib1).Error; err != nil {
		t.Fatalf("update inbound 1: %v", err)
	}
	ib2 := mkInbound(t, 24002, model.Trojan, `{"clients":[]}`)
	ib2.Remark = "relay"
	ib2.Tag = "in-relay"
	if err := db.Save(ib2).Error; err != nil {
		t.Fatalf("update inbound 2: %v", err)
	}

	seedSubscriptionClient(t, model.ClientRecord{Email: "direct@example.com", SubID: "shared-sub", Enable: true, TotalGB: 100, ExpiryTime: 1700000000000}, ib1.Id)
	seedSubscriptionClient(t, model.ClientRecord{Email: "relay@example.com", SubID: "shared-sub", Enable: false, TotalGB: 200, ExpiryTime: 1800000000000}, ib1.Id, ib2.Id)
	seedSubscriptionClient(t, model.ClientRecord{Email: "empty@example.com", SubID: "", Enable: true}, ib2.Id)
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", "relay@example.com").Update("enable", false).Error; err != nil {
		t.Fatalf("disable relay client: %v", err)
	}

	if err := db.Create(&xray.ClientTraffic{Email: "direct@example.com", Up: 10, Down: 20}).Error; err != nil {
		t.Fatalf("create direct traffic: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: "relay@example.com", Up: 30, Down: 40}).Error; err != nil {
		t.Fatalf("create relay traffic: %v", err)
	}

	got, err := (&SubscriptionService{}).ListSummaries()
	if err != nil {
		t.Fatalf("ListSummaries: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one non-empty subId group, got %d: %#v", len(got), got)
	}
	summary := got[0]
	if summary.SubID != "shared-sub" {
		t.Fatalf("SubID = %q, want shared-sub", summary.SubID)
	}
	if summary.MemberCount != 2 || summary.EnabledCount != 1 || summary.DisabledCount != 1 {
		t.Fatalf("counts = members:%d enabled:%d disabled:%d", summary.MemberCount, summary.EnabledCount, summary.DisabledCount)
	}
	if summary.Traffic.Up != 40 || summary.Traffic.Down != 60 || summary.Traffic.Total != 300 {
		t.Fatalf("traffic = %#v, want up=40 down=60 total=300", summary.Traffic)
	}
	if !summary.TotalGB.Mixed || !summary.ExpiryTime.Mixed {
		t.Fatalf("expected mixed total/expiry flags, got total=%#v expiry=%#v", summary.TotalGB, summary.ExpiryTime)
	}
	if !slices.Equal(summary.InboundIDs, []int{ib1.Id, ib2.Id}) {
		t.Fatalf("InboundIDs = %v, want [%d %d]", summary.InboundIDs, ib1.Id, ib2.Id)
	}
	if !slices.Equal(summary.InboundTags, []string{"in-primary", "in-relay"}) {
		t.Fatalf("InboundTags = %v", summary.InboundTags)
	}
}

func TestSubscriptionServiceGetDetailReturnsUrlsAndMembers(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	if err := db.Create(&model.Setting{Key: "subURI", Value: "https://subs.example.com/sub/"}).Error; err != nil {
		t.Fatalf("set subURI: %v", err)
	}
	if err := db.Create(&model.Setting{Key: "subJsonEnable", Value: "true"}).Error; err != nil {
		t.Fatalf("set subJsonEnable: %v", err)
	}
	if err := db.Create(&model.Setting{Key: "subJsonURI", Value: "https://subs.example.com/json/"}).Error; err != nil {
		t.Fatalf("set subJsonURI: %v", err)
	}
	if err := db.Create(&model.Setting{Key: "subClashEnable", Value: "true"}).Error; err != nil {
		t.Fatalf("set subClashEnable: %v", err)
	}
	if err := db.Create(&model.Setting{Key: "subClashURI", Value: "https://subs.example.com/clash/"}).Error; err != nil {
		t.Fatalf("set subClashURI: %v", err)
	}

	ib := mkInbound(t, 25001, model.VLESS, `{"clients":[]}`)
	ib.Remark = "main inbound"
	ib.Tag = "main-tag"
	if err := db.Save(ib).Error; err != nil {
		t.Fatalf("update inbound: %v", err)
	}
	seedSubscriptionClient(t, model.ClientRecord{Email: "alice@example.com", SubID: "detail-sub", Enable: true, TotalGB: 1024, ExpiryTime: 1700000000000}, ib.Id)
	if err := db.Create(&xray.ClientTraffic{Email: "alice@example.com", Up: 7, Down: 11, LastOnline: 12345}).Error; err != nil {
		t.Fatalf("create traffic: %v", err)
	}

	detail, err := (&SubscriptionService{}).GetDetail("detail-sub", "panel.example.com")
	if err != nil {
		t.Fatalf("GetDetail: %v", err)
	}
	if detail.SubID != "detail-sub" {
		t.Fatalf("SubID = %q", detail.SubID)
	}
	if len(detail.URLs) != 3 {
		t.Fatalf("URLs = %#v, want raw/json/clash", detail.URLs)
	}
	if detail.URLs[0].Format != "raw" || detail.URLs[0].URL != "https://subs.example.com/sub/detail-sub" {
		t.Fatalf("raw URL = %#v", detail.URLs[0])
	}
	if detail.URLs[1].Format != "json" || detail.URLs[1].URL != "https://subs.example.com/json/detail-sub" {
		t.Fatalf("json URL = %#v", detail.URLs[1])
	}
	if detail.URLs[2].Format != "clash" || detail.URLs[2].URL != "https://subs.example.com/clash/detail-sub" {
		t.Fatalf("clash URL = %#v", detail.URLs[2])
	}
	if len(detail.Members) != 1 {
		t.Fatalf("Members = %#v", detail.Members)
	}
	member := detail.Members[0]
	if member.Email != "alice@example.com" || !member.Enable {
		t.Fatalf("member identity/state = %#v", member)
	}
	if member.Traffic.Up != 7 || member.Traffic.Down != 11 || member.Traffic.Total != 1024 || member.Traffic.LastOnline != 12345 {
		t.Fatalf("member traffic = %#v", member.Traffic)
	}
	if !slices.Equal(member.InboundIDs, []int{ib.Id}) || len(member.Inbounds) != 1 || member.Inbounds[0].Tag != "main-tag" {
		t.Fatalf("member inbounds = ids:%v refs:%#v", member.InboundIDs, member.Inbounds)
	}
}

func TestClientServiceCreateAllowsSharedSubIdButRejectsDuplicateEmail(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	ib := mkInbound(t, 26001, model.VLESS, `{"clients":[]}`)

	_, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "first@example.com", SubID: "shared-create", Enable: true},
		InboundIds: []int{ib.Id},
	})
	if err != nil {
		t.Fatalf("create first client: %v", err)
	}
	_, err = svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "second@example.com", SubID: "shared-create", Enable: true},
		InboundIds: []int{ib.Id},
	})
	if err != nil {
		t.Fatalf("create second client with shared subId should succeed: %v", err)
	}
	_, err = svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "first@example.com", SubID: "shared-create", Enable: true},
		InboundIds: []int{ib.Id},
	})
	if err == nil {
		t.Fatalf("duplicate email should still be rejected")
	}
}

func TestClientServiceUpdateAllowsSharedSubIdButRejectsDuplicateEmail(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	ib := mkInbound(t, 27001, model.VLESS, `{"clients":[]}`)

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "first-update@example.com", SubID: "sub-a", ID: "11111111-1111-1111-1111-111111111111", Enable: true},
		InboundIds: []int{ib.Id},
	}); err != nil {
		t.Fatalf("create first: %v", err)
	}
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "second-update@example.com", SubID: "sub-b", ID: "22222222-2222-2222-2222-222222222222", Enable: true},
		InboundIds: []int{ib.Id},
	}); err != nil {
		t.Fatalf("create second: %v", err)
	}

	second, err := svc.GetRecordByEmail(nil, "second-update@example.com")
	if err != nil {
		t.Fatalf("load second: %v", err)
	}
	_, err = svc.Update(inboundSvc, second.Id, model.Client{
		Email:  "second-update@example.com",
		SubID:  "sub-a",
		ID:     "22222222-2222-2222-2222-222222222222",
		Enable: true,
	})
	if err != nil {
		t.Fatalf("update to shared subId should succeed: %v", err)
	}

	_, err = svc.Update(inboundSvc, second.Id, model.Client{
		Email:  "first-update@example.com",
		SubID:  "sub-a",
		ID:     "22222222-2222-2222-2222-222222222222",
		Enable: true,
	})
	if err == nil {
		t.Fatalf("duplicate email should still be rejected")
	}
}

func TestSubscriptionServiceCreateRoutedProfileReusesCompatibleRule(t *testing.T) {
	setupBulkDB(t)
	svc := &SubscriptionService{}
	inboundSvc := &InboundService{}
	ib := mkInbound(t, 28001, model.VLESS, `{"clients":[]}`)
	seedSubscriptionClient(t, model.ClientRecord{Email: "base-route@example.com", SubID: "route-sub", Enable: true}, ib.Id)
	seedSubscriptionXrayTemplate(t, `{
		"outbounds":[{"tag":"direct","protocol":"freedom","settings":{}}],
		"routing":{"rules":[{"type":"field","user":["existing@example.com"],"outboundTag":"direct"}]}
	}`)

	member, _, err := svc.CreateRoutedProfile(inboundSvc, RoutedProfileRequest{
		SubID:       "route-sub",
		Email:       "new-route@example.com",
		InboundIDs:  []int{ib.Id},
		OutboundTag: "direct",
	})
	if err != nil {
		t.Fatalf("CreateRoutedProfile: %v", err)
	}
	if member.Email != "new-route@example.com" || member.RouteTag != "direct" || !slices.Equal(member.InboundIDs, []int{ib.Id}) {
		t.Fatalf("member = %#v", member)
	}

	rules := routedProfileRules(t)
	if len(rules) != 1 {
		t.Fatalf("rules = %#v, want reused single rule", rules)
	}
	users := stringSliceFromAny(rules[0]["user"])
	if !slices.Equal(users, []string{"existing@example.com", "new-route@example.com"}) {
		t.Fatalf("users = %v", users)
	}
}

func TestSubscriptionServiceCreateRoutedProfileCreatesRuleAndRejectsDuplicateEmail(t *testing.T) {
	setupBulkDB(t)
	svc := &SubscriptionService{}
	inboundSvc := &InboundService{}
	ib := mkInbound(t, 29001, model.VLESS, `{"clients":[]}`)
	seedSubscriptionClient(t, model.ClientRecord{Email: "base-new-rule@example.com", SubID: "new-rule-sub", Enable: true}, ib.Id)
	seedSubscriptionXrayTemplate(t, `{
		"outbounds":[{"tag":"proxy-out","protocol":"freedom","settings":{}}],
		"routing":{"rules":[{"type":"field","domain":["example.com"],"outboundTag":"proxy-out"}]}
	}`)

	_, _, err := svc.CreateRoutedProfile(inboundSvc, RoutedProfileRequest{
		SubID:       "new-rule-sub",
		Email:       "profile@example.com",
		InboundIDs:  []int{ib.Id},
		OutboundTag: "proxy-out",
	})
	if err != nil {
		t.Fatalf("CreateRoutedProfile: %v", err)
	}
	rules := routedProfileRules(t)
	if len(rules) != 2 {
		t.Fatalf("rules = %#v, want existing domain rule plus new user rule", rules)
	}
	if users := stringSliceFromAny(rules[1]["user"]); !slices.Equal(users, []string{"profile@example.com"}) {
		t.Fatalf("new rule users = %v", users)
	}

	_, _, err = svc.CreateRoutedProfile(inboundSvc, RoutedProfileRequest{
		SubID:       "new-rule-sub",
		Email:       "profile@example.com",
		InboundIDs:  []int{ib.Id},
		OutboundTag: "proxy-out",
	})
	if err == nil {
		t.Fatalf("duplicate email should be rejected")
	}
	rulesAfterDuplicate := routedProfileRules(t)
	if len(rulesAfterDuplicate) != 2 {
		t.Fatalf("duplicate email must not create another routing rule: %#v", rulesAfterDuplicate)
	}
}

func TestSubscriptionServiceCreateRoutedProfileRejectsUnknownOutbound(t *testing.T) {
	setupBulkDB(t)
	svc := &SubscriptionService{}
	inboundSvc := &InboundService{}
	ib := mkInbound(t, 30001, model.VLESS, `{"clients":[]}`)
	seedSubscriptionClient(t, model.ClientRecord{Email: "base-unknown@example.com", SubID: "unknown-out-sub", Enable: true}, ib.Id)
	seedSubscriptionXrayTemplate(t, `{"outbounds":[{"tag":"known","protocol":"freedom","settings":{}}],"routing":{"rules":[]}}`)

	_, _, err := svc.CreateRoutedProfile(inboundSvc, RoutedProfileRequest{
		SubID:       "unknown-out-sub",
		Email:       "bad-out@example.com",
		InboundIDs:  []int{ib.Id},
		OutboundTag: "missing",
	})
	if err == nil {
		t.Fatalf("unknown outbound tag should be rejected")
	}
}

func seedSubscriptionXrayTemplate(t *testing.T, raw string) {
	t.Helper()
	if err := (&XraySettingService{}).SaveXraySetting(raw); err != nil {
		t.Fatalf("seed xray template: %v", err)
	}
}

func routedProfileRules(t *testing.T) []map[string]any {
	t.Helper()
	raw, err := (&SettingService{}).GetXrayConfigTemplate()
	if err != nil {
		t.Fatalf("get xray template: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("decode template: %v", err)
	}
	rules := routingRulesFromCfg(cfg)
	out := make([]map[string]any, 0, len(rules))
	for _, rule := range rules {
		if rule["outboundTag"] == "api" {
			continue
		}
		out = append(out, rule)
	}
	return out
}

func stringSliceFromAny(raw any) []string {
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
