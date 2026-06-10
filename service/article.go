package service

import (
	"errors"
	"log"
	"myecho/config/static_config"
	"myecho/dal"
	"myecho/dal/mysql"
	"strings"
	"time"
)

type ArticleService struct {
}

type ArticleDisplayListQueryParam struct {
	CategoryUID *string `query:"category_uid"`
	Keyword     *string `query:"keyword"`
	TagUID      *string `query:"tag_uid"`
	DateFrom    string  `query:"date_from"`
	DateTo      string  `query:"date_to"`
	mysql.PageFindParam
}

type ArticleRetrieveQueryParam struct {
	ID               uint `json:"id"`
	NoRead           bool `query:"no_read"`
	IncludeNonPublic bool `json:"-"`
}

var ErrArticleNotDisplayable = errors.New("article is not displayable")

func (a *ArticleService) ArticleDisplayList(param *ArticleDisplayListQueryParam) (mysql.PageInfo, []*mysql.ArticleModel, error) {
	status := mysql.ARTICLE_STATUS_TOP
	pageInfo := mysql.PageInfo{}
	sqlParam, err := BuildArticleCommonQueryParam(param.CategoryUID, param.Keyword, param.TagUID, param.DateFrom, param.DateTo)
	if err != nil {
		return pageInfo, nil, err
	}
	sqlParam.Status = &status
	total, err := dal.MySqlDB.Article.CountDisplayable(sqlParam)
	if err != nil {
		return pageInfo, nil, err
	}
	topTotal, err := dal.MySqlDB.Article.CountAll(sqlParam)
	if err != nil {
		return pageInfo, nil, err
	}
	pageInfo.Total = total
	pageParam := param.PageFindParam
	pageInfo.FillInfoFromParam(&pageParam)
	if pageParam.Page < 1 {
		pageParam.Page = 1
	}
	if pageParam.PageSize < 1 {
		pageParam.PageSize = static_config.PageSize
	}
	pageOffset := (pageParam.Page - 1) * pageParam.PageSize

	articles := make([]*mysql.ArticleModel, 0, pageParam.PageSize)
	if pageOffset < int(topTotal) {
		topParam := pageParam
		topParam.UseForceOffset = true
		topParam.ForceOffset = pageOffset
		topParam.PageSize = min(pageParam.PageSize, int(topTotal)-pageOffset)
		topArticles, err := dal.MySqlDB.Article.PageFindByCommonParam(&topParam, sqlParam)
		if err != nil {
			return pageInfo, nil, err
		}
		articles = append(articles, topArticles...)
	}

	restLimit := pageParam.PageSize - len(articles)
	if restLimit == 0 {
		return pageInfo, articles, nil
	}
	restOffset := pageOffset - int(topTotal)
	if restOffset < 0 {
		restOffset = 0
	}
	restParam := pageParam
	restParam.UseForceOffset = true
	restParam.ForceOffset = restOffset
	restParam.PageSize = restLimit
	status = mysql.ARTILCE_STATUS_PUBLIC
	sqlParam.Status = &status
	restArticles, err := dal.MySqlDB.Article.PageFindByCommonParam(&restParam, sqlParam)
	if err != nil {
		return pageInfo, nil, err
	}
	articles = append(articles, restArticles...)
	return pageInfo, articles, nil
}

func (a *ArticleService) ArticleRetrieve(param *ArticleRetrieveQueryParam) (mysql.ArticleModel, error) {
	article, err := dal.MySqlDB.Article.FindByID(param.ID)
	if err != nil {
		return mysql.ArticleModel{}, err
	}
	if !param.IncludeNonPublic && !isArticleDisplayable(article.Status) {
		return mysql.ArticleModel{}, ErrArticleNotDisplayable
	}
	if !param.NoRead {
		go func() {
			if err := dal.MySqlDB.Article.AddReadCountByID(article.ID, 1); err != nil {
				log.Println(err)
			}
		}()
	}
	return article, nil
}

func isArticleDisplayable(status int8) bool {
	return status == int8(mysql.ARTILCE_STATUS_PUBLIC) || status == int8(mysql.ARTICLE_STATUS_TOP)
}

func BuildArticleCommonQueryParam(categoryUID, keyword, tagUID *string, dateFrom, dateTo string) (mysql.ArticleCommonQueryParam, error) {
	param := mysql.ArticleCommonQueryParam{
		CategoryUID: categoryUID,
		Keyword:     keyword,
		TagUID:      tagUID,
	}
	from, err := parseArticleQueryDate(dateFrom, false)
	if err != nil {
		return param, err
	}
	to, err := parseArticleQueryDate(dateTo, true)
	if err != nil {
		return param, err
	}
	param.DateFrom = from
	param.DateTo = to
	return param, nil
}

func parseArticleQueryDate(value string, endOfDay bool) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, value, time.Local)
		if err != nil {
			continue
		}
		if endOfDay && layout == "2006-01-02" {
			t = t.Add(24*time.Hour - time.Nanosecond)
		}
		return &t, nil
	}
	return nil, errors.New("date format must be YYYY-MM-DD or RFC3339")
}
