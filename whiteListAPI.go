package main

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"net/http"
	"strings"
	"time"
)

// 处理数据，并执行ssh调用go程序，添加白名单到ingress
func whitelistAdd(c *gin.Context) {
	log.Println(time.Now().In(time.Local))
	var whiteList WhiteList
	if err := c.ShouldBindJSON(&whiteList); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    40000,
			"message": "传入的数据格式错误",
		})
		return
	}

	if whiteList.OpUser == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":    40000,
			"message": "未登陆，权限被拒绝",
		})
		return
	}

	// ip地址校验
	err := ValidateWhiteListIPs(whiteList)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    40000,
			"message": err.Error(),
		})
		return
	}

	// Split the merchant names
	merchantNames := strings.Split(whiteList.MerchantName, ",")

	// Add each merchant name with the entire IP list
	for _, merchantName := range merchantNames {
		// Retrieve current whitelist IPs for the merchant
		var existingWhiteList WhiteList
		if err := DB.Where("merchant_name = ?", merchantName).First(&existingWhiteList).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{
				"code":    50000,
				"message": "查询数据库失败",
			})
			return
		}

		// Combine current IPs with new IPs
		currentIPs := strings.Split(existingWhiteList.IP, ",")
		newIPs := strings.Split(whiteList.IP, ",")

		// Check for duplicate IPs
		var duplicateIPs []string
		for _, newIP := range newIPs {
			for _, currentIP := range currentIPs {
				if newIP == currentIP {
					duplicateIPs = append(duplicateIPs, newIP)
				}
			}
		}

		if len(duplicateIPs) > 0 {
			c.JSON(http.StatusOK, gin.H{
				"code":    40000,
				"message": fmt.Sprintf("以下IP已存在: %s", strings.Join(duplicateIPs, ", ")),
			})
			return
		}

		// Remove empty strings from currentIPs
		var validCurrentIPs []string
		for _, ip := range currentIPs {
			if ip != "" {
				validCurrentIPs = append(validCurrentIPs, ip)
			}
		}

		combinedIPs := append(validCurrentIPs, newIPs...)
		combinedIPStr := strings.Join(combinedIPs, ",")

		// 根据 country 值执行不同的命令
		var server, command string
		switch whiteList.Country {
		case "br":
			server = "15.229.106.224"
			command = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, combinedIPStr)
		case "pk":
			server = "16.162.63.178"
			command = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config-kp --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, combinedIPStr)
		case "vn":
			server = "16.162.63.178"
			command = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, combinedIPStr)
		case "ph":
			server = "18.167.173.173"
			command = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, combinedIPStr)
		default:
			c.JSON(http.StatusOK, gin.H{
				"code":    40000,
				"message": "无效的国家代码,目前只支持，br,ph,vn,pk",
			})
			return
		}

		if err := executeSSHCommand(server, command); err != nil {
			log.Println(err.Error())
			c.JSON(http.StatusOK, gin.H{
				"code":    50000,
				"message": "执行远程命令失败",
			})
			return
		}

		// Update or create the WhiteList entry
		if existingWhiteList.MerchantName != "" {
			existingWhiteList.IP = combinedIPStr
			if err := DB.Save(&existingWhiteList).Error; err != nil {
				log.Println(err.Error())
				c.JSON(http.StatusOK, gin.H{
					"code":    50000,
					"message": "更新数据库失败",
				})
				return
			}
		} else {
			newWhiteList := WhiteList{
				MerchantName: merchantName,
				IP:           combinedIPStr,
				Country:      whiteList.Country,
				OpUser:       whiteList.OpUser,
			}
			if err := DB.Create(&newWhiteList).Error; err != nil {
				c.JSON(http.StatusOK, gin.H{
					"code":    50000,
					"message": "插入数据库失败",
				})
				return
			}
		}

		// 记录到 WhitelistLog 表
		whitelistLog := WhitelistLog{
			MerchantName: merchantName,
			IP:           combinedIPStr,
			Act:          "add",
			OpUser:       whiteList.OpUser,
			Model: gorm.Model{
				CreatedAt: time.Now().In(time.Local),
			},
		}

		if err := DB.Create(&whitelistLog).Error; err != nil {
			log.Println(err.Error())
			c.JSON(http.StatusOK, gin.H{
				"code":    50000,
				"message": "插入数据库失败",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    20000,
		"message": "商户" + whiteList.MerchantName + "的白名单IP" + whiteList.IP + "已添加成功",
	})
}

func whitelistDelete(c *gin.Context) {
	log.Println(time.Now().In(time.Local))
	var whiteList WhiteList
	if err := c.ShouldBindJSON(&whiteList); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    40000,
			"message": "传入的数据格式错误",
		})
		return
	}

	if whiteList.OpUser == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":    40000,
			"message": "未登陆，权限被拒绝",
		})
		return
	}

	// ip地址校验
	err := ValidateWhiteListIPs(whiteList)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    40000,
			"message": err.Error(),
		})
		return
	}

	// Split the merchant names
	merchantNames := strings.Split(whiteList.MerchantName, ",")

	// Delete each merchant name with the entire IP list
	for _, merchantName := range merchantNames {
		// Retrieve current whitelist IPs for the merchant
		var existingWhiteList WhiteList
		if err := DB.Where("merchant_name = ?", merchantName).First(&existingWhiteList).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    40400,
				"message": "商户" + merchantName + "的白名单IP不存在，无需执行删除操作",
			})
			return
		}

		// Remove the specified IPs from the current IP list
		currentIPs := strings.Split(existingWhiteList.IP, ",")
		newIPs := strings.Split(whiteList.IP, ",")

		// Check for non-existent IPs
		var nonExistentIPs []string
		for _, newIP := range newIPs {
			if !contains(currentIPs, newIP) {
				nonExistentIPs = append(nonExistentIPs, newIP)
			}
		}

		if len(nonExistentIPs) > 0 {
			c.JSON(http.StatusOK, gin.H{
				"code":    40000,
				"message": fmt.Sprintf("以下IP不存在: %s", strings.Join(nonExistentIPs, ", ")),
			})
			return
		}

		remainingIPs := make([]string, 0)
		for _, ip := range currentIPs {
			if !contains(newIPs, ip) {
				remainingIPs = append(remainingIPs, ip)
			}
		}
		remainingIPStr := strings.Join(remainingIPs, ",")

		// 根据 country 值执行不同的命令
		var server, command string
		switch whiteList.Country {
		case "br":
			server = "15.229.106.224"
			command = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, remainingIPStr)
		case "pk":
			server = "16.162.63.178"
			command = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config-kp --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, remainingIPStr)
		case "vn":
			server = "16.162.63.178"
			command = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, remainingIPStr)
		case "ph":
			server = "18.167.173.173"
			command = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, remainingIPStr)
		default:
			c.JSON(http.StatusOK, gin.H{
				"code":    40000,
				"message": "无效的国家代码,目前只支持，br,ph,vn,pk",
			})
			return
		}

		if err := executeSSHCommand(server, command); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    50000,
				"message": "执行远程命令失败",
			})
			return
		}

		// Update the WhiteList entry
		existingWhiteList.IP = remainingIPStr
		if err := DB.Save(&existingWhiteList).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    50000,
				"message": "更新数据库失败",
			})
			return
		}

		// 记录到 WhitelistLog 表
		whitelistLog := WhitelistLog{
			MerchantName: merchantName,
			IP:           whiteList.IP,
			Act:          "del",
			OpUser:       whiteList.OpUser,
			Model: gorm.Model{
				CreatedAt: time.Now().In(time.Local),
			},
		}

		if err := DB.Create(&whitelistLog).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    50000,
				"message": "插入数据库失败",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    20000,
		"message": "商户" + whiteList.MerchantName + "的白名单IP" + whiteList.IP + "已删除",
	})
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
