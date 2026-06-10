package service

import (
	"log"
	"myecho/config/static_config"
	"myecho/dal"
	"myecho/dal/mysql"
)

type ArticleService struct {
}

type ArticleDisplayListQueryParam struct {
	CategoryUID *string `query:"category_uid"`
	mysql.PageFindParam
}

type ArticleRetrieveQueryParam struct {
	ID     uint `json:"id"`
	NoRead bool `query:"no_read"`
}

func (a *ArticleService) ArticleDisplayList(param *ArticleDisplayListQueryParam) (mysql.PageInfo, []*mysql.ArticleModel, error) {
	status := mysql.ARTICLE_STATUS_TOP
	pageInfo := mysql.PageInfo{}
	sqlParam := mysql.ArticleCommonQueryParam{
		CategoryUID: param.CategoryUID,
		Status:      &status,
	}
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
	if !param.NoRead {
		go func() {
			if err := dal.MySqlDB.Article.AddReadCountByID(article.ID, 1); err != nil {
				log.Println(err)
			}
		}()
	}
	return article, nil
}
