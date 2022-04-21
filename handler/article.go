package handler

import (
	"time"

	"github.com/Kimiato/myecho/config"
	"github.com/Kimiato/myecho/model"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm/clause"
)

type ArticleRequest struct {
	Title          string    `json:"title"`
	Summary        string    `json:"summary"`
	Content        string    `json:"content"`
	CategoryID     uint      `json:"category_id"`
	IsAllowComment *bool     `json:"is_allow_comment"`
	PostTime       time.Time `json:"post_time"`
}

type User struct {
	ID       uint   `json:"id"`
	NickName string `json:"nick_name"`
}

type Category struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ArticleResponse struct {
	model.BaseModel
	AuthorID       uint                 `json:"-"`
	Author         *User                `json:"author" gorm:"foreignkey:author_id"`
	Title          string               `json:"title"`
	Summary        string               `json:"summary"`
	DetailID       uint                 `json:"-"`
	Detail         *model.ArticleDetail `json:"detail"`
	CategoryID     uint                 `json:"-"`
	Category       *Category            `json:"category"`
	IsAllowComment *bool                `json:"is_allow_comment"`
	ReadCount      uint                 `json:"read_count"`
	LikeCount      int                  `json:"like_count"`
	CommentCount   uint                 `json:"comment_count"`
	PostTime       time.Time            `json:"post_time"`
}

func ArticleList(c *fiber.Ctx) error {
	var articlesRes []ArticleResponse
	var total int64
	// 总数
	config.Database.Find(&[]model.Article{}).Count(&total)
	// 分页查询
	config.Database.Table("articles").Scopes(Paginate(c)).Preload(clause.Associations).Find(&articlesRes)
	return PaginateData(c, total, &articlesRes)
}

func ArticleRetrieve(c *fiber.Ctx) error {
	var article model.Article
	if err := ValidateID(c, &article); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	var res ArticleResponse
	res.ID = article.ID
	// TODO: 这里可以优化，减少一次sql查询
	config.Database.Table("articles").Preload(clause.Associations).Find(&res)
	return c.JSON(&res)
}

func ArticleCreate(c *fiber.Ctx) error {
	var article model.Article
	var detail model.ArticleDetail
	var res ArticleResponse
	var r ArticleRequest
	if err := c.BodyParser(&r); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	// 校验
	err := validateArticleRequest(&r)
	if err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	detail.Content = r.Content
	structAssign(&article, &r)
	article.Detail = &detail
	article.AuthorID = c.Locals("user").(*model.User).ID
	article.Author = c.Locals("user").(*model.User)
	config.Database.Create(&article).Scan(&res)
	return c.Status(201).JSON(res)
}

// 更新文章
func ArticleUpdate(c *fiber.Ctx) error {
	var article model.Article
	var r ArticleRequest
	if err := c.BodyParser(&r); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	// 校验
	if err := ValidateID(c, &article); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	if err := validateArticleRequest(&r); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}

	var detail model.ArticleDetail
	if result := config.Database.Where("id = ?", article.DetailID).First(&detail); result.Error != nil {
		return InternalErrorResponse(c, InternalSQLError, result.Error.Error())
	}
	detail.Content = r.Content
	if result := config.Database.Save(&detail); result.Error != nil {
		return InternalErrorResponse(c, InternalSQLError, result.Error.Error())
	}

	config.Database.Model(&article).Omit("User").Updates(structAssign(&article, &r))

	var res ArticleResponse
	config.Database.Table("articles").Preload(clause.Associations).Find(&res, article.ID)
	return c.JSON(&res)
}

// 删除文章
func ArticleDelete(c *fiber.Ctx) error {
	var article model.Article
	if err := ValidateID(c, &article); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	config.Database.Select("Detail").Delete(&article)
	return c.SendStatus(204)
}

func validateCategoryID(categoryID uint) error {
	if categoryID == 0 {
		return nil
	}
	err := config.Database.Where("id = ?", categoryID).First(&model.Category{}).Error
	if err != nil {
		return ErrCategoryNotFound
	}
	return nil
}

func validateArticleRequest(articleRequest *ArticleRequest) error {
	if len(articleRequest.Title) == 0 {
		return ErrTitleEmpty
	}
	if len(articleRequest.Content) == 0 {
		return ErrContentEmpty
	}
	if articleRequest.PostTime.IsZero() {
		articleRequest.PostTime = time.Now()
	}
	if err := validateCategoryID(articleRequest.CategoryID); err != nil {
		return err
	}
	return nil
}
