package service

var (
	S = newService()
)

type s struct {
	Article ArticleService
	Setting SettingService
}

func newService() s {
	return s{
		Article: ArticleService{},
		Setting: SettingService{},
	}
}
