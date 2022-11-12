package service

var (
	S = newService()
)

type s struct {
	Article  ArticleService
	Setting  SettingService
	Category CategoryService
	Link     LinkService
	File     FileService
}

func newService() s {
	return s{
		Article:  ArticleService{},
		Setting:  SettingService{},
		Category: CategoryService{},
		Link:     LinkService{},
		File:     FileService{},
	}
}
