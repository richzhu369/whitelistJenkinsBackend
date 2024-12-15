package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func whitelistLogList(c *gin.Context) {
	var logs []struct {
		CreatedAt    string `json:"created_at"`
		IP           string `json:"ip"`
		MerchantName string `json:"merchant_name"`
		Act          string `json:"act"`
		OpUser       string `json:"op_user"`
	}

	if err := DB.Table("whitelist_logs").Select("created_at, ip, merchant_name, act, op_user").Scan(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  50000,
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 20000,
		"data": logs,
	})
}
