package main

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"net/http"
	"net/netip"
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

	// 将 currentIPs 转换为 /48 前缀
	maskedCurrentIPs := make([]string, 0, len(currentIPs))
	for _, ip := range currentIPs {
		maskedIP, err := applyMaskToIPv6Single(ip)
		if err != nil {
			log.Printf("转换当前IP %s 为 /48 前缀失败: %v", ip, err)
			continue // 忽略转换失败的IP
		}
		maskedCurrentIPs = append(maskedCurrentIPs, maskedIP)
	}

	// 关键修改：在函数开始处声明 newIPList
	var newIPList string

	if action == "add" {
		for _, newIP := range newIPs {
			newIP = strings.TrimSpace(newIP)
			maskedNewIP, err := applyMaskToIPv6Single(newIP) // 将新IP转换为 /48 前缀
			if err != nil {
				log.Printf("转换新IP %s 为 /48 前缀失败: %v", newIP, err)
				failedIPs = append(failedIPs, newIP) //转换失败的IP加入到失败列表
				continue
			}
			if contains(maskedCurrentIPs, maskedNewIP) { // 使用转换后的前缀进行比较
				failedIPs = append(failedIPs, newIP)
				continue
			}
			validNewIPs = append(validNewIPs, newIP)
		}

		if len(validNewIPs) > 0 {
			if existingWhiteList.IP == "" {
				newIPList = strings.Join(validNewIPs, "\n")
			} else {
				combinedIPs := append(currentIPs, validNewIPs...)
				uniqueIPs := removeDuplicateValues(combinedIPs)
				newIPList = strings.Join(uniqueIPs, "\n")
			}
		}
		if len(failedIPs) > 0 {
			message := fmt.Sprintf("%s 商户 %s 的 IP %s 已存在 操作用户: %s", whiteList.Country, merchantName, strings.Join(failedIPs, ","), whiteList.OpUser)
			sendLarkMessage(message)
		}

	} else if action == "del" {
		for _, newIP := range newIPs {
			newIP = strings.TrimSpace(newIP)
			maskedNewIP, err := applyMaskToIPv6Single(newIP)
			if err != nil {
				log.Printf("转换要删除的IP %s 为 /48 前缀失败: %v", newIP, err)
				failedIPs = append(failedIPs, newIP) //转换失败的IP加入到失败列表
				continue
			}

			if !contains(maskedCurrentIPs, maskedNewIP) { // 使用转换后的前缀进行比较
				failedIPs = append(failedIPs, newIP)
				continue
			}
			validNewIPs = append(validNewIPs, newIP)
		}
		if len(failedIPs) > 0 {
			message := fmt.Sprintf("%s 商户 %s 的 IP %s 不存在，无法删除 操作用户: %s", whiteList.Country, merchantName, strings.Join(failedIPs, ","), whiteList.OpUser)
			sendLarkMessage(message)
		}

		// 正确地使用 /48 前缀构建 remainingIPs 列表
		remainingIPs := make([]string, 0, len(currentIPs))
		for _, currentIP := range currentIPs {
			maskedCurrentIP, err := applyMaskToIPv6Single(currentIP)
			if err != nil {
				log.Printf("转换数据库中IP %s 为 /48 前缀失败: %v", currentIP, err)
				remainingIPs = append(remainingIPs, currentIP) //转换失败也保留
				continue
			}
			shouldKeep := true
			for _, ipToDelete := range validNewIPs {
				maskedIPToDelete, err := applyMaskToIPv6Single(ipToDelete)
				if err != nil {
					log.Printf("转换要删除的IP %s 为 /48 前缀失败: %v", ipToDelete, err)
					continue
				}
				if maskedCurrentIP == maskedIPToDelete {
					shouldKeep = false
					break
				}
			}
			if shouldKeep {
				remainingIPs = append(remainingIPs, currentIP)
			}
		}
		uniqueIPs := removeDuplicateValues(remainingIPs)
		newIPList = strings.Join(uniqueIPs, "\n")
	} else {
		return "", nil, false, fmt.Errorf("操作类型错误")
	}

	if len(validNewIPs) == 0 {
		return "", nil, false, nil
	}

	newIPList = strings.TrimSpace(newIPList)

	hasValidIPs := true
	if newIPList == strings.TrimSpace(existingWhiteList.IP) {
		hasValidIPs = false
	}

	return newIPList, validNewIPs, hasValidIPs, nil
}

// sendLarkMessage 发送Lark消息的函数，包含去重逻辑
func sendLarkMessage(message string) {
	muLarkSent.Lock()
	if _, ok := larkSent[message]; !ok {
		larkChannel <- message
		larkSent[message] = true
		go func() {
			time.Sleep(1000 * time.Millisecond)
			muLarkSent.Lock()
			delete(larkSent, message)
			muLarkSent.Unlock()
		}()
	}
	muLarkSent.Unlock()
}

// applyMaskToIPv6Single 单个ip转换/48
func applyMaskToIPv6Single(ipStr string) (string, error) {
	ipStr = strings.TrimSpace(ipStr)
	//处理ipv6地址包含/的情况
	if strings.Contains(ipStr, "/") {
		ipStr = strings.Split(ipStr, "/")[0]
	}
	if ip, err := netip.ParseAddr(ipStr); err == nil && ip.Is6() {
		prefix, err := ip.Prefix(48)
		if err != nil {
			return "", fmt.Errorf("Failed to create prefix for %s: %w", ipStr, err)
		}
		return prefix.String(), nil
	} else if err != nil {
		return "", fmt.Errorf("ParseAddr(%q): %w", ipStr, err) // 包含原始错误信息
	} else {
		return ipStr, nil // 不是IPv6地址，直接返回
	}
}

// applyMaskToIPv6 applies a /48 mask to IPv6 addresses in a comma-separated list
func applyMaskToIPv6(ipList string) string {
	ips := strings.Split(ipList, ",")
	var maskedIPs []string

	for _, ipStr := range ips {
		maskedIP, err := applyMaskToIPv6Single(ipStr)
		if err != nil {
			log.Printf("转换IP %s 为 /48 前缀失败: %v", ipStr, err)
			maskedIPs = append(maskedIPs, ipStr) // 保留原始IP，避免丢失数据
			continue
		}
		maskedIPs = append(maskedIPs, maskedIP)
	}
	return strings.Join(maskedIPs, ",")
}

// 执行远程命令
func executeRemoteCommand(country, merchantName, ipList string, validNewIPs []string, action string, whiteList WhiteList) error {
	fmt.Println("商户名：", merchantName)
	var server, command1, command2, act string

	ipList = strings.ReplaceAll(ipList, "\n", ",")
	whiteListIP := strings.Join(validNewIPs, ",")

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
		command1 = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, applyMaskToIPv6(ipList))  // command1 应用掩码
		command2 = fmt.Sprintf("/data/jenkins/workspace/br-all-server/bsicrontask/bsicrontask 172.31.9.57:2379,172.31.4.34:2379,172.31.9.96:2379 /bs/%s.toml %s %s", merchantName, act, whiteListIP) // command2 不应用掩码
	case "pk":
		server = "16.162.63.178"
		command1 = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config-kp --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, applyMaskToIPv6(ipList)) // command1 应用掩码
		command2 = fmt.Sprintf("/opt/jenkins/workspace/pk-all-server/bsicrontask/bsicrontask 10.2.32.103:2379,10.2.32.101:2379,10.2.32.102:2379 /pk/%s.toml %s %s", merchantName, act, whiteListIP)    // command2 不应用掩码
	case "vn":
		server = "16.162.63.178"
		command1 = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, applyMaskToIPv6(ipList)) // command1 应用掩码
		command2 = fmt.Sprintf("/opt/jenkins/workspace/vn-all-server/bsicrontask/bsicrontask 10.0.3.102:2379,10.0.3.101:2379,10.0.3.103:2379 /vn/%s.toml %s %s", merchantName, act, whiteListIP)    // command2 不应用掩码
	case "ph":
		server = "18.167.173.173"
		command1 = fmt.Sprintf("/opt/script/ingressIpLimit --kubeconfig=/root/.kube/config --namespace=%s --ingressName=admin-%s --iplist=%s", merchantName, merchantName, applyMaskToIPv6(ipList))   // command1 应用掩码
		command2 = fmt.Sprintf("/var/lib/jenkins/workspace/php-all-server/bsicrontask/bsicrontask 10.1.3.101:2379,10.1.3.102:2379,10.1.3.103:2379 /ph/%s.toml %s %s", merchantName, act, whiteListIP) // command2 不应用掩码
	default:
		return fmt.Errorf("错误的国家代码")
	}

	// 执行修改ingress的白名单
	if err := executeSSHCommand(server, command1); err != nil {
		return fmt.Errorf("执行命令1失败: %w", err)
	}

	// 执行后端程序加白
	if err := executeSSHCommand(server, command2); err != nil {
		return fmt.Errorf("执行命令2失败: %w", err)
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

// 去除重复元素
func removeDuplicateValues(intSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}

	for _, entry := range intSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// validateAndRespond 验证并响应
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

	// 处理多个商户名
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
		validNewIPsStr := strings.Join(validNewIPs, ",")
		if err != nil {
			SendToLark(fmt.Sprintf("%s商户%s 白名单IP %s %s失败! 操作用户: %s", whiteList.Country, merchantName, validNewIPsStr, resText, whiteList.OpUser))
			mu.Lock()
			delete(processing, merchantName)
			mu.Unlock()
			processNextRequest(merchantName)
			return
		} else {
			// Corrected Lark message construction: use validNewIPs
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
