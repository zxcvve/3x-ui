package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestSubscriptionControllerListAndGet(t *testing.T) {
	newHostTestDB(t)
	engine := gin.New()
	NewSubscriptionController(engine.Group("/panel/api/subscriptions"))

	client := model.ClientRecord{Email: "sub@example.com", SubID: "api-sub", Enable: true}
	if err := database.GetDB().Create(&client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}

	list := doHostReq(t, engine, http.MethodGet, "/panel/api/subscriptions/list", nil)
	var summaries []map[string]any
	if err := json.Unmarshal(list.Obj, &summaries); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(summaries) != 1 || summaries[0]["subId"] != "api-sub" {
		t.Fatalf("list summaries = %#v", summaries)
	}

	get := doHostReq(t, engine, http.MethodGet, "/panel/api/subscriptions/get/api-sub", nil)
	var detail map[string]any
	if err := json.Unmarshal(get.Obj, &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail["subId"] != "api-sub" {
		t.Fatalf("detail = %#v", detail)
	}
}

func TestSubscriptionControllerCreateRoutedProfile(t *testing.T) {
	newHostTestDB(t)
	engine := gin.New()
	NewSubscriptionController(engine.Group("/panel/api/subscriptions"))

	ib := &model.Inbound{Tag: "sub-route-in", Enable: true, Port: 31001, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := model.ClientRecord{Email: "base-api-route@example.com", SubID: "api-route-sub", Enable: true}
	if err := database.GetDB().Create(&client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := database.GetDB().Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client inbound: %v", err)
	}
	if err := database.GetDB().Create(&model.Setting{
		Key:   "xrayTemplateConfig",
		Value: `{"outbounds":[{"tag":"direct","protocol":"freedom","settings":{}}],"routing":{"rules":[]}}`,
	}).Error; err != nil {
		t.Fatalf("seed xray template: %v", err)
	}

	env := doHostReq(t, engine, http.MethodPost, "/panel/api/subscriptions/api-route-sub/routedProfile", map[string]any{
		"email":       "created-api-route@example.com",
		"inboundIds":  []int{ib.Id},
		"outboundTag": "direct",
	})
	if !env.Success {
		t.Fatalf("create routed profile failed: %s", env.Msg)
	}
	var obj map[string]any
	if err := json.Unmarshal(env.Obj, &obj); err != nil {
		t.Fatalf("decode routed profile response: %v", err)
	}
	if obj["email"] != "created-api-route@example.com" || obj["routeTag"] != "direct" {
		t.Fatalf("routed profile obj = %#v", obj)
	}
}

func TestSubscriptionControllerAuthInherited(t *testing.T) {
	newHostTestDB(t)
	engine := gin.New()
	store := cookie.NewStore([]byte("subscription-auth-test-secret"))
	engine.Use(sessions.Sessions("3x-ui", store))

	a := &APIController{}
	api := engine.Group("/panel/api")
	api.Use(a.checkAPIAuth)
	NewSubscriptionController(api.Group("/subscriptions"))

	req := httptest.NewRequest(http.MethodGet, "/panel/api/subscriptions/list", nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated subscriptions/list = %d, want 401 (auth inherited)", w.Code)
	}
}
