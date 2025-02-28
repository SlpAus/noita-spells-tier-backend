package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/Qiuarctica/isaac-ranking-backend/database"
	"github.com/Qiuarctica/isaac-ranking-backend/models"
)

type ItemInfo struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Quality    interface{} `json:"quality"`
	Lost       bool        `json:"lost"`
	Descrption string      `json:"description"`
}

type ItemPools struct {
	Pools []PoolXML `xml:"Pool"`
}
type PoolXML struct {
	Name  string    `xml:"Name,attr"`
	Items []ItemXML `xml:"Item"`
}

type ItemXML struct {
	ID         uint    `xml:"Id,attr"`         // 将 Id 属性映射到 ID 字段
	Weight     float64 `xml:"Weight,attr"`     // 将 Weight 属性映射到 Weight 字段
	DecreaseBy float64 `xml:"DecreaseBy,attr"` // 将 DecreaseBy 属性映射到 DecreaseBy 字段
	RemoveOn   float64 `xml:"RemoveOn,attr"`   // 将 RemoveOn 属性映射到 RemoveOn 字段
}

func main() {
	// update()
	// Print()
	clean()
}

// convertRomanToArabic 将罗马数字转换为阿拉伯数字
func convertRomanToArabic(roman string) string {
	romanToArabic := map[string]string{
		"I":   "1",
		"II":  "2",
		"III": "3",
		"IV":  "4",
	}

	return romanToArabic[strings.ToUpper(roman)]
}

// contains 检查切片中是否包含指定元素
func contains[T comparable](slice []T, element T) bool {
	for _, v := range slice {
		if v == element {
			return true
		}
	}
	return false
}

func find(slice []models.Item, ID uint) int {
	for i, v := range slice {
		if v.ItemID == ID {
			return i
		}
	}
	return -1
}

// 构建道具数据库的描述,url，品质等信息
func build() {
	// 初始化数据库
	exceptionList := []uint{626, 627, 238, 239, 550, 551, 668, 633, 327, 328, 714, 715, 648, 474}

	database.InitDB()

	// 读取 items.json 文件
	jsonFile, err := os.Open("./items.json")
	if err != nil {
		log.Fatal(err)
	}
	defer jsonFile.Close()

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		log.Fatal(err)
	}

	var items []ItemInfo
	if err := json.Unmarshal(byteValue, &items); err != nil {
		log.Fatal(err)
	}
	// 读取 assets/items 目录下的图片名称
	dir := "./assets/items"
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	// 正则表达式匹配图片文件名中的道具ID
	re := regexp.MustCompile(`collectibles_(\d+)_([\w]+)\.png`)

	for _, file := range files {
		if file.IsDir() || file.Name() == "placeholder.png" || file.Name() == "questionmark.png" {
			continue
		}

		matches := re.FindStringSubmatch(file.Name())
		if len(matches) != 3 {
			continue
		}
		var ItemID uint
		// 把字符串转换为数字
		fmt.Sscanf(matches[1], "%d", &ItemID)

		// 如果道具ID在例外列表中，则跳过
		if contains(exceptionList, ItemID) {
			fmt.Println("道具ID:", ItemID, "在例外列表中，跳过")
			continue
		}

		itemID := "c" + matches[1]

		// 查找对应的道具信息
		var itemInfo *ItemInfo
		for _, item := range items {
			if item.ID == itemID {
				itemInfo = &item
				break
			}
		}

		if itemInfo == nil {
			log.Printf("未找到道具ID为 %s 的信息\n", itemID)
			continue
		}

		// 将 Quality 转换为字符串
		qualityStr := fmt.Sprintf("%v", itemInfo.Quality)
		qualityStr = convertRomanToArabic(qualityStr)
		//转换为数字
		var quality uint
		fmt.Sscanf(qualityStr, "%d", &quality)

		// 插入数据库
		var Item models.Item
		Item.ItemID = ItemID
		Item.Name = itemInfo.Name
		Item.Url = "/assets/items/" + file.Name()
		Item.Quality = quality
		Item.Score = 0
		Item.Total = 0
		Item.WinRate = 0
		Item.Lost = itemInfo.Lost
		Item.Descrption = itemInfo.Descrption
		fmt.Println("道具ID:", Item.ItemID, "名称:", Item.Name, "图片URL:", Item.Url, "品质:", Item.Quality, "是否能被Lost获取:", Item.Lost, "描述:", Item.Descrption)
		if err := database.DB.Create(&Item).Error; err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println("数据库构建完成！")
}

// 更新数据库：包括以下：更新rankdb,从道具中删除例外道具，添加胜利总场次信息，添加道具池信息，增加道具池数据库
func update() {
	exceptionList := []uint{626, 627, 238, 239, 550, 551, 668, 633, 327, 328, 714, 715, 648, 474}
	needItemPools := []string{"treasure", "shop", "boss", "devil", "angel", "secret", "goldenChest", "redChest", "curse"}
	database.InitDB()

	tx := database.DB.Begin()

	// 读取ranking.db,从中删除例外道具，添加winCount信息
	var items []models.Item
	if err := tx.Find(&items).Error; err != nil {
		tx.Rollback()
		log.Fatal(err)
	}

	for _, item := range items {
		// 添加WinCount
		item.WinCount = item.Total * item.WinRate
		// 如果道具ID在例外列表中，则删除
		if contains(exceptionList, item.ItemID) {
			fmt.Println("删除道具ID:", item.ItemID, "名称:", item.Name)
			if err := tx.Delete(&item).Error; err != nil {
				tx.Rollback()
				log.Fatal(err)
			}

		}
	}

	// 增加道具池机制

	// 读取 itempools.xml 文件
	xmlFile, err := os.Open("./itempools.xml")
	if err != nil {
		tx.Rollback()
		log.Fatal(err)
	}
	defer xmlFile.Close()

	byteValue, err := io.ReadAll(xmlFile)
	if err != nil {
		tx.Rollback()
		log.Fatal(err)
	}

	var itemPools ItemPools
	if err := xml.Unmarshal(byteValue, &itemPools); err != nil {
		tx.Rollback()
		log.Fatal(err)
	}

	// 创建道具池数据库
	for _, poolXML := range itemPools.Pools {
		fmt.Println(poolXML.Name)
		if !contains(needItemPools, poolXML.Name) {
			continue
		}
		pool := models.Pool{Name: poolXML.Name}
		if err := tx.Create(&pool).Error; err != nil {
			tx.Rollback()
			log.Fatal(err)
		}

		for _, itemXML := range poolXML.Items {
			var item models.Item
			if err := tx.First(&item, "item_id = ?", itemXML.ID).Error; err != nil {
				fmt.Println("道具ID:", itemXML.ID, "未找到")
				continue
			}
			if err := tx.Model(&item).Association("Pools").Append(&pool); err != nil {
				tx.Rollback()
				log.Fatal(err)
			}
			fmt.Println("道具ID:", item.ItemID, "名称:", item.Name, "已添加到道具池:", pool.Name)
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		log.Fatal(err)
	}

	fmt.Println("数据库更新完成！")
}

// 打印部分道具池以检验正确性
func Print() {
	database.InitDB()
	var pool models.Pool
	if err := database.DB.Preload("Items").First(&pool, "name = ?", "treasure").Error; err != nil {
		log.Fatal(err)
	}
	fmt.Println(pool.Name)
	for _, item := range pool.Items {
		fmt.Println("道具ID:", item.ItemID, "名称:", item.Name)
	}

	// 打印几个道具

	var items []models.Item
	if err := database.DB.Preload("Pools").Limit(20).Find(&items).Error; err != nil {
		log.Fatal(err)
	}

	for _, item := range items {
		fmt.Println("道具ID:", item.ItemID, "名称:", item.Name, "图片URL:", item.Url, "品质:", item.Quality, "是否能被Lost获取:", item.Lost, "描述:", item.Descrption, "所在道具池:", item.Pools)
	}

}

// 清除item的 分数 场次 胜率等等
func clean() {
	database.InitDB()
	var items []models.Item
	if err := database.DB.Find(&items).Error; err != nil {
		log.Fatal(err)
	}

	for _, item := range items {
		item.Score = 0
		item.WinRate = 0
		item.WinCount = 0
		item.Total = 0
		if err := database.DB.Save(&item).Error; err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("清除完成！")

}
