package validator

import (
	"errors"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	apierrors "myecho/handler/api/errors"
	"myecho/handler/rtype"
	"myecho/model"
	"testing"
	"time"
)

func TestValidateLoginRequest(t *testing.T) {
	tests := []struct {
		name string
		req  rtype.LoginRequest
		want error
	}{
		{name: "missing identity", req: rtype.LoginRequest{Password: "p"}, want: apierrors.ErrLoginEmailOrNameEmpty},
		{name: "missing password", req: rtype.LoginRequest{Name: "admin"}, want: apierrors.ErrPasswordEmpty},
		{name: "ok by name", req: rtype.LoginRequest{Name: "admin", Password: "p"}},
		{name: "ok by email", req: rtype.LoginRequest{Email: "a@example.com", Password: "p"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLoginRequest(&tt.req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateLoginRequest() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestValidateRegisterRequestRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		req  rtype.RegisterRequest
		want error
	}{
		{name: "missing name", req: rtype.RegisterRequest{Email: "a@example.com", Password: "p"}, want: apierrors.ErrNameEmpty},
		{name: "missing email", req: rtype.RegisterRequest{Name: "admin", Password: "p"}, want: apierrors.ErrEmailEmpty},
		{name: "missing password", req: rtype.RegisterRequest{Name: "admin", Email: "a@example.com"}, want: apierrors.ErrPasswordEmpty},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRegisterRequest(&tt.req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateRegisterRequest() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestValidateCommentRequestRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		req  rtype.CommentRequest
		want error
	}{
		{name: "missing author", req: rtype.CommentRequest{AuthorEmail: "a@example.com", Content: "hello"}, want: apierrors.ErrCommentAuthorNameEmpty},
		{name: "blank author", req: rtype.CommentRequest{AuthorName: " \t ", AuthorEmail: "a@example.com", Content: "hello"}, want: apierrors.ErrCommentAuthorNameEmpty},
		{name: "missing email", req: rtype.CommentRequest{AuthorName: "alice", Content: "hello"}, want: apierrors.ErrCommentAuthorEmailEmpty},
		{name: "blank email", req: rtype.CommentRequest{AuthorName: "alice", AuthorEmail: " \n ", Content: "hello"}, want: apierrors.ErrCommentAuthorEmailEmpty},
		{name: "missing content", req: rtype.CommentRequest{AuthorName: "alice", AuthorEmail: "a@example.com"}, want: apierrors.ErrCommentContentEmpty},
		{name: "blank content", req: rtype.CommentRequest{AuthorName: "alice", AuthorEmail: "a@example.com", Content: " \n\t "}, want: apierrors.ErrCommentContentEmpty},
		{name: "ok without parent", req: rtype.CommentRequest{AuthorName: "alice", AuthorEmail: "a@example.com", Content: "hello"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommentRequest(&tt.req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateCommentRequest() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestValidateArticleRequestRequiredFields(t *testing.T) {
	if err := ValidateArticleRequest(&rtype.ArticleRequest{Content: "body"}); !errors.Is(err, apierrors.ErrTitleEmpty) {
		t.Fatalf("ValidateArticleRequest() error = %v, want ErrTitleEmpty", err)
	}
	if err := ValidateArticleRequest(&rtype.ArticleRequest{Title: "title"}); !errors.Is(err, apierrors.ErrContentEmpty) {
		t.Fatalf("ValidateArticleRequest() error = %v, want ErrContentEmpty", err)
	}
}

func TestValidateTagAndCategoryEmptyInputs(t *testing.T) {
	if err := ValidateTagIDs(nil); err != nil {
		t.Fatalf("ValidateTagIDs(nil) error = %v", err)
	}
	if err := ValidateTagUIDs(nil); err != nil {
		t.Fatalf("ValidateTagUIDs(nil) error = %v", err)
	}
	if err := ValidateCategoryUID(""); err != nil {
		t.Fatalf("ValidateCategoryUID(empty) error = %v", err)
	}
	if err := ValidateParentCommentID(0); err != nil {
		t.Fatalf("ValidateParentCommentID(0) error = %v", err)
	}
}

func setupValidatorTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Category{}, &model.User{}, &model.Tag{}, &model.Article{}, &model.ArticleDetail{}, &model.Comment{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	connect.Database = db
	mysql.InitDB()
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestValidateRegisterRequestDatabasePaths(t *testing.T) {
	setupValidatorTestDB(t)
	req := &rtype.RegisterRequest{Name: "admin", Email: "admin@example.com", Password: "secret"}
	if err := ValidateRegisterRequest(req); err != nil {
		t.Fatalf("ValidateRegisterRequest() error = %v", err)
	}
	if req.NickName != req.Name {
		t.Fatalf("NickName = %q, want name fallback", req.NickName)
	}
	if err := connect.Database.Create(&model.User{Name: req.Name, Email: req.Email}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := ValidateRegisterRequest(req); !errors.Is(err, apierrors.ErrUserExisted) {
		t.Fatalf("duplicate ValidateRegisterRequest() error = %v, want ErrUserExisted", err)
	}
}

func TestValidateTagCategoryAndArticleDatabasePaths(t *testing.T) {
	setupValidatorTestDB(t)
	tag := &model.Tag{Name: "go", UID: "tag-go"}
	if err := connect.Database.Create(tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	category := &mysql.CategoryModel{Name: "Tech", UID: "cat-tech", Type: model.CategoryTypeArticle}
	if err := connect.Database.Create(category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}

	if err := ValidateTagIDs([]uint{tag.ID}); err != nil {
		t.Fatalf("ValidateTagIDs(valid) error = %v", err)
	}
	if err := ValidateTagIDs([]uint{tag.ID, tag.ID + 100}); !errors.Is(err, apierrors.ErrTagNotFound) {
		t.Fatalf("ValidateTagIDs(invalid) error = %v, want ErrTagNotFound", err)
	}
	if err := ValidateTagUIDs([]string{tag.UID}); err != nil {
		t.Fatalf("ValidateTagUIDs(valid) error = %v", err)
	}
	if err := ValidateTagUIDs([]string{"missing"}); !errors.Is(err, apierrors.ErrTagNotFound) {
		t.Fatalf("ValidateTagUIDs(invalid) error = %v, want ErrTagNotFound", err)
	}
	if err := ValidateTagRequest(&rtype.TagRequest{Name: "go"}); !errors.Is(err, apierrors.ErrTagNameExist) {
		t.Fatalf("ValidateTagRequest(duplicate) error = %v, want ErrTagNameExist", err)
	}
	if err := ValidateTagRequest(&rtype.TagRequest{Name: "new"}); err != nil {
		t.Fatalf("ValidateTagRequest(new) error = %v", err)
	}

	if err := ValidateCategoryID(category.ID); err != nil {
		t.Fatalf("ValidateCategoryID(valid) error = %v", err)
	}
	if err := ValidateCategoryID(0); err != nil {
		t.Fatalf("ValidateCategoryID(0) error = %v", err)
	}
	if err := ValidateCategoryID(category.ID + 100); !errors.Is(err, apierrors.ErrCategoryNotFound) {
		t.Fatalf("ValidateCategoryID(missing) error = %v, want ErrCategoryNotFound", err)
	}
	if err := ValidateCategoryUID(category.UID); err != nil {
		t.Fatalf("ValidateCategoryUID(valid) error = %v", err)
	}
	if err := ValidateCategoryUID("missing"); !errors.Is(err, apierrors.ErrCategoryNotFound) {
		t.Fatalf("ValidateCategoryUID(missing) error = %v, want ErrCategoryNotFound", err)
	}
	name := "Updated"
	father := category.UID
	if err := ValidateCategoryUpdate(&rtype.CategoryUpdateRequest{Name: &name, FatherUID: &father}); err != nil {
		t.Fatalf("ValidateCategoryUpdate() error = %v", err)
	}
	emptyName := ""
	if err := ValidateCategoryUpdate(&rtype.CategoryUpdateRequest{Name: &emptyName}); !errors.Is(err, apierrors.ErrCategoryNameEmpty) {
		t.Fatalf("ValidateCategoryUpdate(empty name) error = %v, want ErrCategoryNameEmpty", err)
	}

	articleReq := &rtype.ArticleRequest{Title: "Title", Content: "Body", CategoryUID: category.UID, TagUIDs: []string{tag.UID}}
	if err := ValidateArticleRequest(articleReq); err != nil {
		t.Fatalf("ValidateArticleRequest(valid) error = %v", err)
	}
	if articleReq.PostTime.IsZero() || time.Since(articleReq.PostTime) > time.Minute {
		t.Fatalf("PostTime was not defaulted correctly: %v", articleReq.PostTime)
	}
	var loaded model.Category
	if err := ValidateID(int(category.ID), &loaded); err != nil {
		t.Fatalf("ValidateID(valid) error = %v", err)
	}
	if err := ValidateID(int(category.ID+100), &loaded); !errors.Is(err, apierrors.ErrorIDNotFound) {
		t.Fatalf("ValidateID(invalid) error = %v, want ErrorIDNotFound", err)
	}
}

func TestValidateCommentDatabasePaths(t *testing.T) {
	setupValidatorTestDB(t)
	article := &model.Article{UID: "article", Title: "Article"}
	if err := connect.Database.Create(article).Error; err != nil {
		t.Fatalf("create article: %v", err)
	}
	comment := &model.Comment{ArticleUID: article.UID, AuthorName: "alice", AuthorEmail: "a@example.com", Content: "hello"}
	if err := connect.Database.Create(comment).Error; err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if err := ValidateArticleID(article.ID); err != nil {
		t.Fatalf("ValidateArticleID(valid) error = %v", err)
	}
	if err := ValidateArticleID(article.ID + 100); !errors.Is(err, apierrors.ErrArticleID) {
		t.Fatalf("ValidateArticleID(invalid) error = %v, want ErrArticleID", err)
	}
	if err := ValidateParentCommentID(comment.ID); err != nil {
		t.Fatalf("ValidateParentCommentID(valid) error = %v", err)
	}
	if err := ValidateParentCommentID(comment.ID + 100); !errors.Is(err, apierrors.ErrParentCommentID) {
		t.Fatalf("ValidateParentCommentID(invalid) error = %v, want ErrParentCommentID", err)
	}
}
