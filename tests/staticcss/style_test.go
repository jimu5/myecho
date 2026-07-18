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

func TestArticleInteractionsPreserveAnchorsAndValidateComments(t *testing.T) {
	article := readProjectFile(t, "views", "article.jet.html")
	assertContains(t, article, `let id = heading.id;`)
	assertContains(t, article, `heading.id = id;`)
	assertContains(t, article, `window.requestAnimationFrame(function()`)
	assertContains(t, article, `window.addEventListener('scroll', scheduleReadingStateUpdate, { passive: true })`)
	assertContains(t, article, `name="author_name" autocomplete="name" maxlength="64" required`)
	assertContains(t, article, `name="author_email" type="email" autocomplete="email" maxlength="64" required`)
	assertContains(t, article, `name="author_url" type="url" autocomplete="url" maxlength="256"`)
	assertContains(t, article, `name="content" rows="5" maxlength="2000" required`)
	assertContains(t, article, `payload.msg || '提交失败，请稍后重试。'`)
}

func TestArticleProvidesContinuousReadingSharingCodeCopyAndReplies(t *testing.T) {
	article := readProjectFile(t, "views", "article.jet.html")
	assertContains(t, article, `<nav class="article-neighbors" aria-label="相邻文章">`)
	assertContains(t, article, `id="article-share-button"`)
	assertContains(t, article, `aria-live="polite"`)
	assertContains(t, article, `navigator.share({ title: document.title, url: window.location.href })`)
	assertContains(t, article, `navigator.clipboard.writeText(value)`)
	assertContains(t, article, `button.setAttribute('data-code-copy', '')`)
	assertContains(t, article, `copyText(code.textContent || '')`)
	assertContains(t, article, `data-comment-reply data-parent-id="{{ .ID }}"`)
	assertContains(t, article, `name="parent_id" type="hidden" value="0"`)
	assertContains(t, article, `parent_id: Number(formData.get('parent_id')) || 0`)
	assertContains(t, article, `clearReply(true)`)

	css := readStyle(t)
	assertContains(t, css, `.article-share-button {
    display: inline-flex;
    min-height: 44px;`)
	assertContains(t, css, `.article-neighbor {
    display: flex;`)
	assertContains(t, css, `.code-copy-button {
    min-width: 96px;
    min-height: 44px;`)
	assertContains(t, css, `.comment-reply-action {
    min-height: 44px;`)
	assertContains(t, css, `.article-neighbor-next {
        grid-column: 1;`)
}

func TestDefaultThemeHeadUsesMatchingThemeColor(t *testing.T) {
	head := readProjectFile(t, "views", "components", "common_head.jet.html")
	assertContains(t, head, `<meta name="theme-color" content="#faf9f5" data-myecho-theme-color>`)
	assertContains(t, head, `<meta name="keywords" content="{{ Settings.GetStringValue("SiteIndexMetaKeyword") }}">`)
	assertNotContains(t, head, `<meta name="keyword"`)
	assertNotContains(t, head, `<meta name="theme-color" content="#f6f8f4">`)
}

func TestArticleTocUsesArticleWidthAndFallsBackBeforeOverlap(t *testing.T) {
	css := readStyle(t)
	assertContains(t, css, `left: calc(50% + 440px)`)
	assertContains(t, css, `@media screen and (max-width: 1439px)`)
	assertContains(t, css, `.toc-container.active`)
	assertContains(t, css, `background: var(--glass-strong-color);`)

	article := readProjectFile(t, "views", "article.jet.html")
	assertContains(t, article, `id="article-toc"`)
	assertContains(t, article, `aria-controls="article-toc"`)
	assertContains(t, article, `firstControl.focus()`)
	assertContains(t, article, `closeToc(true)`)
	assertContains(t, article, `tocToggle.focus()`)
}

func TestDefaultThemeUsesLayeredEditorialLayout(t *testing.T) {
	css := readStyle(t)
	assertContains(t, css, `--radius-lg: 18px;`)
	assertContains(t, css, `--shadow-panel: 0 20px 52px var(--shadow-strong-color);`)
	assertContains(t, css, `--display-font: ui-serif`)
	assertContains(t, css, `radial-gradient(`)
	assertContains(t, css, `position: sticky;`)
	assertContains(t, css, `backdrop-filter: blur(18px) saturate(140%);`)
	assertContains(t, css, `gap: 18px;`)
	assertContains(t, css, `background: var(--glass-color);`)
	assertContains(t, css, `box-shadow: var(--shadow-panel);`)
	assertContains(t, css, `.article-block::before {
    position: absolute;`)
	assertContains(t, css, `.article-block-tags a,`)
	assertNotContains(t, css, `.article-block-tags span`)
	assertNotContains(t, css, `--shadow-panel: none;`)
	assertNotContains(t, css, `.article-block::before {
    display: none;`)
	assertContains(t, css, `#article_content {
    padding: 48px 0 0;`)
	assertContains(t, css, `background-color: transparent;`)
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
	assertContains(t, css, `width: 44px;`)
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
	assertContains(t, css, `grid-template-columns: minmax(0, 1fr) 44px 44px;`)
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
	assertContains(t, css, `background: linear-gradient(135deg, var(--surface-muted-color)`)
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

func TestMainScriptMarksCurrentNavigationAndClosesMobileMenu(t *testing.T) {
	js := readProjectFile(t, "views", "static", "js", "main.js")
	assertContains(t, js, `const currentPath = window.location.pathname`)
	assertContains(t, js, `currentPath === '/'`)
	assertContains(t, js, `link.setAttribute('aria-current', 'page')`)
	assertContains(t, js, `!navLinks.contains(event.target) && !menuToggle.contains(event.target)`)
	assertContains(t, js, `menuToggle.focus()`)
	assertContains(t, js, `shouldRestoreSearchFocus`)
	assertContains(t, js, `setSearchState(false, false, true)`)
	assertContains(t, js, `searchToggle.focus()`)

	css := readStyle(t)
	assertContains(t, css, `#nav ul li a[aria-current="page"]`)
}

func TestMobileControlsAndLongContentStayUsable(t *testing.T) {
	css := readStyle(t)
	assertContains(t, css, `outline: 3px solid var(--link-hover-color);`)
	assertContains(t, css, `[tabindex]:focus-visible`)
	assertContains(t, css, `.article-block-actions a {
    display: inline-flex;
    min-height: 44px;`)
	assertContains(t, css, `.article-search-submit {
    display: inline-flex;
    width: 44px;`)
	assertContains(t, css, `.article-search input {
        font-size: 16px;`)
	assertContains(t, css, `text-overflow: ellipsis;`)
	assertContains(t, css, `.article-block-tags a,
.article-taxonomy-item {`)
	assertContains(t, css, `.archive-group a {
    min-width: 0;`)
	assertContains(t, css, `scroll-margin-top: 88px;`)
	assertContains(t, css, `#article_content iframe,
#article_content video,
#article_content audio,
#article_content svg,
#article_content canvas,
#article_content object,
#article_content embed {`)
	assertContains(t, css, `.article-block-tags a,
    .article-taxonomy-item {
        min-height: 44px;`)
	assertContains(t, css, `.empty-state {
        padding: 32px 20px;`)

	js := readProjectFile(t, "views", "static", "js", "main.js")
	assertContains(t, js, `const updateScrollableTables = () =>`)
	assertContains(t, js, `table.scrollWidth > table.clientWidth`)
	assertContains(t, js, `table.tabIndex = 0`)
	assertContains(t, js, `table.dataset.scrollTabindex = 'true'`)
	assertContains(t, js, `table.dataset.scrollLabel = 'true'`)
	assertContains(t, js, `文章内容表格，可横向滚动查看`)
}

func TestArticleResourcesLoadWithoutUnusedGlobalDayjs(t *testing.T) {
	article := readProjectFile(t, "views", "article.jet.html")
	headEnd := strings.Index(article, `</head>`)
	prismCSS := strings.Index(article, `href="/static/lib/prism/prism.css"`)
	prismJS := strings.Index(article, `<script src="/static/lib/prism/prism.js" defer></script>`)
	if headEnd < 0 || prismCSS < 0 || prismCSS > headEnd || prismJS < 0 || prismJS > headEnd {
		t.Fatal("Prism resources must load from the article head")
	}

	head := readProjectFile(t, "views", "components", "common_head.jet.html")
	assertNotContains(t, head, `dayjs`)
}

func TestThemeCSSAndJSRenderAsTrustedRawContent(t *testing.T) {
	head := readProjectFile(t, "views", "components", "common_head.jet.html")
	script := readProjectFile(t, "views", "components", "theme_script.jet.html")
	assertContains(t, head, `{{ Theme.CSS | raw }}`)
	assertNotContains(t, head, `Theme.JS`)
	assertContains(t, script, `{{ Theme.JS | raw }}`)
	assertContains(t, script, `data-myecho-theme-runtime`)
	assertNotContains(t, head, `{{ Theme.CSS }}`)
}

func TestDefaultThemeColorModeInitializesBeforeStylesAndStaysScoped(t *testing.T) {
	head := readProjectFile(t, "views", "components", "common_head.jet.html")
	header := readProjectFile(t, "views", "components", "header.jet.html")
	initializerIndex := strings.Index(head, `data-myecho-color-mode-theme`)
	stylesheetIndex := strings.Index(head, `href="/static/css/style.css"`)
	if initializerIndex < 0 || stylesheetIndex < 0 || initializerIndex > stylesheetIndex {
		t.Fatal("default theme color mode must initialize before the stylesheet")
	}
	assertContains(t, head, `{{ if SupportsColorMode }}`)
	assertContains(t, head, `window.localStorage.getItem('myecho:color-mode')`)
	assertContains(t, head, `window.matchMedia('(prefers-color-scheme: dark)')`)
	assertContains(t, head, `data-myecho-color-mode`)
	assertContains(t, head, `data-myecho-theme-color`)

	assertContains(t, header, `{{ if SupportsColorMode }}`)
	assertContains(t, header, `data-myecho-color-mode-toggle`)
	assertContains(t, header, `type="button"`)
	assertContains(t, header, `aria-pressed="false"`)
}

func TestDefaultThemeColorModeUsesSemanticSurfacesAndSystemPreference(t *testing.T) {
	css := readStyle(t)
	js := readProjectFile(t, "views", "static", "js", "main.js")
	darkSelector := `:root[data-myecho-color-mode-theme][data-myecho-color-mode="dark"]`
	assertContains(t, css, `--glass-color:`)
	assertContains(t, css, `--glass-strong-color:`)
	assertContains(t, css, `--glass-soft-color:`)
	assertContains(t, css, `--header-background-color:`)
	assertContains(t, css, `--inline-code-background-color:`)
	assertContains(t, css, `.article-block {
    position: relative;`)
	assertContains(t, css, `background: var(--glass-color);`)
	assertContains(t, css, `background: var(--glass-strong-color);`)
	assertContains(t, css, `background: var(--glass-soft-color);`)
	assertContains(t, css, `background: var(--glass-muted-color);`)
	assertContains(t, css, `color: var(--blockquote-color);`)
	assertContains(t, css, `color: var(--inline-code-color);`)
	assertContains(t, css, darkSelector)
	if strings.LastIndex(css, darkSelector) < strings.LastIndex(css, `@media (prefers-reduced-motion: reduce)`) {
		t.Fatal("default theme dark palette must stay at the end of the stylesheet")
	}
	assertNotContains(t, css, `background: rgba(255, 254, 250`)
	assertNotContains(t, css, `background-color: rgba(255, 254, 250`)

	assertContains(t, js, `root.hasAttribute('data-myecho-color-mode-theme')`)
	assertContains(t, js, `window.localStorage.setItem(colorModeStorageKey, nextMode)`)
	assertContains(t, js, `'(prefers-color-scheme: dark)'`)
	assertContains(t, js, `darkModeQuery.addEventListener('change', handleSystemColorModeChange)`)
	assertContains(t, js, `colorModeToggle.setAttribute('aria-pressed', String(isDark))`)
	assertContains(t, js, `切换至浅色模式`)
	assertContains(t, js, `window.getComputedStyle(root).getPropertyValue('--background-color').trim()`)
	assertContains(t, js, `themeColor?.setAttribute('content'`)
	assertNotContains(t, css, `data-myecho-default-theme`)
}

func TestBundledThemePresetsUseSharedColorModeContract(t *testing.T) {
	for _, name := range []string{"paper.css", "night.css", "anime.css"} {
		css := readProjectFile(t, "views", "static", "css", "presets", name)
		assertContains(t, css, `--link-color:`)
		assertContains(t, css, `--display-font:`)
		assertContains(t, css, `:root[data-myecho-color-mode-theme][data-myecho-color-mode="dark"]`)
	}
}

func TestThemeScriptLoadsAfterPageDOMAndBeforePageScripts(t *testing.T) {
	include := `{{ include "components/theme_script" }}`
	for _, name := range []string{"index.jet.html", "category.jet.html", "tags.jet.html", "archive.jet.html", "link.jet.html", "article_password.jet.html", "article.jet.html"} {
		content := readProjectFile(t, "views", name)
		includeIndex := strings.Index(content, include)
		bodyIndex := strings.Index(content, "<body>")
		bodyEndIndex := strings.Index(content, "</body>")
		if includeIndex < bodyIndex || includeIndex > bodyEndIndex {
			t.Fatalf("%s theme script include must be inside body", name)
		}
		if name == "article.jet.html" || name == "article_password.jet.html" {
			if strings.Index(content[includeIndex+len(include):], "<script>") < 0 {
				t.Fatalf("%s theme script must load before page script", name)
			}
		}
	}
	article := readProjectFile(t, "views", "article.jet.html")
	if strings.Index(article, `class="toc-backdrop"`) > strings.Index(article, include) {
		t.Fatal("article theme script must load after article navigation DOM")
	}
}

func TestPublicTemplatesUseLocalizedDatesAndDynamicFooter(t *testing.T) {
	articles := readProjectFile(t, "views", "components", "articles.jet.html")
	assertContains(t, articles, `Format("2006年1月2日")`)
	assertNotContains(t, articles, `January`)

	article := readProjectFile(t, "views", "article.jet.html")
	assertContains(t, article, `Format("2006年1月2日 15:04")`)
	assertNotContains(t, article, `January`)

	archive := readProjectFile(t, "views", "archive.jet.html")
	assertContains(t, archive, `Format("1月2日")`)

	footer := readProjectFile(t, "views", "components", "footer.jet.html")
	assertContains(t, footer, `© {{ CurrentYear }}`)
	assertNotContains(t, footer, `© 2025`)

	pagination := readProjectFile(t, "views", "components", "pagination.jet.html")
	assertNotContains(t, pagination, `style="padding-right:`)
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
