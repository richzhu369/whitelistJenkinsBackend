package main

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Request struct {
	WhiteList WhiteList
	Action    string
}

var (
	merchantQueue = make(map[string][]Request)
	processing    = make(map[string]bool)
	mu            sync.Mutex
	larkChannel   = make(chan string)     // 用于发送 Lark 消息的通道
	larkSent      = make(map[string]bool) // 记录是否已发送过 Lark 消息
	muLarkSent    sync.Mutex              // 保护 larkSent 的互斥锁
)

func processNextRequest(merchantName string) {
	mu.Lock()
	defer mu.Unlock()

	if len(merchantQueue[merchantName]) == 0 {
		delete(processing, merchantName)
		return
	}

	req := merchantQueue[merchantName][0]
	merchantQueue[merchantName] = merchantQueue[merchantName][1:]

	go func() {
		whitelistModify(req.WhiteList, req.Action)
	}()
}

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
func processIPs(whiteList WhiteList, merchantName string, action string) (string, []string, bool, error) {
	var existingWhiteList WhiteList
	if err := DB.Where("merchant_name = ?", merchantName).First(&existingWhiteList).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil, false, fmt.Errorf("查询数据库失败: %w", err)
	}

	currentIPs := strings.Split(existingWhiteList.IP, "\n")
	newIPs := strings.Split(whiteList.IP, "\n")

	failedIPs := make([]string, 0)
	validNewIPs := make([]string, 0)

	if action == "add" {
		for _, newIP := range newIPs {
			if contains(currentIPs, newIP) {
				failedIPs = append(failedIPs, newIP)
				continue
			}
			validNewIPs = append(validNewIPs, newIP)
		}
	} else if action == "del" {
		for _, newIP := range newIPs {
			if !contains(currentIPs, newIP) {
				failedIPs = append(failedIPs, newIP)
				continue
			}
			validNewIPs = append(validNewIPs, newIP)
		}
	} else {
		return "", nil, false, fmt.Errorf("操作类型错误")
	}

	if len(failedIPs) > 0 {
		message := fmt.Sprintf("%s 商户 %s 的 IP %s 已存在 操作用户: %s", whiteList.Country, merchantName, strings.Join(failedIPs, ","), whiteList.OpUser)

		muLarkSent.Lock()
		if _, ok := larkSent[message]; !ok {
			larkChannel <- message
			larkSent[message] = true
			go func() {
				time.Sleep(5 * time.Minute)
				muLarkSent.Lock()
				delete(larkSent, message)
				muLarkSent.Unlock()
			}()
		}
		muLarkSent.Unlock()
	}

	if len(validNewIPs) == 0 {
		return "", nil, false, nil // 返回 nil 的 validNewIPs
	}

	var newIPList string
	if action == "add" {
		if existingWhiteList.IP == "" {
			newIPList = strings.Join(validNewIPs, "\n")
		} else {
			combinedIPs := append(currentIPs, validNewIPs...)
			newIPList = strings.Join(combinedIPs, "\n")
		}
	} else {
		remainingIPs := make([]string, 0)
		for _, ip := range currentIPs {
			if !contains(validNewIPs, ip) {
				remainingIPs = append(remainingIPs, ip)
			}
		}
		newIPList = strings.Join(remainingIPs, "\n")
	}

	hasValidIPs := true
	if newIPList == existingWhiteList.IP {
		hasValidIPs = false
	}

	return newIPList, validNewIPs, hasValidIPs, nil // 返回 newIPList, validNewIPs, hasValidIPs
}

// 执行远程命令
func executeRemoteCommand(country, merchantName, ipList string, validNewIPs []string, action string, whiteList WhiteList) error {
	fmt.Println("商户名：", merchantName)
	var server, command1, command2, act string

	ipList = strings.ReplaceAll(ipList, "\n", ",")

	whiteListIP := strings.Join(validNewIPs, ",") // 使用 validNewIPs 构建 whiteListIP

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
		command2 = fmt.Sprintf("/data/jenkins/workspace/br-all-server/bsicrontask/bsicrontask 172.31.9.57:2379,172.31.4.34:2379,172.31.9.96:2379 /bs/%s.toml %s %s", merchantName, act, whiteListIP)
	case "pk":
		server = "16.162.63.178"
		command1 = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config-kp --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, ipList)
		command2 = fmt.Sprintf("/opt/jenkins/workspace/pk-all-server/bsicrontask/bsicrontask 10.2.32.103:2379,10.2.32.101:2379,10.2.32.102:2379 /pk/%s.toml %s %s", merchantName, act, whiteListIP)
	case "vn":
		server = "16.162.63.178"
		command1 = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, ipList)
		command2 = fmt.Sprintf("/opt/jenkins/workspace/vn-all-server/bsicrontask/bsicrontask 10.0.3.102:2379,10.0.3.101:2379,10.0.3.103:2379 /vn/%s.toml %s %s", merchantName, act, whiteListIP)
	case "ph":
		server = "18.167.173.173"
		command1 = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, ipList)
		command2 = fmt.Sprintf("/var/lib/jenkins/workspace/php-all-server/bsicrontask/bsicrontask 10.1.3.101:2379,10.1.3.102:2379,10.1.3.103:2379 /ph/%s.toml %s %s", merchantName, act, whiteListIP)
	default:
		return fmt.Errorf("错误的国家代码")
	}

	// 执行修改ingress的白名单
	if err := executeSSHCommand(server, command1); err != nil {
		return fmt.Errorf("执行命令1失败: %w", err) // 包装错误
	}

	// 执行后端程序加白
	if err := executeSSHCommand(server, command2); err != nil {
		return fmt.Errorf("执行命令2失败: %w", err) // 包装错误
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
		_, _, _, err := processIPs(whiteList, merchantName, action) // Capture all 4 return values
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
	merchantNames := strings.Split(whiteList.MerchantName, ",")
	for _, merchantName := range merchantNames {
		mu.Lock()
		if processing[merchantName] {
			merchantQueue[merchantName] = append(merchantQueue[merchantName], Request{WhiteList: whiteList, Action: action})
			mu.Unlock()
			return
		}
		processing[merchantName] = true
		mu.Unlock()

		ipList, validNewIPs, hasValidIPs, err := processIPs(whiteList, merchantName, action)
		if err != nil {
			log.Printf("处理IP失败: %v", err)
			mu.Lock()
			delete(processing, merchantName)
			mu.Unlock()
			processNextRequest(merchantName)
			return
		}

		if !hasValidIPs {
			mu.Lock()
			delete(processing, merchantName)
			mu.Unlock()
			processNextRequest(merchantName)
			return
		}

		resText := ""
		if action == "add" {
			resText = "添加"
		} else {
			resText = "删除"
		}

		err = executeRemoteCommand(whiteList.Country, merchantName, ipList, validNewIPs, action, whiteList)
		if err != nil {
			log.Printf("执行远程命令失败: %v", err)
			SendToLark(fmt.Sprintf("%s商户%s 白名单IP %s %s失败! 操作用户: %s", whiteList.Country, merchantName, ipList, resText, whiteList.OpUser))
			mu.Lock()
			delete(processing, merchantName)
			mu.Unlock()
			processNextRequest(merchantName)
			return
		} else {
			// Corrected Lark message construction: use validNewIPs
			validNewIPsStr := strings.Join(validNewIPs, ",") // Format validNewIPs for the message
			SendToLark(fmt.Sprintf("%s商户%s 白名单IP %s %s成功! 操作用户: %s", whiteList.Country, merchantName, validNewIPsStr, resText, whiteList.OpUser))
		}

		err = updateDatabaseAndLog(whiteList, merchantName, ipList, action)
		if err != nil {
			log.Printf("更新数据库失败: %v", err)
			mu.Lock()
			delete(processing, merchantName)
			mu.Unlock()
			processNextRequest(merchantName)
			return
		}

		mu.Lock()
		delete(processing, merchantName)
		mu.Unlock()

		processNextRequest(merchantName)
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
