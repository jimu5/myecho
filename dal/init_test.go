package dal

import "testing"

func TestNewMysqlDBRepoWiresRepositories(t *testing.T) {
	repo := NewMysqlDBRepo()
	if repo.Article == nil || repo.File == nil || repo.Category == nil || repo.Setting == nil || repo.Link == nil || repo.Theme == nil {
		t.Fatalf("NewMysqlDBRepo() returned incomplete repo: %+v", repo)
	}
}
