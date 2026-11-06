package model

import "time"

type ArticleReviewAction string

const (
	ArticleReviewSubmit   ArticleReviewAction = "submit"
	ArticleReviewApprove  ArticleReviewAction = "approve"
	ArticleReviewReject   ArticleReviewAction = "reject"
	ArticleReviewWithdraw ArticleReviewAction = "withdraw"
)

type ArticleReview struct {
	ID           uint                `json:"id" gorm:"primarykey"`
	CreatedAt    time.Time           `json:"created_at"`
	ArticleID    uint                `json:"article_id" gorm:"index"`
	SubmissionID uint                `json:"submission_id" gorm:"index"`
	ActorID      uint                `json:"actor_id" gorm:"index"`
	Actor        *User               `json:"-"`
	Action       ArticleReviewAction `json:"action" gorm:"size:16"`
	FromStatus   int8                `json:"from_status"`
	ToStatus     int8                `json:"to_status"`
	Note         string              `json:"note" gorm:"size:500"`
}
