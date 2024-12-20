package main

import (
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"time"
)

var DB *gorm.DB
var ERR error

func init() {
	// 设置时区为上海时区
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		log.Fatal("failed to load location: ", err)
	}
	now := time.Now().In(loc) // 获取当前上海时间
	log.Println("Current time in Shanghai:", now)
	time.Local = loc

	// 初始化数据库
	dsn := "gorm.db?parseTime=true&loc=Asia%2FShanghai"
	DB, ERR = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
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

	if err := router.Run(":8081"); err != nil {
		log.Fatal(err.Error())
	}

}
