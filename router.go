package main

import "github.com/gin-gonic/gin"

func SetupRoutes(router *gin.Engine) {

	// 用户路由组
	user := router.Group("/api/user")
	{
		user.POST("/login", userLogin)
		user.GET("/info", userInfo)
		user.POST("/logout", userLogout)
		user.POST("/create", userCreate)
		user.DELETE("/delete", userDelete)
		user.GET("/list", userList)
		user.POST("/reset", userReset)
	}

	// 白名单路由组
	whiteList := router.Group("/api/whitelist")
	{
		whiteList.POST("/add", whitelistAdd)
		whiteList.DELETE("/delete", whitelistDelete)
	}

	// 白名单日志路由组
	whiteListLog := router.Group("/api/whitelistlog")
	{
		whiteListLog.GET("/list", whitelistLogList)
	}
}
