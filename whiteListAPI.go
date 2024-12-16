package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func whitelistAdd(c *gin.Context) {
	var whiteList WhiteList
	if err := c.ShouldBindJSON(&whiteList); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if whiteList.OpUser == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  40000,
			"error": "未登陆，权限被拒绝",
		})
		return
	}

	// Split the IP addresses
	ips := strings.Split(whiteList.IP, ",")

	// 检测商户号+IP的组合是否存在于 WhiteList 表
	for _, ip := range ips {
		var existingWhiteList WhiteList
		if err := DB.Where("merchant_name = ? AND ip = ?", whiteList.MerchantName, ip).First(&existingWhiteList).Error; err == nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    40900,
				"message": "商户" + whiteList.MerchantName + "的白名单IP" + ip + "已存在",
			})
			return
		}
	}
	//var existingWhiteList WhiteList
	//if err := DB.Where("merchant_name = ? AND ip = ?", whiteList.MerchantName, whiteList.IP).First(&existingWhiteList).Error; err == nil {
	//	c.JSON(http.StatusOK, gin.H{
	//		"code":    40900,
	//		"message": "商户" + whiteList.MerchantName + "的白名单IP" + whiteList.IP + "已存在",
	//	})
	//	return
	//}

	// 根据 country 值执行不同的命令
	var server, command string
	switch whiteList.Country {
	case "br":
		server = "15.229.106.224"
		command = fmt.Sprintf("/data/jenkins/workspace/br-all-server/bsicrontask/bsicrontask 172.31.9.57:2379,172.31.4.34:2379,172.31.9.96:2379 /bs/%s.toml add_ip %s", whiteList.MerchantName, whiteList.IP)
	case "pk":
		server = "15.229.106.224"
		command = fmt.Sprintf("/opt/jenkins/workspace/pk-all-server/bsicrontask/bsicrontask 10.2.32.103:2379,10.2.32.101:2379,10.2.32.102:2379 /pk/%s.toml add_ip %s", whiteList.MerchantName, whiteList.IP)
	case "vn":
		server = "15.229.106.224"
		command = fmt.Sprintf("/opt/jenkins/workspace/vn-all-server/bsicrontask/bsicrontask 10.0.3.102:2379,10.0.3.101:2379,10.0.3.103:2379 /vn/%s.toml add_ip %s", whiteList.MerchantName, whiteList.IP)
	case "ph":
		server = "18.167.173.173"
		command = fmt.Sprintf("/var/lib/jenkins/workspace/php-all-server/bsicrontask/bsicrontask 10.1.3.101:2379,10.1.3.102:2379,10.1.3.103:2379 /ph%s.toml add_ip %s", whiteList.MerchantName, whiteList.IP)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  40000,
			"error": "无效的国家代码",
		})
		return
	}

	if err := executeSSHCommand(server, command); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":  50000,
			"error": err.Error(),
		})
		return
	}

	// 添加到 WhiteList 表
	if err := DB.Create(&whiteList).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":  50000,
			"error": err.Error(),
		})
		return
	}

	// 记录到 WhitelistLog 表
	whitelistLog := WhitelistLog{
		MerchantName: whiteList.MerchantName,
		IP:           whiteList.IP,
		Act:          "add",
		OpUser:       whiteList.OpUser,
	}

	if err := DB.Create(&whitelistLog).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":  50000,
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    20000,
		"message": "商户" + whiteList.MerchantName + "的白名单IP" + whiteList.IP + "已添加成功",
	})
}

func whitelistDelete(c *gin.Context) {
	var whiteList WhiteList
	if err := c.ShouldBindJSON(&whiteList); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":  40000,
			"error": err.Error(),
		})
		return
	}

	if whiteList.OpUser == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":  40000,
			"error": "未登陆，权限被拒绝",
		})
		return
	}

	// Split the IP addresses
	ips := strings.Split(whiteList.IP, ",")

	// 检测商户名+IP的组合是否存在于 WhiteList 表
	for _, ip := range ips {
		var existingWhiteList WhiteList
		if err := DB.Where("merchant_name = ? AND ip = ?", whiteList.MerchantName, ip).First(&existingWhiteList).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    40400,
				"message": "商户" + whiteList.MerchantName + "的白名单IP" + ip + "不存在，无需执行删除操作",
			})
			return
		}
	}

	// 检测商户名+IP的组合是否存在于 WhiteList 表
	//var existingWhiteList WhiteList
	//if err := DB.Where("merchant_name = ? AND ip = ?", whiteList.MerchantName, whiteList.IP).First(&existingWhiteList).Error; err != nil {
	//	c.JSON(http.StatusOK, gin.H{
	//		"code":    40400,
	//		"message": "商户" + whiteList.MerchantName + "的白名单IP" + whiteList.IP + "不存在，无需执行删除操作",
	//	})
	//	return
	//}

	// 根据 country 值执行不同的命令
	var server, command string
	switch whiteList.Country {
	case "br":
		server = "15.229.106.224"
		command = fmt.Sprintf("/data/jenkins/workspace/br-all-server/bsicrontask/bsicrontask 172.31.9.57:2379,172.31.4.34:2379,172.31.9.96:2379 /bs/%s.toml del_ip %s", whiteList.MerchantName, whiteList.IP)
	case "pk":
		server = "15.229.106.224"
		command = fmt.Sprintf("/opt/jenkins/workspace/pk-all-server/bsicrontask/bsicrontask 10.2.32.103:2379,10.2.32.101:2379,10.2.32.102:2379 /pk/%s.toml del_ip %s", whiteList.MerchantName, whiteList.IP)
	case "vn":
		server = "15.229.106.224"
		command = fmt.Sprintf("/opt/jenkins/workspace/vn-all-server/bsicrontask/bsicrontask 10.0.3.102:2379,10.0.3.101:2379,10.0.3.103:2379 /vn/%s.toml del_ip %s", whiteList.MerchantName, whiteList.IP)
	case "ph":
		server = "18.167.173.173"
		command = fmt.Sprintf("/var/lib/jenkins/workspace/php-all-server/bsicrontask/bsicrontask 10.1.3.101:2379,10.1.3.102:2379,10.1.3.103:2379 /ph/%s.toml del_ip %s", whiteList.MerchantName, whiteList.IP)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  40000,
			"error": "无效的国家代码",
		})
		return
	}

	if err := executeSSHCommand(server, command); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  50000,
			"error": err.Error(),
		})
		return
	}

	// 删除 WhiteList 表中的白名单
	if err := DB.Where("merchant_name = ? AND ip = ?", whiteList.MerchantName, whiteList.IP).Delete(&WhiteList{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  50000,
			"error": err.Error(),
		})
		return
	}

	// 记录到 WhitelistLog 表
	whitelistLog := WhitelistLog{
		MerchantName: whiteList.MerchantName,
		IP:           whiteList.IP,
		Act:          "del",
		OpUser:       whiteList.OpUser,
	}

	if err := DB.Create(&whitelistLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  50000,
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    20000,
		"message": "商户" + whiteList.MerchantName + "的白名单IP" + whiteList.IP + "已删除",
	})
}
