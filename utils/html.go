package utils

import "github.com/microcosm-cc/bluemonday"

var articleHTMLPolicy = bluemonday.UGCPolicy()

func SanitizeArticleHTML(content string) string {
	return articleHTMLPolicy.Sanitize(content)
}
