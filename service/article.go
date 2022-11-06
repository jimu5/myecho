package service

import (
	"log"
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
	topArticles, err := dal.MySqlDB.Article.PageFindByCommonParam(&pageParam, sqlParam)
	if err != nil {
		return pageInfo, nil, err
	}
	pageParam.ForceOffset = pageParam.PageSize*(pageParam.Page-1) - int(topTotal) // 注意这里 topTotal 位数
	status = mysql.ARTILCE_STATUS_PUBLIC
	sqlParam.Status = &status
	restArticles, err := dal.MySqlDB.Article.PageFindByCommonParam(&pageParam, sqlParam)
	if err != nil {
		return pageInfo, nil, err
	}
	articles := topArticles
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
