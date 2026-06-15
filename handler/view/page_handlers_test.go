package view

import (
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler/rtype"
	"myecho/model"
	"myecho/service"
)

type spyViews struct {
	template string
	data     interface{}
}

func (s *spyViews) Load() error {
	return nil
}

func (s *spyViews) Render(out io.Writer, template string, data interface{}, layout ...string) error {
	s.template = template
	s.data = data
	_, err := out.Write([]byte(template))
	return err
}

func TestArticlePagesRenderListAndDetail(t *testing.T) {
	setupViewThemeTestDB(t)
	category := &mysql.CategoryModel{Name: "Tech", UID: "tech", Type: model.CategoryTypeArticle}
	if err := (&mysql.CategoryRepo{}).Create(category); err != nil {
		t.Fatalf("create category: %v", err)
	}
	publicArticle := &mysql.ArticleModel{
		Title:          "Public",
		Summary:        "summary",
		CategoryUID:    category.UID,
		Status:         int8(mysql.ARTILCE_STATUS_PUBLIC),
		PostTime:       time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
		IsAllowComment: boolPtr(true),
		Detail:         &model.ArticleDetail{Content: "hello **world**"},
	}
	htmlArticle := &mysql.ArticleModel{
		Title:         "HTML",
		Summary:       "html summary",
		CategoryUID:   category.UID,
		Status:        int8(mysql.ARTILCE_STATUS_PUBLIC),
		PostTime:      time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		ContentFormat: model.ArticleContentFormatHTML,
		Detail:        &model.ArticleDetail{Content: `<section class="fancy"><style>.fancy{color:red}</style><h2>*Not Markdown*</h2></section>`},
	}
	draftArticle := &mysql.ArticleModel{
		Title:    "Draft",
		Status:   int8(mysql.ARTICLE_STATUS_DRAFT),
		PostTime: time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC),
		Detail:   &model.ArticleDetail{Content: "draft"},
	}
	repo := &mysql.ArticleDBRepo{}
	if err := repo.Create(publicArticle); err != nil {
		t.Fatalf("create public article: %v", err)
	}
	if err := repo.Create(htmlArticle); err != nil {
		t.Fatalf("create html article: %v", err)
	}
	if err := repo.Create(draftArticle); err != nil {
		t.Fatalf("create draft article: %v", err)
	}
	approved := int8(model.CommentStatusApproved)
	pending := int8(model.CommentStatusPending)
	comments := []model.Comment{
		{ArticleUID: publicArticle.UID, AuthorName: "Approved", Content: "ok", Status: &approved, PostTime: time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC)},
		{ArticleUID: publicArticle.UID, AuthorName: "Pending", Content: "wait", Status: &pending, PostTime: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)},
	}
	if err := connect.Database.Create(&comments).Error; err != nil {
		t.Fatalf("create comments: %v", err)
	}

	listViews := &spyViews{}
	app := fiber.New(fiber.Config{Views: listViews})
	app.Get("/", ArticleDisplayList)
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("list request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK || listViews.template != "index" {
		t.Fatalf("list status=%d template=%q", resp.StatusCode, listViews.template)
	}
	listData := listViews.data.(fiber.Map)["Data"].(Pagination)
	articles := listData.PageData.([]*mysql.ArticleModel)
	if len(articles) != 2 || articles[0].Title != "HTML" || articles[1].Title != "Public" {
		t.Fatalf("list articles = %+v", articles)
	}

	detailViews := &spyViews{}
	app = fiber.New(fiber.Config{Views: detailViews})
	app.Get("/articles/:id", ArticleRetrieve)
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/articles/"+strconv.Itoa(int(publicArticle.ID))+"?no_read=true", nil))
	if err != nil {
		t.Fatalf("detail request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK || detailViews.template != "article" {
		t.Fatalf("detail status=%d template=%q", resp.StatusCode, detailViews.template)
	}
	detailData := detailViews.data.(fiber.Map)
	article := detailData["Data"].(mysql.ArticleModel)
	if !strings.Contains(article.Detail.Content, "<strong>world</strong>") {
		t.Fatalf("rendered markdown content = %q", article.Detail.Content)
	}
	approvedComments := detailData["Comments"].([]rtype.CommentResponse)
	if len(approvedComments) != 1 || approvedComments[0].AuthorName != "Approved" {
		t.Fatalf("approved comments = %+v", approvedComments)
	}
	if !detailData["IsAllowComment"].(bool) {
		t.Fatal("IsAllowComment = false, want true")
	}

	htmlViews := &spyViews{}
	app = fiber.New(fiber.Config{Views: htmlViews})
	app.Get("/articles/:id", ArticleRetrieve)
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/articles/"+strconv.Itoa(int(htmlArticle.ID))+"?no_read=true", nil))
	if err != nil {
		t.Fatalf("html detail request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK || htmlViews.template != "article" {
		t.Fatalf("html detail status=%d template=%q", resp.StatusCode, htmlViews.template)
	}
	htmlData := htmlViews.data.(fiber.Map)
	htmlRendered := htmlData["Data"].(mysql.ArticleModel)
	if !strings.Contains(htmlRendered.Detail.Content, `<section class="fancy">`) || strings.Contains(htmlRendered.Detail.Content, "<em>Not Markdown</em>") {
		t.Fatalf("rendered html content = %q", htmlRendered.Detail.Content)
	}
}

func TestPostPageTagAndArchiveViews(t *testing.T) {
	setupViewThemeTestDB(t)
	category := &mysql.CategoryModel{Name: "Tech", UID: "tech", Type: model.CategoryTypeArticle}
	if err := (&mysql.CategoryRepo{}).Create(category); err != nil {
		t.Fatalf("create category: %v", err)
	}
	tag := &model.Tag{Name: "Go", UID: "tag-go"}
	if err := connect.Database.Create(tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	repo := &mysql.ArticleDBRepo{}
	post := &mysql.ArticleModel{
		Title:       "Slug Post",
		Slug:        "slug-post",
		Type:        model.ArticleTypePost,
		Summary:     "summary",
		CategoryUID: category.UID,
		Status:      int8(mysql.ARTILCE_STATUS_PUBLIC),
		PostTime:    time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
		Detail:      &model.ArticleDetail{Content: "post body"},
		Tags:        []*model.Tag{tag},
	}
	page := &mysql.ArticleModel{
		Title:    "About",
		Slug:     "about",
		Type:     model.ArticleTypePage,
		Status:   int8(mysql.ARTILCE_STATUS_PUBLIC),
		PostTime: time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC),
		Detail:   &model.ArticleDetail{Content: "page body"},
	}
	if err := repo.Create(post); err != nil {
		t.Fatalf("create post: %v", err)
	}
	if err := repo.Create(page); err != nil {
		t.Fatalf("create page: %v", err)
	}

	postViews := &spyViews{}
	app := fiber.New(fiber.Config{Views: postViews})
	app.Get("/posts/:slug", PostRetrieveBySlug)
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/posts/slug-post?no_read=true", nil))
	if err != nil {
		t.Fatalf("post slug request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK || postViews.template != "article" {
		t.Fatalf("post slug status=%d template=%q", resp.StatusCode, postViews.template)
	}
	if got := postViews.data.(fiber.Map)["Data"].(mysql.ArticleModel); got.ID != post.ID {
		t.Fatalf("post slug data = %+v, want id %d", got, post.ID)
	}

	pageViews := &spyViews{}
	app = fiber.New(fiber.Config{Views: pageViews})
	app.Get("/pages/:slug", PageRetrieveBySlug)
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/pages/about?no_read=true", nil))
	if err != nil {
		t.Fatalf("page slug request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK || pageViews.template != "article" {
		t.Fatalf("page slug status=%d template=%q", resp.StatusCode, pageViews.template)
	}
	if got := pageViews.data.(fiber.Map)["Data"].(mysql.ArticleModel); got.ID != page.ID {
		t.Fatalf("page slug data = %+v, want id %d", got, page.ID)
	}

	tagsViews := &spyViews{}
	app = fiber.New(fiber.Config{Views: tagsViews})
	app.Get("/tags", TagArchive)
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/tags", nil))
	if err != nil {
		t.Fatalf("tags request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK || tagsViews.template != "tags" {
		t.Fatalf("tags status=%d template=%q", resp.StatusCode, tagsViews.template)
	}
	tagData := tagsViews.data.(fiber.Map)["Data"].([]TagArchiveItem)
	if len(tagData) != 1 || tagData[0].UID != "tag-go" || tagData[0].Count != 1 {
		t.Fatalf("tag archive data = %+v", tagData)
	}

	archiveViews := &spyViews{}
	app = fiber.New(fiber.Config{Views: archiveViews})
	app.Get("/archive", Archive)
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/archive", nil))
	if err != nil {
		t.Fatalf("archive request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK || archiveViews.template != "archive" {
		t.Fatalf("archive status=%d template=%q", resp.StatusCode, archiveViews.template)
	}
	archiveData := archiveViews.data.(fiber.Map)["Data"].([]ArchiveGroup)
	if len(archiveData) != 1 || archiveData[0].Month != "2026-06" || len(archiveData[0].Articles) != 1 {
		t.Fatalf("archive data = %+v", archiveData)
	}
}

func TestCategoryAndLinkPagesRenderData(t *testing.T) {
	setupViewThemeTestDB(t)
	articleCategory := &mysql.CategoryModel{Name: "Articles", UID: "article-cat", Type: model.CategoryTypeArticle}
	linkCategory := &mysql.CategoryModel{Name: "Links", UID: "link-cat", Type: model.CategoryTypeLink}
	categoryRepo := &mysql.CategoryRepo{}
	if err := categoryRepo.Create(articleCategory); err != nil {
		t.Fatalf("create article category: %v", err)
	}
	if err := categoryRepo.Create(linkCategory); err != nil {
		t.Fatalf("create link category: %v", err)
	}
	if err := (&mysql.LinkRepo{}).Create(&mysql.LinkModel{Name: "Friend", URL: "https://friend.example.com", CategoryUID: linkCategory.UID}); err != nil {
		t.Fatalf("create link: %v", err)
	}

	categoryViews := &spyViews{}
	app := fiber.New(fiber.Config{Views: categoryViews})
	app.Get("/article/categories", CategoryArticleAll)
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/article/categories", nil))
	if err != nil {
		t.Fatalf("category request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK || categoryViews.template != "category" {
		t.Fatalf("category status=%d template=%q", resp.StatusCode, categoryViews.template)
	}
	categories := categoryViews.data.(fiber.Map)["Data"].([]*service.Category)
	if len(categories) != 1 || categories[0].UID != articleCategory.UID {
		t.Fatalf("categories = %+v", categories)
	}

	linkViews := &spyViews{}
	app = fiber.New(fiber.Config{Views: linkViews})
	app.Get("/links", LinkAll)
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/links", nil))
	if err != nil {
		t.Fatalf("link request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK || linkViews.template != "link" {
		t.Fatalf("link status=%d template=%q", resp.StatusCode, linkViews.template)
	}
	links := linkViews.data.(fiber.Map)["Data"].([]*mysql.LinkModel)
	if len(links) != 1 || links[0].Name != "Friend" {
		t.Fatalf("links = %+v", links)
	}
}

func TestPageInfoResponseBuildsNavigationURLs(t *testing.T) {
	app := fiber.New()
	app.Get("/posts", func(c *fiber.Ctx) error {
		first := getPageInfoRespByMysqlPageInfo(c, &mysql.PageInfo{Total: 21, Page: 1, PageSize: 10})
		if first.Pre != "" || first.Next != "/posts?category=go&page=2" || first.Total != 0 {
			t.Fatalf("first page info = %+v", first)
		}
		middle := getPageInfoRespByMysqlPageInfo(c, &mysql.PageInfo{Total: 25, Page: 2, PageSize: 10})
		if middle.Pre != "/posts?category=go&page=1" || middle.Next != "/posts?category=go&page=3" {
			t.Fatalf("middle page info = %+v", middle)
		}
		empty := getPageInfoRespByMysqlPageInfo(c, &mysql.PageInfo{Total: 0})
		if empty.Total != 0 || empty.Next != "" || empty.Pre != "" {
			t.Fatalf("empty page info = %+v", empty)
		}
		if genRawUrl("/x", "a=1") != "/x?a=1" {
			t.Fatal("genRawUrl() mismatch")
		}
		if got := absoluteURL(c); !strings.HasSuffix(got, "/posts") {
			t.Fatalf("absoluteURL() = %q", got)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/posts?category=go&page=2", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
