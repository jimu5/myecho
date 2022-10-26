package service

import (
	"fmt"
	"myecho/model"
	"testing"
)

func TestFillTotalCountCategories(t *testing.T) {
	modelCategories := []*model.Category{
		{UID: "A", Count: 8},
		{UID: "B", Count: 6, FatherUID: "A"},
		{UID: "C", Count: 5, FatherUID: "A"},
		{UID: "D", Count: 2, FatherUID: "C"},
		{UID: "E", Count: 3, FatherUID: "C"},
		{UID: "G", Count: 4, FatherUID: "C"},
		{UID: "F", Count: 1, FatherUID: "E"},
	}
	categories := make([]*Category, len(modelCategories))
	for i := range modelCategories {
		categories[i] = &Category{
			CategoryModel: *modelCategories[i],
		}
	}
	FillTotalCountCategories(categories)
	for _, c := range categories {
		fmt.Println(c.UID, c.TotalCount)
	}
}
