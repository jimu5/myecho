package service

import (
	"myecho/dal/mysql"
	"myecho/model"
	"testing"
)

func TestMysqlToServiceCategoryAndBuildChildMap(t *testing.T) {
	input := []*mysql.CategoryModel{
		{UID: "root", Count: 1},
		{UID: "child", FatherUID: "root", Count: 2},
		{UID: "orphan", FatherUID: "missing", Count: 3},
	}
	got := mysqlToServiceCategory(input)
	if len(got) != len(input) {
		t.Fatalf("len = %d, want %d", len(got), len(input))
	}
	if got[0].TotalCount != 3 || got[1].TotalCount != 2 || got[2].TotalCount != 3 {
		t.Fatalf("unexpected total counts: %+v", got)
	}

	children := BuildChildMap(got)
	if len(children["root"]) != 1 || children["root"][0].UID != "child" {
		t.Fatalf("children for root = %+v", children["root"])
	}
	if len(children["orphan"]) != 0 {
		t.Fatalf("orphan should not be attached to missing parent")
	}
}

func TestFillTotalCountCategoriesWithDeepTree(t *testing.T) {
	categories := []*Category{
		{CategoryModel: mysql.CategoryModel{UID: "a", Count: 1}},
		{CategoryModel: mysql.CategoryModel{UID: "b", FatherUID: "a", Count: 2}},
		{CategoryModel: mysql.CategoryModel{UID: "c", FatherUID: "b", Count: 3}},
		{CategoryModel: mysql.CategoryModel{UID: "d", FatherUID: "a", Count: 4}},
	}
	FillTotalCountCategories(categories)
	want := map[string]uint{"a": 10, "b": 5, "c": 3, "d": 4}
	for _, category := range categories {
		if category.TotalCount != want[category.UID] {
			t.Fatalf("%s TotalCount = %d, want %d", category.UID, category.TotalCount, want[category.UID])
		}
	}
}

func TestUpdateFileParamFillModelAndModelConversions(t *testing.T) {
	fileModel := &mysql.FileModel{
		BaseModel:     model.BaseModel{ID: 9},
		Name:          "old",
		ExtensionName: ".txt",
		UUID:          "file-uuid",
		MD5:           "md5",
		Note:          "old note",
	}
	(&UpdateFileParam{FullName: "report.txt", Note: "new note"}).FillModel(fileModel)
	if fileModel.Name != "report" || fileModel.ExtensionName != ".txt" || fileModel.Note != "new note" {
		t.Fatalf("FillModel() = %+v", fileModel)
	}

	dto := ModelToFile(fileModel)
	if dto.ID != fileModel.ID || dto.FullName != "report.txt" || dto.UUID != fileModel.UUID || dto.Note != fileModel.Note {
		t.Fatalf("ModelToFile() = %+v", dto)
	}

	files := MModelToFiles([]*mysql.FileModel{fileModel})
	if len(files) != 1 || files[0].FullName != "report.txt" {
		t.Fatalf("MModelToFiles() = %+v", files)
	}
}
