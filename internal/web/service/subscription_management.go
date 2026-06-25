package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

type SubscriptionService struct{}

type SubscriptionMixedInt64 struct {
	Value int64 `json:"value"`
	Mixed bool  `json:"mixed"`
}

type SubscriptionSummary struct {
	SubID         string                   `json:"subId"`
	MemberCount   int                      `json:"memberCount"`
	EnabledCount  int                      `json:"enabledCount"`
	DisabledCount int                      `json:"disabledCount"`
	Traffic       xray.ClientTraffic       `json:"traffic"`
	TotalGB       SubscriptionMixedInt64   `json:"totalGB"`
	ExpiryTime    SubscriptionMixedInt64   `json:"expiryTime"`
	InboundIDs    []int                    `json:"inboundIds"`
	InboundTags   []string                 `json:"inboundTags"`
	Inbounds      []SubscriptionInboundRef `json:"inbounds"`
}

type SubscriptionInboundRef struct {
	ID     int    `json:"id"`
	Tag    string `json:"tag"`
	Remark string `json:"remark"`
}

type SubscriptionDetail struct {
	SubscriptionSummary
	URLs    []SubscriptionURL    `json:"urls"`
	Members []SubscriptionMember `json:"members"`
}

type SubscriptionURL struct {
	Format string `json:"format"`
	URL    string `json:"url"`
}

type SubscriptionMember struct {
	ID         int                      `json:"id"`
	Email      string                   `json:"email"`
	Enable     bool                     `json:"enable"`
	TotalGB    int64                    `json:"totalGB"`
	ExpiryTime int64                    `json:"expiryTime"`
	InboundIDs []int                    `json:"inboundIds"`
	Inbounds   []SubscriptionInboundRef `json:"inbounds"`
	Traffic    xray.ClientTraffic       `json:"traffic"`
	RouteTag   string                   `json:"routeTag,omitempty"`
}

type RoutedProfileRequest struct {
	SubID       string `json:"subId"`
	Email       string `json:"email"`
	InboundIDs  []int  `json:"inboundIds"`
	OutboundTag string `json:"outboundTag"`
}

// ListSummaries groups clients by non-empty subId and derives operational
// counters without requiring clients to be present in inbound settings JSON.
func (s *SubscriptionService) ListSummaries() ([]SubscriptionSummary, error) {
	db := database.GetDB()
	var clients []model.ClientRecord
	if err := db.Where("sub_id <> ?", "").Order("sub_id ASC, id ASC").Find(&clients).Error; err != nil {
		return nil, err
	}
	if len(clients) == 0 {
		return []SubscriptionSummary{}, nil
	}

	clientIDs := make([]int, 0, len(clients))
	emails := make([]string, 0, len(clients))
	byID := make(map[int]model.ClientRecord, len(clients))
	for _, c := range clients {
		clientIDs = append(clientIDs, c.Id)
		emails = append(emails, c.Email)
		byID[c.Id] = c
	}

	trafficByEmail, err := subscriptionTrafficByEmail(db, emails)
	if err != nil {
		return nil, err
	}
	inboundsByClient, inboundRefs, err := subscriptionInboundsByClient(db, clientIDs)
	if err != nil {
		return nil, err
	}

	groups := make(map[string]*SubscriptionSummary)
	for _, c := range clients {
		summary := groups[c.SubID]
		if summary == nil {
			summary = &SubscriptionSummary{
				SubID:       c.SubID,
				InboundIDs:  []int{},
				InboundTags: []string{},
				Inbounds:    []SubscriptionInboundRef{},
			}
			groups[c.SubID] = summary
		}
		summary.MemberCount++
		if c.Enable {
			summary.EnabledCount++
		} else {
			summary.DisabledCount++
		}
		if t, ok := trafficByEmail[c.Email]; ok {
			summary.Traffic.Up += t.Up
			summary.Traffic.Down += t.Down
			if t.LastOnline > summary.Traffic.LastOnline {
				summary.Traffic.LastOnline = t.LastOnline
			}
		}
		summary.Traffic.Total += c.TotalGB
		summary.TotalGB = foldMixedInt64(summary.TotalGB, summary.MemberCount, c.TotalGB)
		summary.ExpiryTime = foldMixedInt64(summary.ExpiryTime, summary.MemberCount, c.ExpiryTime)
		for _, inboundID := range inboundsByClient[c.Id] {
			if ref, ok := inboundRefs[inboundID]; ok {
				appendInboundRef(summary, ref)
			}
		}
	}

	out := make([]SubscriptionSummary, 0, len(groups))
	for _, summary := range groups {
		out = append(out, *summary)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SubID < out[j].SubID
	})
	return out, nil
}

func (s *SubscriptionService) GetDetail(subID, host string) (*SubscriptionDetail, error) {
	subID = strings.TrimSpace(subID)
	if subID == "" {
		return nil, common.NewError("subscription subId is required")
	}
	db := database.GetDB()
	var clients []model.ClientRecord
	if err := db.Where("sub_id = ?", subID).Order("id ASC").Find(&clients).Error; err != nil {
		return nil, err
	}
	if len(clients) == 0 {
		return nil, common.NewError("subscription not found:", subID)
	}

	clientIDs := make([]int, 0, len(clients))
	emails := make([]string, 0, len(clients))
	for _, c := range clients {
		clientIDs = append(clientIDs, c.Id)
		emails = append(emails, c.Email)
	}
	trafficByEmail, err := subscriptionTrafficByEmail(db, emails)
	if err != nil {
		return nil, err
	}
	inboundsByClient, inboundRefs, err := subscriptionInboundsByClient(db, clientIDs)
	if err != nil {
		return nil, err
	}

	detail := &SubscriptionDetail{
		SubscriptionSummary: SubscriptionSummary{
			SubID:       subID,
			InboundIDs:  []int{},
			InboundTags: []string{},
			Inbounds:    []SubscriptionInboundRef{},
		},
		Members: make([]SubscriptionMember, 0, len(clients)),
	}
	for _, c := range clients {
		detail.MemberCount++
		if c.Enable {
			detail.EnabledCount++
		} else {
			detail.DisabledCount++
		}
		detail.TotalGB = foldMixedInt64(detail.TotalGB, detail.MemberCount, c.TotalGB)
		detail.ExpiryTime = foldMixedInt64(detail.ExpiryTime, detail.MemberCount, c.ExpiryTime)

		member := SubscriptionMember{
			ID:         c.Id,
			Email:      c.Email,
			Enable:     c.Enable,
			TotalGB:    c.TotalGB,
			ExpiryTime: c.ExpiryTime,
			InboundIDs: []int{},
			Inbounds:   []SubscriptionInboundRef{},
		}
		if t, ok := trafficByEmail[c.Email]; ok {
			member.Traffic = t
			detail.Traffic.Up += t.Up
			detail.Traffic.Down += t.Down
			if t.LastOnline > detail.Traffic.LastOnline {
				detail.Traffic.LastOnline = t.LastOnline
			}
		}
		member.Traffic.Total = c.TotalGB
		member.Traffic.ExpiryTime = c.ExpiryTime
		detail.Traffic.Total += c.TotalGB

		for _, inboundID := range inboundsByClient[c.Id] {
			if ref, ok := inboundRefs[inboundID]; ok {
				appendInboundRef(&detail.SubscriptionSummary, ref)
				member.InboundIDs = append(member.InboundIDs, ref.ID)
				member.Inbounds = append(member.Inbounds, ref)
			}
		}
		detail.Members = append(detail.Members, member)
	}
	detail.URLs = subscriptionURLsForSubID(&SettingService{}, subID, host)
	return detail, nil
}

func (s *SubscriptionService) CreateRoutedProfile(inboundSvc *InboundService, req RoutedProfileRequest) (*SubscriptionMember, bool, error) {
	req.SubID = strings.TrimSpace(req.SubID)
	req.Email = strings.TrimSpace(req.Email)
	req.OutboundTag = strings.TrimSpace(req.OutboundTag)
	if req.SubID == "" {
		return nil, false, common.NewError("subscription subId is required")
	}
	if req.Email == "" {
		return nil, false, common.NewError("client email is required")
	}
	if len(req.InboundIDs) == 0 {
		return nil, false, common.NewError("at least one inbound is required")
	}
	if req.OutboundTag == "" {
		return nil, false, common.NewError("outbound tag is required")
	}
	if err := validateRoutedProfileTarget(req.SubID, req.OutboundTag); err != nil {
		return nil, false, err
	}

	clientSvc := &ClientService{}
	needRestart, err := clientSvc.Create(inboundSvc, &ClientCreatePayload{
		Client: model.Client{
			Email:  req.Email,
			SubID:  req.SubID,
			Enable: true,
		},
		InboundIds: req.InboundIDs,
	})
	if err != nil {
		return nil, needRestart, err
	}
	if err := appendRoutedProfileRule(req.Email, req.OutboundTag); err != nil {
		return nil, needRestart, err
	}

	rec, err := clientSvc.GetRecordByEmail(nil, req.Email)
	if err != nil {
		return nil, needRestart, err
	}
	_, inboundRefs, err := subscriptionInboundsByClient(database.GetDB(), []int{rec.Id})
	if err != nil {
		return nil, needRestart, err
	}
	ids, err := clientSvc.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		return nil, needRestart, err
	}
	member := &SubscriptionMember{
		ID:         rec.Id,
		Email:      rec.Email,
		Enable:     rec.Enable,
		TotalGB:    rec.TotalGB,
		ExpiryTime: rec.ExpiryTime,
		InboundIDs: ids,
		Inbounds:   make([]SubscriptionInboundRef, 0, len(ids)),
		RouteTag:   req.OutboundTag,
	}
	for _, id := range ids {
		if ref, ok := inboundRefs[id]; ok {
			member.Inbounds = append(member.Inbounds, ref)
		}
	}
	return member, needRestart, nil
}

func validateRoutedProfileTarget(subID, outboundTag string) error {
	var count int64
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("sub_id = ?", subID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return common.NewError("subscription not found:", subID)
	}
	template, err := (&SettingService{}).GetXrayConfigTemplate()
	if err != nil {
		return err
	}
	cfg, err := decodeXrayTemplateMap(template)
	if err != nil {
		return err
	}
	for _, out := range outboundsFromCfg(cfg) {
		m, ok := out.(map[string]any)
		if !ok {
			continue
		}
		if tag, _ := m["tag"].(string); tag == outboundTag {
			return nil
		}
	}
	nodeTags, err := (&NodeService{}).AllActiveNodeOutboundTags()
	if err != nil {
		return err
	}
	for _, tag := range nodeTags {
		if tag == outboundTag {
			return nil
		}
	}
	return common.NewError("unknown outbound tag:", outboundTag)
}

func appendRoutedProfileRule(email, outboundTag string) error {
	settingSvc := &SettingService{}
	xraySvc := &XraySettingService{}
	template, err := settingSvc.GetXrayConfigTemplate()
	if err != nil {
		return err
	}
	updated, changed, err := mutateXrayTemplateRouting(template, func(cfg map[string]any) bool {
		rules := routingRulesFromCfg(cfg)
		for _, rule := range rules {
			if !isCompatibleUserRule(rule, outboundTag) {
				continue
			}
			users := stringSliceFromRoutingValue(rule["user"])
			for _, existing := range users {
				if existing == email {
					return false
				}
			}
			users = append(users, email)
			rule["user"] = users
			setRoutingRulesInCfg(cfg, rules)
			return true
		}
		rules = append(rules, map[string]any{
			"type":        "field",
			"enabled":     true,
			"user":        []string{email},
			"outboundTag": outboundTag,
		})
		setRoutingRulesInCfg(cfg, rules)
		return true
	})
	if err != nil || !changed {
		return err
	}
	return xraySvc.SaveXraySetting(updated)
}

func decodeXrayTemplateMap(raw string) (map[string]any, error) {
	var cfg map[string]any
	if err := json.Unmarshal([]byte(UnwrapXrayTemplateConfig(raw)), &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func isCompatibleUserRule(rule map[string]any, outboundTag string) bool {
	if typ, _ := rule["type"].(string); typ != "" && typ != "field" {
		return false
	}
	if enabled, ok := rule["enabled"].(bool); ok && !enabled {
		return false
	}
	if tag, _ := rule["outboundTag"].(string); tag != outboundTag {
		return false
	}
	for _, key := range routingMatcherKeys {
		if key == "user" {
			continue
		}
		if hasRoutingMatcherValue(rule[key]) {
			return false
		}
	}
	return true
}

func stringSliceFromRoutingValue(raw any) []string {
	switch v := raw.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	default:
		return nil
	}
}

func foldMixedInt64(current SubscriptionMixedInt64, count int, next int64) SubscriptionMixedInt64 {
	if count == 1 {
		return SubscriptionMixedInt64{Value: next}
	}
	if current.Value != next {
		current.Mixed = true
	}
	return current
}

func appendInboundRef(summary *SubscriptionSummary, ref SubscriptionInboundRef) {
	i := sort.SearchInts(summary.InboundIDs, ref.ID)
	if i < len(summary.InboundIDs) && summary.InboundIDs[i] == ref.ID {
		return
	}
	summary.InboundIDs = append(summary.InboundIDs, 0)
	copy(summary.InboundIDs[i+1:], summary.InboundIDs[i:])
	summary.InboundIDs[i] = ref.ID
	summary.InboundTags = append(summary.InboundTags, "")
	copy(summary.InboundTags[i+1:], summary.InboundTags[i:])
	summary.InboundTags[i] = ref.Tag
	summary.Inbounds = append(summary.Inbounds, SubscriptionInboundRef{})
	copy(summary.Inbounds[i+1:], summary.Inbounds[i:])
	summary.Inbounds[i] = ref
}

func subscriptionTrafficByEmail(db *gorm.DB, emails []string) (map[string]xray.ClientTraffic, error) {
	out := make(map[string]xray.ClientTraffic, len(emails))
	if len(emails) == 0 {
		return out, nil
	}
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var rows []xray.ClientTraffic
		if err := db.Where("email IN ?", batch).Find(&rows).Error; err != nil {
			return nil, err
		}
		overlayGlobalTrafficValues(db, rows)
		for _, row := range rows {
			out[row.Email] = row
		}
	}
	return out, nil
}

func subscriptionInboundsByClient(db *gorm.DB, clientIDs []int) (map[int][]int, map[int]SubscriptionInboundRef, error) {
	byClient := make(map[int][]int, len(clientIDs))
	refs := map[int]SubscriptionInboundRef{}
	if len(clientIDs) == 0 {
		return byClient, refs, nil
	}
	var links []model.ClientInbound
	for _, batch := range chunkInts(clientIDs, sqlInChunk) {
		var batchLinks []model.ClientInbound
		if err := db.Where("client_id IN ?", batch).Find(&batchLinks).Error; err != nil {
			return nil, nil, err
		}
		links = append(links, batchLinks...)
	}
	inboundIDs := make([]int, 0, len(links))
	seen := map[int]struct{}{}
	for _, link := range links {
		byClient[link.ClientId] = append(byClient[link.ClientId], link.InboundId)
		if _, ok := seen[link.InboundId]; ok {
			continue
		}
		seen[link.InboundId] = struct{}{}
		inboundIDs = append(inboundIDs, link.InboundId)
	}
	sort.Ints(inboundIDs)
	for _, batch := range chunkInts(inboundIDs, sqlInChunk) {
		var inbounds []model.Inbound
		if err := db.Select("id, tag, remark").Where("id IN ?", batch).Find(&inbounds).Error; err != nil {
			return nil, nil, err
		}
		for _, ib := range inbounds {
			refs[ib.Id] = SubscriptionInboundRef{ID: ib.Id, Tag: ib.Tag, Remark: ib.Remark}
		}
	}
	for clientID := range byClient {
		sort.Ints(byClient[clientID])
	}
	return byClient, refs, nil
}

func subscriptionURLsForSubID(settingSvc *SettingService, subID, host string) []SubscriptionURL {
	if enabled, err := settingSvc.GetSubEnable(); err != nil || !enabled {
		return []SubscriptionURL{}
	}
	urls := make([]SubscriptionURL, 0, 3)
	if raw := subscriptionURL(settingSvc, subID, host, "raw"); raw != "" {
		urls = append(urls, SubscriptionURL{Format: "raw", URL: raw})
	}
	if enabled, err := settingSvc.GetSubJsonEnable(); err == nil && enabled {
		if raw := subscriptionURL(settingSvc, subID, host, "json"); raw != "" {
			urls = append(urls, SubscriptionURL{Format: "json", URL: raw})
		}
	}
	if enabled, err := settingSvc.GetSubClashEnable(); err == nil && enabled {
		if raw := subscriptionURL(settingSvc, subID, host, "clash"); raw != "" {
			urls = append(urls, SubscriptionURL{Format: "clash", URL: raw})
		}
	}
	return urls
}

func subscriptionURL(settingSvc *SettingService, subID, host, format string) string {
	var uri, path string
	var port int
	switch format {
	case "json":
		uri, _ = settingSvc.GetSubJsonURI()
		path, _ = settingSvc.GetSubJsonPath()
	case "clash":
		uri, _ = settingSvc.GetSubClashURI()
		path, _ = settingSvc.GetSubClashPath()
	default:
		uri, _ = settingSvc.GetSubURI()
		path, _ = settingSvc.GetSubPath()
	}
	if uri != "" {
		return strings.TrimRight(uri, "/") + "/" + subID
	}
	port, _ = settingSvc.GetSubPort()
	if path == "" {
		path = "/sub/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	if host == "" {
		if domain, _ := settingSvc.GetSubDomain(); domain != "" {
			host = domain
		} else {
			host = "localhost"
		}
	}
	if port > 0 {
		host = fmt.Sprintf("%s:%d", host, port)
	}
	return "http://" + host + path + subID
}
