package view

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"myecho/dal"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/api"
	"myecho/handler/rtype"
	"myecho/model"
	"myecho/service"
	"myecho/utils"
	"net/url"
	"strings"
)

func ArticleDisplayList(c *fiber.Ctx) error {
	queryParam := service.ArticleDisplayListQueryParam{}
	if err := c.QueryParser(&queryParam); err != nil {
		return err
	}
	pageInfo, data, err := service.S.Article.ArticleDisplayList(&queryParam)
	if err != nil {
		return err
	}
	pageInfoResp := getPageInfoRespByMysqlPageInfo(c, &pageInfo)
	filterLabel := articleFilterLabel(c)
	pageTitle := "最近更新"
	if filterLabel != "" {
		pageTitle = "筛选结果"
	}
	return c.Render("index", respToMap(c, Pagination{PageInfo: pageInfoResp, PageData: data, FilterLabel: filterLabel}, PageMeta{
		Description: "最近更新的文章列表",
		Canonical:   absoluteURL(c),
		OGTitle:     pageTitle,
	}))
}

func articleFilterLabel(c *fiber.Ctx) string {
	filters := make([]string, 0, 5)
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		filters = append(filters, "关键词“"+keyword+"”")
	}
	if categoryUID := strings.TrimSpace(c.Query("category_uid")); categoryUID != "" {
		categoryName := strings.TrimSpace(c.Query("category_name"))
		if categoryName == "" {
			filters = append(filters, "所选分类")
		} else {
			filters = append(filters, "分类“"+categoryName+"”")
		}
	}
	if tagUID := strings.TrimSpace(c.Query("tag_uid")); tagUID != "" {
		tagName := strings.TrimSpace(c.Query("tag_name"))
		if tagName == "" {
			filters = append(filters, "所选标签")
		} else {
			filters = append(filters, "标签“"+tagName+"”")
		}
	}
	if dateFrom := strings.TrimSpace(c.Query("date_from")); dateFrom != "" {
		filters = append(filters, "从 "+dateFrom)
	}
	if dateTo := strings.TrimSpace(c.Query("date_to")); dateTo != "" {
		filters = append(filters, "到 "+dateTo)
	}
	return strings.Join(filters, " · ")
}

func ArticleRetrieve(c *fiber.Ctx) error {
	queryParam := service.ArticleRetrieveQueryParam{}
	if err := c.QueryParser(&queryParam); err != nil {
		return err
	}
	article := new(mysql.ArticleModel)
	if err := handler.DetailPreHandleByParam(c, &article); err != nil {
		return NotFound(c)
	}
	return renderArticle(c, article, &queryParam)
}

func PostRetrieveBySlug(c *fiber.Ctx) error {
	return retrieveBySlug(c, model.ArticleTypePost)
}

func PageRetrieveBySlug(c *fiber.Ctx) error {
	return retrieveBySlug(c, model.ArticleTypePage)
}

func retrieveBySlug(c *fiber.Ctx, articleType model.ArticleType) error {
	queryParam := service.ArticleRetrieveQueryParam{}
	if err := c.QueryParser(&queryParam); err != nil {
		return err
	}
	article, err := dal.MySqlDB.Article.FindBySlug(c.Params("slug"), articleType)
	if err != nil {
		target, redirectErr := dal.MySqlDB.Article.FindByRedirectSlug(c.Params("slug"), articleType)
		if redirectErr != nil || !service.IsArticlePubliclyVisible(target.Status, target.PostTime) {
			return NotFound(c)
		}
		return c.Redirect(articlePublicPath(target), fiber.StatusMovedPermanently)
	}
	return renderArticle(c, &article, &queryParam)
}

func renderArticle(c *fiber.Ctx, article *mysql.ArticleModel, queryParam *service.ArticleRetrieveQueryParam) error {
	queryParam.ID = article.ID
	queryParam.PasswordUnlocked = service.ValidateArticlePasswordToken(article, c.Cookies(service.ArticlePasswordCookieName(article.ID)))
	res, err := service.S.Article.ArticleRetrieve(queryParam)
	if err != nil {
		if errors.Is(err, service.ErrArticleNotDisplayable) {
			return NotFound(c)
		}
		if errors.Is(err, service.ErrArticlePasswordRequired) {
			return c.Status(fiber.StatusForbidden).Render("article_password", respToMap(c, *article, PageMeta{
				Description: articleMetaDescription(article),
				Canonical:   articleCanonicalURL(c, article),
				OGTitle:     articleMetaTitle(article),
				OGType:      "article",
				Image:       articleShareImageURL(c, article.ShareImage),
			}))
		}
		return err
	}
	if res.Detail != nil {
		content, err := renderArticleContent(res.Detail.Content, res.ContentFormat)
		if err != nil {
			return err
		}
		res.Detail.Content = content
	}
	comments, err := approvedComments(res.UID)
	if err != nil {
		return err
	}
	previousArticle, nextArticle, err := service.S.Article.PostNeighbors(&res)
	if err != nil {
		return err
	}
	relatedArticles, err := service.S.Article.RelatedPosts(&res)
	if err != nil {
		return err
	}
	data := respToMap(c, res, PageMeta{
		Description: articleMetaDescription(&res),
		Canonical:   articleCanonicalURL(c, &res),
		OGTitle:     articleMetaTitle(&res),
		OGType:      "article",
		Image:       articleShareImageURL(c, res.ShareImage),
		JSONLD:      articleJSONLD(c, &res),
	})
	data["Comments"] = comments
	data["IsAllowComment"] = isAllowComment(res.IsAllowComment)
	data["PreviousArticle"] = previousArticle
	data["NextArticle"] = nextArticle
	data["HasArticleNeighbors"] = previousArticle != nil || nextArticle != nil
	data["RelatedArticles"] = relatedArticles
	return c.Render("article", data)
}

func articleJSONLD(c *fiber.Ctx, article *mysql.ArticleModel) string {
	if article == nil {
		return ""
	}
	doc := map[string]interface{}{
		"@context":      "https://schema.org",
		"@type":         "BlogPosting",
		"headline":      articleMetaTitle(article),
		"description":   articleMetaDescription(article),
		"url":           articleCanonicalURL(c, article),
		"datePublished": article.PostTime.Format("2006-01-02T15:04:05Z07:00"),
		"dateModified":  article.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	image := articleShareImageURL(c, article.ShareImage)
	if image == "" {
		image = settingAbsoluteURL(c, "SiteShareImage")
	}
	if image != "" {
		doc["image"] = image
	}
	if article.Author != nil {
		name := article.Author.NickName
		if name == "" {
			name = article.Author.Name
		}
		if name != "" {
			doc["author"] = map[string]string{"@type": "Person", "name": name}
		}
	}
	body, _ := json.Marshal(doc)
	return string(body)
}

func articleMetaTitle(article *mysql.ArticleModel) string {
	if article != nil && strings.TrimSpace(article.SEOTitle) != "" {
		return strings.TrimSpace(article.SEOTitle)
	}
	if article == nil {
		return ""
	}
	return article.Title
}

func articleMetaDescription(article *mysql.ArticleModel) string {
	if article != nil && strings.TrimSpace(article.SEODescription) != "" {
		return strings.TrimSpace(article.SEODescription)
	}
	if article == nil {
		return ""
	}
	return article.Summary
}

func articleShareImageURL(c *fiber.Ctx, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	ref, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if ref.IsAbs() {
		if ref.Scheme == "http" || ref.Scheme == "https" {
			return ref.String()
		}
		return ""
	}
	base, err := url.Parse(siteBaseURL(c) + "/")
	if err != nil {
		return ""
	}
	return base.ResolveReference(ref).String()
}

func articleCanonicalURL(c *fiber.Ctx, article *mysql.ArticleModel) string {
	return siteBaseURL(c) + articlePublicPath(*article)
}

func renderArticleContent(content string, contentFormat model.ArticleContentFormat) (string, error) {
	if model.NormalizeArticleContentFormat(contentFormat) == model.ArticleContentFormatHTML {
		return content, nil
	}
	var buf bytes.Buffer
	if err := utils.MDParser.Convert([]byte(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func isAllowComment(value *bool) bool {
	return value == nil || *value
}

func approvedComments(articleUID string) ([]rtype.CommentResponse, error) {
	comments := make([]rtype.CommentResponse, 0)
	err := connect.Database.Table("comments").
		Where("article_uid = ?", articleUID).
		Where("status IS NULL OR status = ? OR status = ?", int8(model.CommentStatusLegacyApproved), int8(model.CommentStatusApproved)).
		Order("post_time asc, created_at asc").
		Find(&comments).Error
	if err != nil {
		return nil, err
	}
	return api.BuildCommentTree(comments), nil
}
