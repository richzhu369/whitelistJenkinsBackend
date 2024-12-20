package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

func userLogin(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
		return
	}

	// 现在可以使用 loginReq.Username 和 loginReq.Password 获取 JSON 中的值
	fmt.Printf("Username: %s, Password: %s, Role: %s\n", user.Username, user.Password, user.Role)

	// 进行登录验证逻辑...
	dbUser := new(User)
	if err := DB.Where("username = ?", user.Username).First(dbUser).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 60000, "message": "用户不存在"})
		return
	}

	// 密码验证成功，生成 token 并返回
	token := user.Username

	if dbUser.CheckPassword(user.Password, dbUser.PasswordHash) {
		// 更新登陆时间
		now := time.Now()
		location, _ := time.LoadLocation("Asia/Shanghai")
		nowInChina := now.In(location)

		DB.Model(&User{}).Where("username = ?", user.Username).Update("LastLoginTime", nowInChina)

		c.JSON(http.StatusOK, gin.H{
			"code": 20000,
			"data": gin.H{
				"token": token,
			},
		})
	} else {
		// 密码验证失败
		c.JSON(http.StatusOK, gin.H{
			"code":    60000,
			"message": "账号密码错误",
		})
	}
}

func userInfo(c *gin.Context) {
	token := c.Query("token")
	fmt.Println(token)

	if token == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":  40001,
			"error": "未登录，权限被拒绝",
		})
	}

	var user User
	if err := DB.Where("username = ?", token).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":  40001,
			"error": "用户不存在",
		})
		return
	}

	fmt.Println("用户存在：", user.Username, user.Role)

	c.JSON(http.StatusOK, gin.H{
		"code": 20000,
		"data": gin.H{
			"roles":        []string{user.Role},
			"introduction": "Hello i'm" + token,
			"avatar":       "https://wpimg.wallstcn.com/f778738c-e4f8-4870-b634-56703b4acafe.gif",
			"name":         token,
		},
	})
}

func userLogout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    20000,
		"message": "success",
	})
}

func userCreate(c *gin.Context) {
	var user User
	username := c.PostForm("username")
	password := c.PostForm("password")
	role := c.PostForm("role")
	if role == "" {
		role = "user"
	}

	if username == "" || password == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":    40001,
			"message": "账户名，或密码，不能为空",
		})
		return
	}

	// 加密密码
	user.PasswordHash = user.SetPassword(password)
	user.Username = username
	user.Password = password
	user.Role = role

	// 创建用户
	if err := user.CreateUser(DB, user); err != nil {
		if err.Error() == "user already exists" {
			c.JSON(http.StatusOK, gin.H{"code": 40001, "message": "用户已存在"})
		} else {
			c.JSON(http.StatusOK, gin.H{"code": 40002, "message": "创建用户失败", "detail": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    20000,
		"message": "用户创建成功",
	})
}

func userDelete(c *gin.Context) {
	username := c.PostForm("username")
	if username == "" {
		c.JSON(http.StatusOK, gin.H{"code": 40001, "message": "用户名不能为空"})
		return
	}

	// 使用用户名来删除用户
	result := DB.Where("username = ?", username).Delete(&User{})
	if result.Error != nil {
		c.JSON(http.StatusOK, gin.H{"code": 40001, "message": "删除用户失败:", "detail": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 40001, "message": "用不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    20000,
		"message": "删除用户成功",
	})
}

func userReset(c *gin.Context) {
	var user User
	username := c.PostForm("username")
	password := c.PostForm("password")

	fmt.Println(username, password)

	if username == "" {
		c.JSON(http.StatusOK, gin.H{"error": "用户名不能为空"})
		return
	}

	// 进行登录验证逻辑...
	dbUser := new(User)
	if err := DB.Where("username = ?", username).First(dbUser).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 60000, "message": "用户不存在"})
		return
	}

	dbUser.Username = username
	dbUser.Password = password
	if dbUser.ResetPassword(*dbUser) {
		// 更新登陆时间
		now := time.Now()
		location, _ := time.LoadLocation("Asia/Shanghai")
		nowInChina := now.In(location)

		DB.Model(&User{}).Where("username = ?", user.Username).Update("LastLoginTime", nowInChina)

		c.JSON(http.StatusOK, gin.H{
			"code":    20000,
			"message": "修改密码成功",
		})
	} else {
		// 密码验证失败
		c.JSON(http.StatusOK, gin.H{
			"code":    60000,
			"message": "账号密码错误",
		})
	}
}

func userList(c *gin.Context) {
	var user []User
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":  20000,
			"error": "未登录，权限被拒绝",
		})
		return
	}

	DB.Select("username", "CreatedAt", "LastLoginTime").Find(&user)

	c.JSON(http.StatusOK, gin.H{
		"code": 20000,
		"data": gin.H{
			"items": user,
		},
	})
}
