package database

import (
	"fmt"
	"log"

	"github.com/Qiuarctica/isaac-ranking-backend/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	var err error
	DB, err = gorm.Open(sqlite.Open("ranking.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("数据库连接失败:", err)
	}

	// 自动迁移（创建表）
	err = DB.AutoMigrate(&models.Item{})
	if err != nil {
		log.Fatal("数据表迁移失败:", err)
	}

	fmt.Println("SQLite 数据库初始化完成！")
}

func SeedDB() {
	var count int64
	DB.Model(&models.Item{}).Count(&count)
	if count > 0 {
		fmt.Println("数据库中已有数据，跳过插入测试数据")
		return
	}
	items := []models.Item{
		{ItemID: 1, Name: "Item1", Url: "https://pic3.zhimg.com/v2-87d78fc44236a144aa52cd8ea18e9da2_r.jpg", Quality: 1},
		{ItemID: 2, Name: "Item2", Url: "https://img.tukuppt.com/png_preview/02/93/58/OSUyEoAy3q.jpg!/fw/780", Quality: 2},
		{ItemID: 3, Name: "Item3", Url: "https://pic.616pic.com/ys_bnew_img/00/47/11/RstMvrKMRF.jpg", Quality: 3},
		{ItemID: 4, Name: "Item4", Url: "https://img.tukuppt.com/png_preview/02/93/89/HTSrGAGCdW.jpg!/fw/780", Quality: 4},
		{ItemID: 5, Name: "Item5", Url: "https://pic.616pic.com/ys_bnew_img/00/45/02/8RVugjOgB3.jpg", Quality: 0},
		{ItemID: 6, Name: "Item6", Url: "https://pic.616pic.com/ys_bnew_img/00/47/74/tkvN8VBwXD.jpg", Quality: 1},
		{ItemID: 7, Name: "Item7", Url: "https://pic.616pic.com/ys_bnew_img/00/47/74/tkvN8VBwXD.jpg", Quality: 2},
		{ItemID: 8, Name: "Item8", Url: "https://pic.616pic.com/ys_bnew_img/00/47/74/tkvN8VBwXD.jpg", Quality: 3},
		{ItemID: 9, Name: "Item9", Url: "https://pic.616pic.com/ys_bnew_img/00/47/74/tkvN8VBwXD.jpg", Quality: 4},
		{ItemID: 10, Name: "Item10", Url: "https://pic.616pic.com/ys_bnew_img/00/47/74/tkvN8VBwXD.jpg", Quality: 0},
	}

	for _, item := range items {
		DB.Create(&item)
	}

	fmt.Println("测试数据插入完成！")
}
