package service

var (
	S = newService()
)

type s struct {
	Article Article
	Setting Setting
}

func newService() s {
	return s{
		Article: Article{},
		Setting: Setting{},
	}
}
