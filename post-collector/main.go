package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
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

	// err = saveLatestPosts(db)
	// if err != nil {
	// 	panic("error in saving posts: " + err.Error())
	// }

	err = savePostDetails(db)
	if err != nil {
		panic("error in saving posts: " + err.Error())
	}
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

func savePostDetails(db *gorm.DB) error {
	baseUrl := "https://api.divar.ir/v8/posts-v2/web/"

	postsToFetchDetails, err := fetchPostsWithoutDetail(db)
	if err != nil {
		return err
	}

	for _, item := range postsToFetchDetails {
		requestUrl := baseUrl + item.Token

		response, err := http.Get(requestUrl)
		if err != nil {
			return err
		}

		if response.StatusCode == 429 {
			fmt.Println("too many request")
		}

		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}

		var result models.PostDetailResponse
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			continue
		}

		if len(result.Sections) > 0 {
			sectionMap := map[string]*models.Section{}

			for _, section := range result.Sections {
				sectionMap[section.SectionName] = section
			}

			detailToSave := &models.PostDetail{
				Token:  item.Token,
				Title:  item.Title,
				Region: trimAfterLastPattern(item.BottomDescriptionText, "در "),
			}

			listDataSection := sectionMap["LIST_DATA"]

			listDataWidgetTypeMap := map[string]*models.Widget{}

			for _, widget := range listDataSection.Widgets {
				listDataWidgetTypeMap[widget.WidgetType] = widget

				if widget.Data.Title == "اجارهٔ ماهانه" {
					if widget.Data.Value == "مجانی" {
						detailToSave.Rent = decimal.Zero
					} else if widget.Data.Value == "توافقی" {
						detailToSave.Mortage = decimal.NewFromInt(-1)
					} else {
						number, err := parsePersianNumberToDecimal(widget.Data.Value)
						if err != nil {
							return err
						}
						detailToSave.Rent = number
					}
				}

				if widget.Data.Title == "ودیعه" {
					if widget.Data.Value == "مجانی" {
						detailToSave.Mortage = decimal.Zero
					} else if widget.Data.Value == "توافقی" {
						detailToSave.Mortage = decimal.NewFromInt(-1)
					} else {
						number, err := parsePersianNumberToDecimal(widget.Data.Value)
						if err != nil {
							return err
						}
						detailToSave.Mortage = number
					}
				}

			}

			groupFeatureRow := listDataWidgetTypeMap["GROUP_FEATURE_ROW"]

			for _, item := range groupFeatureRow.Data.Items {
				isElevator := strings.Contains(item.Title, "آسانسور")
				isParking := strings.Contains(item.Title, "پارکینگ")
				isWarehouse := strings.Contains(item.Title, "انباری")

				if isElevator {
					detailToSave.Elevator = item.Available
				}

				if isParking {
					detailToSave.Parking = item.Available
				}

				if isWarehouse {
					detailToSave.Warehouse = item.Available
				}
			}

			groupInfoRow := listDataWidgetTypeMap["GROUP_INFO_ROW"]

			for _, item := range groupInfoRow.Data.Items {
				isMeterage := strings.Contains(item.Title, "متراژ")
				isConstructionYear := strings.Contains(item.Title, "ساخت")
				isNumberOfRooms := strings.Contains(item.Title, "اتاق")

				englishDigits := convertPersianToEnglishDigits(item.Value)

				if isMeterage {
					valueAsDecimal, err := decimal.NewFromString(englishDigits)
					if err != nil {
						return err
					}

					detailToSave.Meterage = valueAsDecimal
				}

				if isConstructionYear {
					valueAsInt, err := strconv.Atoi(englishDigits)
					if err != nil {
						return err
					}

					detailToSave.ConstructionYear = valueAsInt
				}

				if isNumberOfRooms {
					if item.Value == "بدون اتاق" {
						detailToSave.Rooms = 0
					} else {
						valueAsInt, err := strconv.Atoi(englishDigits)
						if err != nil {
							return err
						}

						detailToSave.Rooms = valueAsInt
					}
				}
			}

			if detailToSave.Mortage.Equal(decimal.Zero) && detailToSave.Rent.Equal(decimal.Zero) {
				rentSliderWidget, found := listDataWidgetTypeMap["RENT_SLIDER"]
				if !found {
					fmt.Print("no rent and credit found for token" + detailToSave.Token)
				}

				if rentSliderWidget.Data.Credit != nil {
					if rentSliderWidget.Data.Credit.Value != "" {
						credit, err := decimal.NewFromString(rentSliderWidget.Data.Credit.Value)
						if err != nil {
							return err
						}

						detailToSave.Mortage = credit
					}
				}

				if rentSliderWidget.Data.Rent != nil {
					if rentSliderWidget.Data.Rent.Value != "" {
						rent, err := decimal.NewFromString(rentSliderWidget.Data.Rent.Value)
						if err != nil {
							return err
						}

						detailToSave.Rent = rent
					}
				}
			}

			err = db.Save(detailToSave).Error
			if err != nil {
				return err
			}

		} else {
			err = db.Exec(`
				DELETE FROM posts
				WHERE token = ?
			`, item.Token).Error
			if err != nil {
				return err
			}
		}

		time.Sleep(1 * time.Second)
	}

	return nil
}

func fetchPostsWithoutDetail(db *gorm.DB) ([]*models.Post, error) {
	postsToFetchDetails := []*models.Post{}

	err := db.Raw(`
		SELECT post.* FROM posts post
		LEFT JOIN post_details detail ON detail.token = post.token
		WHERE detail.token IS NULL
	`).Scan(&postsToFetchDetails).Error
	if err != nil {
		return nil, err
	}

	return postsToFetchDetails, err
}

func trimAfterLastPattern(s, pattern string) string {
	lengthofPattern := len(pattern)
	idx := strings.LastIndex(s, pattern)
	if idx == -1 {
		return s
	}
	return s[idx+lengthofPattern:]
}
