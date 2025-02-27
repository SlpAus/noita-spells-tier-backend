package main

import (
	"encoding/json"
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

func main() {
	// 初始化数据库
	exceptionList := []uint{626, 627, 238, 239, 550, 551, 668, 633, 327, 328}

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
func contains(slice []uint, element uint) bool {
	for _, v := range slice {
		if v == element {
			return true
		}
	}
	return false
}
