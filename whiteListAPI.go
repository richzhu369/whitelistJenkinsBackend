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

// 对比ip是否在列表中
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// processIPs IP地址格式处理与检查是否存在
func processIPs(whiteList WhiteList, merchantName string, action string) (string, error) {
	var existingWhiteList WhiteList
	if err := DB.Where("merchant_name = ?", merchantName).First(&existingWhiteList).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	currentIPs := strings.Split(existingWhiteList.IP, ",")
	newIPs := strings.Split(whiteList.IP, ",")

	if action == "add" {
		// 检测重复IP
		for _, newIP := range newIPs {
			if contains(currentIPs, newIP) {
				return "", fmt.Errorf("IP %s 已存在", newIP)
			}
		}
		if existingWhiteList.IP == "" {
			return strings.Join(newIPs, ","), nil
		}
		combinedIPs := append(currentIPs, newIPs...)
		return strings.Join(combinedIPs, ","), nil
	} else if action == "del" {
		// 检测不存在IP
		for _, newIP := range newIPs {
			if !contains(currentIPs, newIP) {
				return "", fmt.Errorf("IP %s 不存在", newIP)
			}
		}
		remainingIPs := make([]string, 0)
		for _, ip := range currentIPs {
			if !contains(newIPs, ip) {
				remainingIPs = append(remainingIPs, ip)
			}
		}
		return strings.Join(remainingIPs, ","), nil
	}
	return "", fmt.Errorf("操作类型错误")
}

// 执行远程命令
func executeRemoteCommand(country, merchantName, ipList, action string, whiteList WhiteList) error {
	var server, command1, command2, act string
	switch action {
	case "add":
		act = "add_ip"
	case "del":
		act = "del_ip"
	default:
		return fmt.Errorf("错误的操作类型")
	}

	switch country {
	case "br":
		server = "15.229.106.224"
		command1 = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, ipList)
		command2 = fmt.Sprintf("/data/jenkins/workspace/br-all-server/bsicrontask/bsicrontask 172.31.9.57:2379,172.31.4.34:2379,172.31.9.96:2379 /bs/%s.toml %s %s", merchantName, act, whiteList.IP)
	case "pk":
		server = "16.162.63.178"
		command1 = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config-kp --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, ipList)
		command2 = fmt.Sprintf("/opt/jenkins/workspace/pk-all-server/bsicrontask/bsicrontask 10.2.32.103:2379,10.2.32.101:2379,10.2.32.102:2379 /pk/%s.toml %s %s", merchantName, act, whiteList.IP)
	case "vn":
		server = "16.162.63.178"
		command1 = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, ipList)
		command2 = fmt.Sprintf("/opt/jenkins/workspace/vn-all-server/bsicrontask/bsicrontask 10.0.3.102:2379,10.0.3.101:2379,10.0.3.103:2379 /vn/%s.toml %s %s", merchantName, act, whiteList.IP)
	case "ph":
		server = "18.167.173.173"
		command1 = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, ipList)
		command2 = fmt.Sprintf("/var/lib/jenkins/workspace/php-all-server/bsicrontask/bsicrontask 10.1.3.101:2379,10.1.3.102:2379,10.1.3.103:2379 /ph/%s.toml %s %s", merchantName, act, whiteList.IP)
	default:
		return fmt.Errorf("错误的国家代码")
	}

	// 执行修改ingress的白名单
	if err := executeSSHCommand(server, command1); err != nil {
		return err
	}

	// 执行后端程序加白
	if err := executeSSHCommand(server, command2); err != nil {
		return err
	}

	return nil
}

// 更新数据库并记录日志
func updateDatabaseAndLog(whiteList WhiteList, merchantName, ipList, action string) error {
	var existingWhiteList WhiteList
	if err := DB.Where("merchant_name = ?", merchantName).First(&existingWhiteList).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if action == "add" {
		if existingWhiteList.MerchantName != "" {
			existingWhiteList.IP = ipList
			if err := DB.Save(&existingWhiteList).Error; err != nil {
				return err
			}
		} else {
			newWhiteList := WhiteList{
				MerchantName: merchantName,
				IP:           ipList,
				Country:      whiteList.Country,
				OpUser:       whiteList.OpUser,
			}
			if err := DB.Create(&newWhiteList).Error; err != nil {
				return err
			}
		}
	} else if action == "del" {
		existingWhiteList.IP = ipList
		if err := DB.Save(&existingWhiteList).Error; err != nil {
			return err
		}
	}

	whitelistLog := WhitelistLog{
		MerchantName: merchantName,
		IP:           whiteList.IP,
		Act:          action,
		OpUser:       whiteList.OpUser,
		Model: gorm.Model{
			CreatedAt: time.Now().In(time.Local),
		},
	}
	return DB.Create(&whitelistLog).Error
}

// validateAndRespond performs validation and responds to the client
func validateAndRespond(c *gin.Context, action string) (WhiteList, error) {
	var whiteList WhiteList
	if err := c.ShouldBindJSON(&whiteList); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    40000,
			"message": "格式错误",
		})
		return whiteList, err
	}

	if whiteList.OpUser == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":    40000,
			"message": "您未登录，权限被拒绝",
		})
		return whiteList, fmt.Errorf("未登录")
	}

	// 校验IP地址的格式
	err := ValidateWhiteListIPs(whiteList)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    40000,
			"message": err.Error(),
		})
		return whiteList, err
	}

	var actionText string
	if action == "add" {
		actionText = "添加"
	} else {
		actionText = "删除"
	}

	// Process IPs and check for errors
	merchantNames := strings.Split(whiteList.MerchantName, ",")
	for _, merchantName := range merchantNames {
		_, err := processIPs(whiteList, merchantName, action)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    40000,
				"message": err.Error(),
			})
			return whiteList, err
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    20000,
		"message": fmt.Sprintf("正在%s白名单，请稍后查看结果", actionText),
	})
	return whiteList, nil
}

// 添加或删除白名单业务逻辑
func whitelistModify(whiteList WhiteList, action string) {
	log.Println(time.Now().In(time.Local))

	// 拆分商户，一个商户一个商户的处理
	merchantNames := strings.Split(whiteList.MerchantName, ",")
	for _, merchantName := range merchantNames {
		ipList, err := processIPs(whiteList, merchantName, action)
		if err != nil {
			log.Printf("处理IP失败: %v", err)
			SendToLark(fmt.Sprintf("%s商户%s 白名单IP %s %s失败! 操作用户: %s", whiteList.Country, merchantName, whiteList.IP, action, whiteList.OpUser))
			return
		}

		err = executeRemoteCommand(whiteList.Country, merchantName, ipList, action, whiteList)
		if err != nil {
			log.Printf("执行远程命令失败: %v", err)
			SendToLark(fmt.Sprintf("%s商户%s 白名单IP %s %s失败! 操作用户: %s", whiteList.Country, merchantName, whiteList.IP, action, whiteList.OpUser))
			return
		} else {
			SendToLark(fmt.Sprintf("%s商户%s 白名单IP %s %s成功! 操作用户: %s", whiteList.Country, merchantName, whiteList.IP, action, whiteList.OpUser))
		}

		err = updateDatabaseAndLog(whiteList, merchantName, ipList, action)
		if err != nil {
			log.Printf("更新数据库失败: %v", err)
			return
		}
	}
}

// 添加白名单入口
func whitelistAdd(c *gin.Context) {
	whiteList, err := validateAndRespond(c, "add")
	if err == nil {
		go whitelistModify(whiteList, "add")
	}
}

// 删除白名单入口
func whitelistDelete(c *gin.Context) {
	whiteList, err := validateAndRespond(c, "del")
	if err == nil {
		go whitelistModify(whiteList, "del")
	}
}
