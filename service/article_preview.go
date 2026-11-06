package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"time"

	"gorm.io/gorm"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
)

const ArticlePreviewTTL = time.Hour

var ErrArticlePreviewUnavailable = errors.New("article preview is unavailable")

type ArticlePreviewLink struct {
	Token     string
	ExpiresAt time.Time
}

type ArticlePreviewPage struct {
	Snapshot  model.ArticlePreviewSnapshot
	ExpiresAt time.Time
}

// CreateArticlePreview freezes the actor's editable submission when present,
// otherwise the live article, without changing the article edit version.
func CreateArticlePreview(articleID, creatorID uint, expectedEditVersion uint64) (ArticlePreviewLink, error) {
	var link ArticlePreviewLink
	if articleID == 0 || creatorID == 0 {
		return link, ErrArticlePreviewUnavailable
	}

	rawToken := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, rawToken); err != nil {
		return link, err
	}
	link.Token = base64.RawURLEncoding.EncodeToString(rawToken)
	tokenHash := sha256.Sum256([]byte(link.Token))
	now := time.Now().UTC()
	link.ExpiresAt = now.Add(ArticlePreviewTTL)

	repo := mysql.ArticlePreviewDBRepo{}
	err := connect.Database.Transaction(func(tx *gorm.DB) error {
		source, err := repo.LoadCreateSource(tx, articleID, creatorID)
		if err != nil {
			return err
		}
		if source.Article.EditVersion != expectedEditVersion {
			return mysql.ErrArticleEditConflict
		}

		snapshot := liveArticlePreviewSnapshot(&source.Article)
		if source.Submission != nil {
			var submissionSnapshot model.ArticleSubmissionSnapshot
			if err := json.Unmarshal([]byte(source.Submission.Snapshot), &submissionSnapshot); err != nil {
				return err
			}
			if err := applyArticleSubmissionPreviewSnapshot(&repo, tx, &snapshot, submissionSnapshot); err != nil {
				return err
			}
		}

		serialized, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		preview := &model.ArticlePreview{
			ArticleID: articleID,
			CreatorID: creatorID,
			TokenHash: hex.EncodeToString(tokenHash[:]),
			Snapshot:  string(serialized),
			ExpiresAt: link.ExpiresAt,
		}
		return repo.Create(tx, preview)
	})
	if err != nil {
		return ArticlePreviewLink{}, err
	}
	return link, nil
}

func RevokeArticlePreviews(articleID, creatorID uint) error {
	if articleID == 0 || creatorID == 0 {
		return ErrArticlePreviewUnavailable
	}
	return (&mysql.ArticlePreviewDBRepo{}).RevokeForArticleCreator(articleID, creatorID, time.Now().UTC())
}

// LoadArticlePreview validates the one-hour bearer token and returns only its
// persisted snapshot. It never reloads mutable article content for rendering.
func LoadArticlePreview(token string) (*ArticlePreviewPage, error) {
	if token == "" {
		return nil, ErrArticlePreviewUnavailable
	}
	digest := sha256.Sum256([]byte(token))
	preview, err := (&mysql.ArticlePreviewDBRepo{}).FindValidByTokenHash(hex.EncodeToString(digest[:]), time.Now().UTC())
	if err != nil {
		return nil, ErrArticlePreviewUnavailable
	}

	var page ArticlePreviewPage
	if err := connect.Database.Transaction(func(tx *gorm.DB) error {
		if _, _, err := (&mysql.ArticlePreviewDBRepo{}).LoadAccessSource(tx, preview.ArticleID, preview.CreatorID); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(preview.Snapshot), &page.Snapshot); err != nil {
			return err
		}
		page.ExpiresAt = preview.ExpiresAt
		return nil
	}); err != nil {
		return nil, ErrArticlePreviewUnavailable
	}
	return &page, nil
}

func liveArticlePreviewSnapshot(article *mysql.ArticleModel) model.ArticlePreviewSnapshot {
	snapshot := model.ArticlePreviewSnapshot{
		EditVersion:    article.EditVersion,
		UID:            article.UID,
		Title:          article.Title,
		Slug:           article.Slug,
		SEOTitle:       article.SEOTitle,
		SEODescription: article.SEODescription,
		ShareImage:     article.ShareImage,
		Type:           article.Type,
		ContentFormat:  model.NormalizeArticleContentFormat(article.ContentFormat),
		Summary:        article.Summary,
		CategoryUID:    article.CategoryUID,
		PostTime:       article.PostTime,
		UpdatedAt:      article.UpdatedAt,
		ReadCount:      article.ReadCount,
		IsAllowComment: cloneBool(article.IsAllowComment),
		Tags:           make([]model.ArticlePreviewTag, 0, len(article.Tags)),
	}
	if article.Detail != nil {
		snapshot.Content = article.Detail.Content
	}
	if article.Category != nil {
		snapshot.CategoryName = article.Category.Name
	}
	for _, tag := range article.Tags {
		if tag != nil {
			snapshot.Tags = append(snapshot.Tags, model.ArticlePreviewTag{UID: tag.UID, Name: tag.Name})
		}
	}
	if article.Author != nil {
		snapshot.AuthorName = article.Author.Name
		snapshot.AuthorNickName = article.Author.NickName
		snapshot.AuthorSlug = article.Author.Slug
		snapshot.AuthorAvatar = article.Author.AvatarURL
	}
	return snapshot
}

func applyArticleSubmissionPreviewSnapshot(repo *mysql.ArticlePreviewDBRepo, tx *gorm.DB, target *model.ArticlePreviewSnapshot, source model.ArticleSubmissionSnapshot) error {
	target.Title = source.Title
	target.Slug = source.Slug
	target.SEOTitle = source.SEOTitle
	target.SEODescription = source.SEODescription
	target.ShareImage = source.ShareImage
	target.Type = source.Type
	target.ContentFormat = model.NormalizeArticleContentFormat(source.ContentFormat)
	target.Summary = source.Summary
	target.Content = source.Content
	target.CategoryUID = source.CategoryUID
	target.IsAllowComment = cloneBool(source.IsAllowComment)
	if !source.PostTime.IsZero() {
		target.PostTime = source.PostTime
	}

	categoryName, tags, err := repo.LoadTaxonomy(tx, source.CategoryUID, source.TagUIDs)
	if err != nil {
		return err
	}
	target.CategoryName = categoryName
	target.Tags = tags
	return nil
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
