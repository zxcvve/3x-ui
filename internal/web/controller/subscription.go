package controller

import (
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type SubscriptionController struct {
	subscriptionService service.SubscriptionService
	inboundService      service.InboundService
	xrayService         service.XrayService
}

func NewSubscriptionController(g *gin.RouterGroup) *SubscriptionController {
	a := &SubscriptionController{}
	a.initRouter(g)
	return a
}

func (a *SubscriptionController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/get/:subId", a.get)
	g.POST("/:subId/routedProfile", a.createRoutedProfile)
}

func (a *SubscriptionController) list(c *gin.Context) {
	rows, err := a.subscriptionService.ListSummaries()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonObj(c, rows, nil)
}

func (a *SubscriptionController) get(c *gin.Context) {
	detail, err := a.subscriptionService.GetDetail(c.Param("subId"), c.Request.Host)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonObj(c, detail, nil)
}

func (a *SubscriptionController) createRoutedProfile(c *gin.Context) {
	var req service.RoutedProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	req.SubID = c.Param("subId")
	member, needRestart, err := a.subscriptionService.CreateRoutedProfile(&a.inboundService, req)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	jsonObj(c, member, nil)
}
