package rtype

import (
	"time"

	"myecho/model"
)

type ArticleRevisionDetailResponse struct {
	ID             uint                       `json:"id"`
	ArticleID      uint                       `json:"article_id"`
	CreatedAt      time.Time                  `json:"created_at"`
	ActorID        uint                       `json:"actor_id"`
	Title          string                     `json:"title"`
	Slug           string                     `json:"slug"`
	Type           model.ArticleType          `json:"type"`
	ContentFormat  model.ArticleContentFormat `json:"content_format"`
	Summary        string                     `json:"summary"`
	SEOTitle       string                     `json:"seo_title"`
	SEODescription string                     `json:"seo_description"`
	ShareImage     string                     `json:"share_image"`
	Content        string                     `json:"content"`
}
