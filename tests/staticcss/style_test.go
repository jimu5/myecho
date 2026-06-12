package staticcss_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArticleCodeBlocksOverridePrismForDarkShellOutput(t *testing.T) {
	css := readStyle(t)
	assertContains(t, css, `#article_content pre[class*="language-"]`)
	assertContains(t, css, `#article_content pre[class*="language-"] code[class*="language-"]`)
	assertContains(t, css, `text-shadow: none`)
	assertContains(t, css, `#article_content pre .token.space::before`)
	assertContains(t, css, `#article_content pre .token.lf::before`)
	assertContains(t, css, `content: none`)
}

func TestArticlePageIncludesSiteFooter(t *testing.T) {
	article := readProjectFile(t, "views", "article.jet.html")
	assertContains(t, article, `{{ include "components/footer" RequestTimeDuration }}`)
}

func TestDefaultThemeHeadUsesMatchingThemeColor(t *testing.T) {
	head := readProjectFile(t, "views", "components", "common_head.jet.html")
	assertContains(t, head, `<meta name="theme-color" content="#faf9f5">`)
	assertNotContains(t, head, `<meta name="theme-color" content="#f6f8f4">`)
}

func TestArticleTocUsesArticleWidthAndFallsBackBeforeOverlap(t *testing.T) {
	css := readStyle(t)
	assertContains(t, css, `left: calc(50% + 440px)`)
	assertContains(t, css, `@media screen and (max-width: 1439px)`)
	assertContains(t, css, `.toc-container.active`)
}

func TestDefaultThemeUsesFlatMagazineLayout(t *testing.T) {
	css := readStyle(t)
	assertContains(t, css, `--radius-lg: 8px;`)
	assertContains(t, css, `--shadow-panel: none;`)
	assertContains(t, css, `background: var(--background-color);`)
	assertContains(t, css, `gap: 0;`)
	assertContains(t, css, `border-top: 1px solid var(--border-color);`)
	assertContains(t, css, `border-bottom: 1px solid var(--border-color);`)
	assertContains(t, css, `.article-block::before {
    display: none;
}`)
	assertContains(t, css, `#article_content {
    padding: 44px 0 0;`)
	assertContains(t, css, `background-color: transparent;`)
	assertContains(t, css, `box-shadow: none;`)
	assertContains(t, css, `border-left: 1px solid var(--border-color);`)
	assertNotContains(t, css, `gradient(`)
	assertNotContains(t, css, `backdrop-filter`)
}

func TestSharedLayoutKeepsShortPagesGrounded(t *testing.T) {
	css := readStyle(t)
	assertContains(t, css, `body {
    display: flex;`)
	assertContains(t, css, `flex-direction: column;`)
	assertContains(t, css, `body > main {
    flex: 1 0 auto;
}`)

	article := readProjectFile(t, "views", "article.jet.html")
	assertContains(t, article, `{{ include "components/footer" RequestTimeDuration }}`)
}

func TestIndexSearchLivesCollapsedInHeadingTools(t *testing.T) {
	index := readProjectFile(t, "views", "index.jet.html")
	assertContains(t, index, `<div class="index-heading-main">`)
	assertContains(t, index, `<div class="index-heading-title">`)
	assertContains(t, index, `<div class="index-tools">`)
	assertContains(t, index, `<form class="article-search" action="/" method="get" role="search" data-search-form>`)
	assertContains(t, index, `<button class="article-search-toggle" type="button" aria-label="展开搜索" aria-expanded="false">`)
	assertContains(t, index, `<span class="article-search-icon" aria-hidden="true"></span>`)
	assertContains(t, index, `<span class="sr-only">搜索</span>`)
	assertContains(t, index, `<div class="article-search-panel" hidden>`)
	assertContains(t, index, `<button class="article-search-submit" type="submit" aria-label="提交搜索">`)
	assertContains(t, index, `<span class="article-search-submit-icon" aria-hidden="true"></span>`)
	assertNotContains(t, index, `aria-expanded="false">搜索</button>`)
	assertNotContains(t, index, `type="submit">确认</button>`)
	assertNotContains(t, index, `</section>
    <form class="article-search"`)
}

func TestArchivePagesUseAccurateKickersAndLinkImageFallback(t *testing.T) {
	category := readProjectFile(t, "views", "category.jet.html")
	assertContains(t, category, `<p class="page-kicker">Categories</p>`)
	assertNotContains(t, category, `<p class="page-kicker">Archive</p>`)

	link := readProjectFile(t, "views", "link.jet.html")
	assertContains(t, link, `<div class="link-card-image">`)
	assertContains(t, link, `<span class="link-card-fallback" aria-hidden="true"></span>`)
	assertContains(t, link, `<img src="{{ .IconURL }}" alt="" loading="lazy" onload="this.nextElementSibling.hidden=true;" onerror="this.hidden=true;">`)
	assertNotContains(t, link, `style="background-image: url('{{ .IconURL }}');"`)
}

func TestSearchControlHasCollapsedAndExpandedStates(t *testing.T) {
	css := readStyle(t)
	assertContains(t, css, `.index-heading-main {`)
	assertContains(t, css, `.index-tools {`)
	assertContains(t, css, `.article-search {`)
	assertContains(t, css, `width: 40px;`)
	assertContains(t, css, `overflow: hidden;`)
	assertContains(t, css, `transition: width 0.22s ease;`)
	assertContains(t, css, `.article-search.active {
    width: min(340px, calc(100vw - 40px));
}`)
	assertContains(t, css, `.article-search-icon::after`)
	assertContains(t, css, `.article-search.active .article-search-toggle {
    display: none;
}`)
	assertContains(t, css, `.article-search.closing .article-search-toggle {
    display: none;
}`)
	assertContains(t, css, `.article-search-panel[hidden]`)
	assertContains(t, css, `.article-search.active .article-search-panel`)
	assertContains(t, css, `.article-search.closing .article-search-panel`)
	assertContains(t, css, `opacity: 0;`)
	assertContains(t, css, `transform: translateX(8px) scaleX(0.98);`)
	assertContains(t, css, `transition: opacity 0.22s ease, transform 0.22s ease;`)
	assertContains(t, css, `pointer-events: none;`)
	assertContains(t, css, `.article-search.active .article-search-toggle`)
	assertContains(t, css, `grid-template-columns: minmax(0, 1fr) 34px;`)
	assertContains(t, css, `justify-content: stretch;`)
	assertContains(t, css, `.index-tools.active {`)
	assertContains(t, css, `grid-column: 1 / -1;`)
	assertContains(t, css, `width: 100%;`)
	assertContains(t, css, `.sr-only {`)
}

func TestLinkCardsRenderImageFallbackWithoutBlankPanel(t *testing.T) {
	css := readStyle(t)
	assertContains(t, css, `.link-card-image img {`)
	assertContains(t, css, `object-fit: cover;`)
	assertContains(t, css, `.link-card-fallback {`)
	assertContains(t, css, `.link-card-fallback::after`)
	assertContains(t, css, `background: var(--surface-muted-color);`)
	assertNotContains(t, css, `.link-card-image img:not([hidden]) + .link-card-fallback`)
}

func TestMainScriptTogglesSearchExpansion(t *testing.T) {
	js := readProjectFile(t, "views", "static", "js", "main.js")
	assertContains(t, js, `document.querySelectorAll('[data-search-form]')`)
	assertContains(t, js, `setSearchState`)
	assertContains(t, js, `const finishSearchClose = () =>`)
	assertContains(t, js, `searchPanel.hidden = false`)
	assertContains(t, js, `searchPanel.hidden = true`)
	assertContains(t, js, `searchForm.classList.add('closing')`)
	assertContains(t, js, `searchPanel.addEventListener('transitionend'`)
	assertContains(t, js, `window.setTimeout(finishSearchClose`)
	assertContains(t, js, `searchToggle.setAttribute('aria-expanded', String(open))`)
	assertContains(t, js, `searchTools?.classList.toggle('active', open)`)
	assertContains(t, js, `new URLSearchParams(window.location.search)`)
	assertContains(t, js, `searchInput.focus()`)
	assertContains(t, js, `closeSearchIfEmpty`)
	assertContains(t, js, `searchInput.value.trim()`)
	assertContains(t, js, `document.addEventListener('pointerdown'`)
	assertContains(t, js, `!searchForm.contains(event.target)`)
	assertContains(t, js, `searchForm.addEventListener('focusout'`)
	assertContains(t, js, `document.activeElement`)
	assertContains(t, js, `event.key === 'Escape'`)
}

func readStyle(t *testing.T) string {
	t.Helper()
	css, err := os.ReadFile(projectPath("views", "static", "css", "style.css"))
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	return string(css)
}

func readProjectFile(t *testing.T, parts ...string) string {
	t.Helper()
	content, err := os.ReadFile(projectPath(parts...))
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Join(parts...), err)
	}
	return string(content)
}

func projectPath(parts ...string) string {
	return filepath.Join(append([]string{"..", ".."}, parts...)...)
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("style.css missing %q", needle)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("style.css should not contain %q", needle)
	}
}
