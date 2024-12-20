package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func whitelistLogList(c *gin.Context) {
	var logs []struct {
		CreatedAt    string `json:"created_at"`
		IP           string `json:"ip"`
		MerchantName string `json:"merchant_name"`
		Act          string `json:"act"`
		OpUser       string `json:"op_user"`
	}

	// 获得分页信息
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "20")

	// 转换分页参数为int
	pageInt, err := strconv.Atoi(page)
	if err != nil || pageInt < 1 {
		pageInt = 1
	}
	limitInt, err := strconv.Atoi(limit)
	if err != nil || limitInt < 1 {
		limitInt = 20
	}

	// 计算偏移量
	offset := (pageInt - 1) * limitInt

	// 查询数据库
	if err := DB.Table("whitelist_logs").Select("created_at, ip, merchant_name, act, op_user").Offset(offset).Limit(limitInt).Scan(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  50000,
			"error": err.Error(),
		})
		return
	}

	// 查询总数
	var total int64
	if err := DB.Table("whitelist_logs").Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  50000,
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":  20000,
		"data":  logs,
		"total": total,
	})
}
