package service

import (
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler/rtype"
)

type Article struct {
}

type ArticleDisplayListQueryParam struct {
	rtype.ArticleDisplayListQueryParam
	mysql.PageFindParam
}

func (a *Article) ArticleDisplayList(param *ArticleDisplayListQueryParam) (int64, []*rtype.ArticleResponse, error) {
	status := mysql.ARTICLE_STATUS_TOP
	sqlParam := mysql.ArticleCommonQueryParam{
		CategoryID: param.CategoryID,
		Status:     &status,
	}
	total, err := dal.MySqlDB.Article.CountAll(sqlParam)
	if err != nil {
		return total, nil, err
	}
	pageParam := param.PageFindParam
	topArticles, err := dal.MySqlDB.Article.PageFindByCommonParam(&pageParam, sqlParam)
	if err != nil {
		return total, nil, err
	}
	pageParam.PageSize = pageParam.PageSize - len(topArticles)
	status = mysql.ARTILCE_STATUS_PUBLIC
	sqlParam.Status = &status
	restArticles, err := dal.MySqlDB.Article.PageFindByCommonParam(&pageParam, sqlParam)
	if err != nil {
		return total, nil, err
	}
	articles := topArticles
	articles = append(articles, restArticles...)
	res := rtype.MultiModelToArticleResponse(articles)
	return total, res, nil
}
