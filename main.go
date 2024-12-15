package main

import (
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
)

var DB *gorm.DB
var ERR error

func init() {
	// 初始化数据库
	DB, ERR = gorm.Open(sqlite.Open("gorm.db"), &gorm.Config{})
	if ERR != nil {
		log.Fatal(ERR.Error())
	}

	// 自动迁移模式
	ERR = DB.AutoMigrate(&User{}, &WhiteList{}, &WhitelistLog{})
	if ERR != nil {
		log.Fatal("failed to migrate database: ", ERR)
	}

}

func main() {

	router := gin.Default()
	router.Use(CORSMiddleware())

	SetupRoutes(router)

	if err := router.Run(":8080"); err != nil {
		log.Fatal(err.Error())
	}

}
