package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"myecho/model"
	"myecho/utils"
	"net/http"
	"strings"
	"time"
)

func NotifyPendingComment(article *model.Article, comment *model.Comment) error {
	setting, err := S.Setting.GetByKey("CommentNotificationWebhook")
	if err != nil || strings.TrimSpace(setting.Value) == "" {
		return nil
	}
	if err := utils.ValidateRemoteFileURL(setting.Value); err != nil {
		return err
	}
	body, err := json.Marshal(map[string]interface{}{
		"event":         "comment.pending",
		"comment_id":    comment.ID,
		"article_id":    article.ID,
		"article_title": article.Title,
		"author_name":   comment.AuthorName,
		"content":       comment.Content,
		"post_time":     comment.PostTime,
	})
	if err != nil {
		return err
	}
	client := *utils.RemoteFileHTTPClient
	client.Timeout = 5 * time.Second
	response, err := client.Post(setting.Value, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("comment notification webhook returned %s", response.Status)
	}
	return nil
}
