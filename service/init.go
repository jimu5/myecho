package service

var (
	S = newService()
)

type s struct {
	Article Article
}

func newService() s {
	return s{
		Article: Article{},
	}
}
