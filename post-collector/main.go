package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	"net/http"

	"github.com/Amir1848/ajarent/models"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file")
	}
}

func main() {

	db, err := getDB()
	if err != nil {
		panic("Error loading .env file")
	}

	saveLatestPosts(db)
}

func getDB() (*gorm.DB, error) {
	dsn := os.Getenv("DB_CONNECTION")

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("Failed to connect to the database:", err)
	}

	return db, nil
}

func saveLatestPosts(db *gorm.DB) error {
	pageNumber := 0

	for true {
		postList, err := fetchPostList(pageNumber)
		if err != nil {
			return errors.New("fetch posts failed: " + err.Error())
		}

		pageNumber++

		if postList == nil || len(postList.ListWidgets) == 0 {
			return nil
		}

		for _, item := range postList.ListWidgets {
			err = savePostToDB(db, item)
			if err != nil {
				return err
			}
		}

		time.Sleep(time.Second * 1)
	}

	return nil
}

func savePostToDB(db *gorm.DB, widget *models.Widget) error {
	shoudlFilter := strings.Contains(widget.Data.Title, "همخونه") ||
		strings.Contains(widget.Data.Title, "هم خانه") ||
		strings.Contains(widget.Data.Title, "خوابگاه")
	if shoudlFilter {
		return nil
	}

	result := models.Post{
		Title:                 widget.Data.Title,
		Token:                 widget.Data.Token,
		TopDescriptionText:    widget.Data.TopDescriptionText,
		MiddleDescriptionText: widget.Data.MiddleDescriptionText,
		BottomDescriptionText: widget.Data.BottomDescriptionText,
	}

	err := db.Save(&result).Error
	if err != nil {
		return err
	}

	return nil
}

func fetchPostList(pageNumber int) (*models.WidgetList, error) {
	url := "https://api.divar.ir/v8/postlist/w/search"

	data := map[string]interface{}{
		"city_ids":               []string{"1"},
		"disable_recommendation": true,
		"search_data": models.SearchData{
			FormData: models.FormData{
				Data: models.Data{
					Category: models.Category{
						Str: models.Str{
							Value: "apartment-rent",
						},
					},
				},
			},
		},
		"pagination_data": map[string]any{
			"layer_page": pageNumber,
			"page":       pageNumber,
			"@type":      "type.googleapis.com/post_list.PaginationData",
		},
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, errors.New("Error encoding JSON: " + err.Error())
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.New("Error sending request: " + err.Error())
	}

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result models.WidgetList
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func convertPersianToEnglishDigits(input string) string {
	var result strings.Builder
	for _, r := range input {
		if r >= '۰' && r <= '۹' {
			result.WriteRune(r - '۰' + '0')
		} else if unicode.IsDigit(r) || r == '.' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func parsePersianNumberToDecimal(input string) (decimal.Decimal, error) {
	cleaned := strings.ReplaceAll(input, "٬", "")

	cleaned = strings.ReplaceAll(cleaned, "تومان", "")

	cleaned = strings.TrimSpace(cleaned)

	englishDigits := convertPersianToEnglishDigits(cleaned)

	result, err := decimal.NewFromString(englishDigits)
	if err != nil {
		return decimal.Zero, err
	}

	return result, nil
}
