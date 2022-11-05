package rtype

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
	"myecho/dal/mysql"
	"myecho/model"
	"myecho/utils"
	"time"
)

type ArticleDisplayListQueryParam struct {
	CategoryUID *string `query:"category_uid"`
}

type ArticleRequest struct {
	Title          string    `json:"title"`
	Summary        string    `json:"summary"`
	Content        string    `json:"content"`
	CategoryUID    string    `json:"category_uid"`
	IsAllowComment *bool     `json:"is_allow_comment"`
	PostTime       time.Time `json:"post_time"`
	Status         int8      `json:"status"`
	Visibility     int8      `json:"visibility"`
	Password       string    `json:"password"`
	TagUIDs        []string  `json:"tag_uids"`
}

func (a *ArticleRequest) SetSummary() {
	strByte := []byte(a.Content)
	originDoc := goldmark.DefaultParser().Parse(text.NewReader(strByte))
	parseContent := originDoc.Text(strByte)
	// 转换 rune 类型, 用于处理中文
	runeStr := []rune(string(parseContent))
	if len(runeStr) > 255 {
		a.Summary = string(runeStr[:255])
	} else {
		a.Summary = string(parseContent)
	}
}
func (a *ArticleRequest) PreHandle() {
	a.TagUIDs = utils.GetDuplicateSlice(a.TagUIDs)
}

type User struct {
	ID       uint   `json:"id"`
	NickName string `json:"nick_name"`
}

type Category struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type ArticleResponse struct {
	model.BaseModel
	AuthorID       uint                 `json:"-"`
	Author         *User                `json:"author"`
	Title          string               `json:"title"`
	Summary        string               `json:"summary"`
	DetailUID      string               `json:"-"`
	Detail         *model.ArticleDetail `json:"detail"`
	CategoryUID    string               `json:"category_uid"`
	Category       *Category            `json:"category"`
	IsAllowComment *bool                `json:"is_allow_comment"`
	ReadCount      uint                 `json:"read_count"`
	LikeCount      int                  `json:"like_count"`
	CommentCount   uint                 `json:"comment_count"`
	PostTime       time.Time            `json:"post_time"`
	Status         int8                 `json:"status"`
	Visibility     int8                 `json:"visibility"` // 1: 置顶 2: 公开 3: 私密
	Tags           []*model.Tag         `json:"tags" gorm:"many2many:article_tags;joinForeignKey:ArticleID"`
}

func ModelToUser(user *model.User) *User {
	if user == nil {
		return nil
	}
	return &User{
		ID:       user.ID,
		NickName: user.NickName,
	}
}

func ModelToCategory(category *model.Category) *Category {
	if category == nil {
		return nil
	}
	return &Category{
		ID:   category.ID,
		Name: category.Name,
	}
}

func ModelToArticleResponse(article *mysql.ArticleModel) *ArticleResponse {
	if article == nil {
		return nil
	}
	return &ArticleResponse{
		BaseModel:      article.BaseModel,
		AuthorID:       article.AuthorID,
		Author:         ModelToUser(article.Author),
		Title:          article.Title,
		Summary:        article.Summary,
		DetailUID:      article.DetailUID,
		Detail:         article.Detail,
		CategoryUID:    article.CategoryUID,
		Category:       ModelToCategory(article.Category),
		IsAllowComment: article.IsAllowComment,
		ReadCount:      article.ReadCount,
		LikeCount:      article.LikeCount,
		CommentCount:   article.CommentCount,
		PostTime:       article.PostTime,
		Status:         article.Status,
		Tags:           article.Tags,
	}
}

func MultiModelToArticleResponse(articles []*mysql.ArticleModel) []*ArticleResponse {
	result := make([]*ArticleResponse, 0, len(articles))
	for _, article := range articles {
		res := ModelToArticleResponse(article)
		result = append(result, res)
	}
	return result
}
