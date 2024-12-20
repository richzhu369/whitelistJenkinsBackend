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

	// Get pagination parameters from query string
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "20")

	// Convert page and limit to integers
	pageInt, err := strconv.Atoi(page)
	if err != nil || pageInt < 1 {
		pageInt = 1
	}
	limitInt, err := strconv.Atoi(limit)
	if err != nil || limitInt < 1 {
		limitInt = 20
	}

	// Calculate offset
	offset := (pageInt - 1) * limitInt

	// Get search parameters from query string
	ip := c.DefaultQuery("Ip", "")
	opUser := c.DefaultQuery("OpUser", "")
	merchantNumber := c.DefaultQuery("MerchantNumber", "")

	// Build the query with pagination and sorting
	query := DB.Table("whitelist_logs").Select("created_at, ip, merchant_name, act, op_user").Order("created_at DESC").Offset(offset).Limit(limitInt)

	// Add search conditions to the query
	if ip != "" {
		query = query.Where("ip LIKE ?", "%"+ip+"%")
	}
	if opUser != "" {
		query = query.Where("op_user LIKE ?", "%"+opUser+"%")
	}
	if merchantNumber != "" {
		query = query.Where("merchant_name LIKE ?", "%"+merchantNumber+"%")
	}

	// Execute the query
	if err := query.Scan(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  50000,
			"error": err.Error(),
		})
		return
	}

	// Get the total count of filtered logs
	var total int64
	countQuery := DB.Table("whitelist_logs")
	if ip != "" {
		countQuery = countQuery.Where("ip LIKE ?", "%"+ip+"%")
	}
	if opUser != "" {
		countQuery = countQuery.Where("op_user LIKE ?", "%"+opUser+"%")
	}
	if merchantNumber != "" {
		countQuery = countQuery.Where("merchant_name LIKE ?", "%"+merchantNumber+"%")
	}
	if err := countQuery.Count(&total).Error; err != nil {
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
