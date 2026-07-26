package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"myecho/config"
	"myecho/config/static_config"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/model"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
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
	PasswordUnlocked bool `json:"-"`
}

var ErrArticleNotDisplayable = errors.New("article is not displayable")
var ErrArticlePasswordRequired = errors.New("article password required")
var ErrArticlePasswordInvalid = errors.New("article password invalid")

const (
	ArticlePasswordSecretKey    = "ArticlePasswordTokenSecret"
	ArticlePasswordCookiePrefix = "myecho_article_unlock_"
)

func (a *ArticleService) ArticleDisplayList(param *ArticleDisplayListQueryParam) (mysql.PageInfo, []*mysql.ArticleModel, error) {
	status := mysql.ARTICLE_STATUS_TOP
	articleType := model.ArticleTypePost
	pageInfo := mysql.PageInfo{}
	sqlParam, err := BuildArticleCommonQueryParam(param.CategoryUID, param.Keyword, param.TagUID, param.DateFrom, param.DateTo)
	if err != nil {
		return pageInfo, nil, err
	}
	now := time.Now()
	if sqlParam.DateTo == nil || sqlParam.DateTo.After(now) {
		sqlParam.DateTo = &now
	}
	sqlParam.Status = &status
	sqlParam.Type = &articleType
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
	pageParam.NoPage = false
	if pageParam.Page < 1 {
		pageParam.Page = 1
	}
	if pageParam.PageSize < 1 {
		pageParam.PageSize = static_config.PageSize
	}
	if pageParam.PageSize > 100 {
		pageParam.PageSize = 100
	}
	pageInfo.FillInfoFromParam(&pageParam)
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
	if !param.IncludeNonPublic && !IsArticlePubliclyVisible(article.Status, article.PostTime) {
		return mysql.ArticleModel{}, ErrArticleNotDisplayable
	}
	if !param.IncludeNonPublic && article.Password != "" && !param.PasswordUnlocked {
		return mysql.ArticleModel{}, ErrArticlePasswordRequired
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

func (a *ArticleService) PostNeighbors(article *mysql.ArticleModel) (*mysql.ArticleModel, *mysql.ArticleModel, error) {
	if article == nil || article.Type != model.ArticleTypePost {
		return nil, nil, nil
	}
	return dal.MySqlDB.Article.FindPostNeighbors(article)
}

func (a *ArticleService) RelatedPosts(article *mysql.ArticleModel) ([]*mysql.ArticleModel, error) {
	return dal.MySqlDB.Article.FindRelatedPosts(article, 3)
}

func HashArticlePassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", nil
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hashedPassword), err
}

func CheckArticlePassword(storedPassword, password string) error {
	if storedPassword == "" {
		return nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password)); err != nil {
		return ErrArticlePasswordInvalid
	}
	return nil
}

func ArticlePasswordCookieName(articleID uint) string {
	return fmt.Sprintf("%s%d", ArticlePasswordCookiePrefix, articleID)
}

func CreateArticlePasswordToken(article *mysql.ArticleModel) (string, error) {
	if article == nil || article.ID == 0 || article.Password == "" {
		return "", ErrArticlePasswordInvalid
	}
	secret, err := getArticlePasswordSecret()
	if err != nil {
		return "", err
	}
	return signArticlePassword(article.ID, article.Password, secret), nil
}

func ValidateArticlePasswordToken(article *mysql.ArticleModel, token string) bool {
	if article == nil || article.ID == 0 || article.Password == "" || token == "" {
		return false
	}
	secret, err := getArticlePasswordSecret()
	if err != nil {
		return false
	}
	expected := signArticlePassword(article.ID, article.Password, secret)
	return hmac.Equal([]byte(expected), []byte(token))
}

func signArticlePassword(articleID uint, passwordHash, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d:%s", articleID, passwordHash)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func getArticlePasswordSecret() (string, error) {
	setting, err := dal.MySqlDB.Setting.GetByKey(ArticlePasswordSecretKey)
	if err == nil && setting.Value != "" {
		return setting.Value, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	secret, err := randomArticleSecret()
	if err != nil {
		return "", err
	}
	setting = mysql.SettingModel{
		Key:         ArticlePasswordSecretKey,
		Value:       secret,
		Type:        model.SettingModelTypeString,
		Description: "Article password unlock token signing secret",
		IsSystem:    true,
	}
	if err := dal.MySqlDB.Setting.Create(&setting); err != nil {
		latest, latestErr := dal.MySqlDB.Setting.GetByKey(ArticlePasswordSecretKey)
		if latestErr == nil && latest.Value != "" {
			return latest.Value, nil
		}
		return "", err
	}
	if config.MySqlSettingModelCache != nil {
		config.MySqlSettingModelCache.Set(setting.Key, &setting)
	}
	return secret, nil
}

func randomArticleSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func IsArticlePubliclyVisible(status int8, postTime time.Time) bool {
	return (status == int8(mysql.ARTILCE_STATUS_PUBLIC) || status == int8(mysql.ARTICLE_STATUS_TOP)) &&
		!postTime.After(time.Now())
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
