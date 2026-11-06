package model

import "time"

type ArticleSubmissionStatus int8

const (
	ArticleSubmissionDraft ArticleSubmissionStatus = iota + 1
	ArticleSubmissionPending
	ArticleSubmissionApproved
	ArticleSubmissionRejected
	ArticleSubmissionWithdrawn
)

// ArticleSubmission stores the exact payload reviewed by an editor. Snapshot is
// JSON so article schema changes do not require a second mirrored table.
type ArticleSubmission struct {
	BaseModel
	ArticleID   uint                    `json:"article_id" gorm:"index"`
	SubmitterID uint                    `json:"submitter_id" gorm:"index"`
	Submitter   *User                   `json:"submitter,omitempty"`
	ReviewerID  uint                    `json:"reviewer_id" gorm:"index"`
	Reviewer    *User                   `json:"reviewer,omitempty"`
	Status      ArticleSubmissionStatus `json:"status" gorm:"index"`
	Snapshot    string                  `json:"-" gorm:"type:text"`
	// BaseEditVersion is the article edit token committed with this snapshot.
	BaseEditVersion uint64     `json:"base_edit_version" gorm:"not null;default:0"`
	Note            string     `json:"note" gorm:"size:500"`
	SubmittedAt     *time.Time `json:"submitted_at"`
	ReviewedAt      *time.Time `json:"reviewed_at"`
}

type ArticleSubmissionSnapshot struct {
	Title          string               `json:"title"`
	Slug           string               `json:"slug"`
	SEOTitle       string               `json:"seo_title"`
	SEODescription string               `json:"seo_description"`
	ShareImage     string               `json:"share_image"`
	Type           ArticleType          `json:"type"`
	ContentFormat  ArticleContentFormat `json:"content_format"`
	Summary        string               `json:"summary"`
	Content        string               `json:"content"`
	CategoryUID    string               `json:"category_uid"`
	IsAllowComment *bool                `json:"is_allow_comment"`
	PostTime       time.Time            `json:"post_time"`
	TargetStatus   int8                 `json:"target_status"`
	PasswordHash   string               `json:"password_hash"`
	TagUIDs        []string             `json:"tag_uids"`
}
