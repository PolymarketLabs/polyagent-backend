package controller

import "github.com/gin-gonic/gin"

type InvestorController struct {
	BaseController
	// 这里通常会注入 Service 接口
	// InvestorService service.InvestorService
}

// 个人投资组合
func (ic *InvestorController) GetPortfolio(c *gin.Context) {
	Success(c, "TODO.. GetPortfolio Success")
}

// 申赎历史
func (ic *InvestorController) GetHistory(c *gin.Context) {
	Success(c, "TODO.. GetHistory Success")
}

// 收益榜单
func (ic *InvestorController) GetRankings(c *gin.Context) {
	Success(c, "TODO.. GetRankings Success")
}
