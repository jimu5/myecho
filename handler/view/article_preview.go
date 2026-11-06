package view

import (
	"myecho/dal/mysql"
	"myecho/model"
	"myecho/service"

	"github.com/gofiber/fiber/v2"
)

const articlePreviewCSP = "sandbox; default-src 'none'; img-src 'self' http: https: data: blob:; style-src 'self' http: https: 'unsafe-inline'; font-src 'self' http: https: data:; media-src 'self' http: https: data: blob:; script-src 'none'; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'"

func ArticlePreview(c *fiber.Ctx) error {
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Robots-Tag", "noindex, nofollow")
	c.Set("Referrer-Policy", "no-referrer")
	c.Set("Content-Security-Policy", articlePreviewCSP)

	page, err := service.LoadArticlePreview(c.Params("token"))
	if err != nil {
		return NotFound(c)
	}

	// Shared article rendering selects the active site theme. Ignore the
	// editor-only theme preview cookie for a publicly shareable article link.
	c.Request().Header.DelCookie(service.ThemePreviewCookieName)
	article := articleModelFromPreviewSnapshot(page.Snapshot)
	return renderArticlePage(c, article, &page.ExpiresAt)
}

func articleModelFromPreviewSnapshot(snapshot model.ArticlePreviewSnapshot) mysql.ArticleModel {
	article := mysql.ArticleModel{
		UID:            snapshot.UID,
		Title:          snapshot.Title,
		Slug:           snapshot.Slug,
		SEOTitle:       snapshot.SEOTitle,
		SEODescription: snapshot.SEODescription,
		ShareImage:     snapshot.ShareImage,
		Type:           snapshot.Type,
		ContentFormat:  snapshot.ContentFormat,
		Summary:        snapshot.Summary,
		CategoryUID:    snapshot.CategoryUID,
		IsAllowComment: snapshot.IsAllowComment,
		PostTime:       snapshot.PostTime,
		ReadCount:      snapshot.ReadCount,
		Detail:         &model.ArticleDetail{Content: snapshot.Content},
		Tags:           make([]*model.Tag, 0, len(snapshot.Tags)),
	}
	article.UpdatedAt = snapshot.UpdatedAt
	if snapshot.CategoryUID != "" || snapshot.CategoryName != "" {
		article.Category = &model.Category{UID: snapshot.CategoryUID, Name: snapshot.CategoryName}
	}
	for _, tag := range snapshot.Tags {
		article.Tags = append(article.Tags, &model.Tag{UID: tag.UID, Name: tag.Name})
	}
	if snapshot.AuthorName != "" || snapshot.AuthorNickName != "" || snapshot.AuthorSlug != "" || snapshot.AuthorAvatar != "" {
		nickName := snapshot.AuthorNickName
		if nickName == "" {
			nickName = snapshot.AuthorName
		}
		article.Author = &model.User{
			Name:      snapshot.AuthorName,
			NickName:  nickName,
			Slug:      snapshot.AuthorSlug,
			AvatarURL: snapshot.AuthorAvatar,
		}
	}
	return article
}
