document.addEventListener('DOMContentLoaded', () => {
  const root = document.documentElement;
  const colorModeToggle = document.querySelector('[data-myecho-color-mode-toggle]');

  if (root.hasAttribute('data-myecho-color-mode-theme') && colorModeToggle) {
    const colorModeStorageKey = 'myecho:color-mode';
    const darkModeQuery = window.matchMedia?.('(prefers-color-scheme: dark)');
    const themeColor = document.querySelector('[data-myecho-theme-color]');
    let sessionPreference = null;

    const readColorModePreference = () => {
      try {
        const preference = window.localStorage.getItem(colorModeStorageKey);
        return preference === 'light' || preference === 'dark' ? preference : null;
      } catch (_) {
        return null;
      }
    };

    const applyColorMode = (mode) => {
      const isDark = mode === 'dark';
      root.setAttribute('data-myecho-color-mode', isDark ? 'dark' : 'light');
      colorModeToggle.setAttribute('aria-pressed', String(isDark));
      colorModeToggle.setAttribute('aria-label', isDark ? '切换至浅色模式' : '切换至深色模式');
      const backgroundColor = window.getComputedStyle(root).getPropertyValue('--background-color').trim();
      themeColor?.setAttribute('content', backgroundColor || (isDark ? '#101714' : '#faf9f5'));
    };

    const resolvedMode = readColorModePreference()
      || root.getAttribute('data-myecho-color-mode')
      || (darkModeQuery?.matches ? 'dark' : 'light');
    applyColorMode(resolvedMode);

    colorModeToggle.addEventListener('click', () => {
      const nextMode = root.getAttribute('data-myecho-color-mode') === 'dark' ? 'light' : 'dark';
      sessionPreference = nextMode;
      try {
        window.localStorage.setItem(colorModeStorageKey, nextMode);
      } catch (_) {}
      applyColorMode(nextMode);
    });

    const handleSystemColorModeChange = (event) => {
      if (!sessionPreference && !readColorModePreference()) {
        applyColorMode(event.matches ? 'dark' : 'light');
      }
    };
    if (darkModeQuery?.addEventListener) {
      darkModeQuery.addEventListener('change', handleSystemColorModeChange);
    } else {
      darkModeQuery?.addListener?.(handleSystemColorModeChange);
    }
  }

  const menuToggle = document.querySelector('.menu-toggle');
  const navLinks = document.querySelector('.nav-links');

  if (menuToggle && navLinks) {
    const currentPath = window.location.pathname;
    navLinks.querySelectorAll('a').forEach((link) => {
      const href = link.getAttribute('href');
      const isCurrent = href === '/'
        ? currentPath === '/'
        : currentPath === href || currentPath.startsWith(`${href}/`);
      if (isCurrent) {
        link.setAttribute('aria-current', 'page');
      }
    });

    const setMenuState = (open) => {
      navLinks.classList.toggle('active', open);
      menuToggle.classList.toggle('active', open);
      menuToggle.setAttribute('aria-expanded', String(open));
      menuToggle.setAttribute('aria-label', open ? '关闭导航菜单' : '打开导航菜单');
    };

    menuToggle.addEventListener('click', () => {
      setMenuState(!navLinks.classList.contains('active'));
    });

    navLinks.querySelectorAll('a').forEach((link) => {
      link.addEventListener('click', () => setMenuState(false));
    });

    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape' && navLinks.classList.contains('active')) {
        setMenuState(false);
        menuToggle.focus();
      }
    });

    document.addEventListener('pointerdown', (event) => {
      if (!navLinks.classList.contains('active')) {
        return;
      }
      if (!navLinks.contains(event.target) && !menuToggle.contains(event.target)) {
        setMenuState(false);
      }
    });

    window.addEventListener('resize', () => {
      if (window.innerWidth > 720) {
        setMenuState(false);
      }
    });
  }

  document.querySelectorAll('[data-search-form]').forEach((searchForm) => {
    const searchTools = searchForm.closest('.index-tools');
    const searchToggle = searchForm.querySelector('.article-search-toggle');
    const searchPanel = searchForm.querySelector('.article-search-panel');
    const searchInput = searchForm.querySelector('[data-search-input]');
    const searchClose = searchForm.querySelector('.article-search-close');

    if (!searchToggle || !searchPanel || !searchInput) {
      return;
    }

    const searchCloseFallback = 260;
    let closeTimer;
    let shouldRestoreSearchFocus = false;

    const finishSearchClose = () => {
      if (searchForm.classList.contains('active')) {
        return;
      }

      window.clearTimeout(closeTimer);
      searchForm.classList.remove('closing');
      searchTools?.classList.toggle('active', false);
      searchPanel.hidden = true;
      if (shouldRestoreSearchFocus) {
        shouldRestoreSearchFocus = false;
        searchToggle.focus();
      }
    };

    const setSearchState = (open, shouldFocus = false, restoreFocus = false) => {
      window.clearTimeout(closeTimer);
      searchToggle.setAttribute('aria-expanded', String(open));
      searchToggle.setAttribute('aria-label', open ? '收起搜索' : '展开搜索');

      if (!open) {
        shouldRestoreSearchFocus = shouldRestoreSearchFocus || restoreFocus;
        searchForm.classList.remove('active');

        if (searchPanel.hidden) {
          finishSearchClose();
          return;
        }

        searchForm.classList.add('closing');
        closeTimer = window.setTimeout(finishSearchClose, searchCloseFallback);
        return;
      }

      shouldRestoreSearchFocus = false;
      searchPanel.hidden = false;
      searchForm.classList.remove('closing');
      searchForm.classList.add('active');
      searchTools?.classList.toggle('active', open);

      if (open && shouldFocus) {
        window.requestAnimationFrame(() => {
          searchInput.focus();
        });
      }
    };

    const closeSearchIfEmpty = () => {
      if (searchInput.value.trim()) {
        return;
      }
      setSearchState(false);
    };

    const keyword = new URLSearchParams(window.location.search).get('keyword');
    if (keyword) {
      searchInput.value = keyword;
      setSearchState(true);
    }

    searchToggle.addEventListener('click', () => {
      setSearchState(!searchForm.classList.contains('active'), true);
    });

    searchClose?.addEventListener('click', () => {
      setSearchState(false, false, true);
    });

    searchPanel.addEventListener('transitionend', (event) => {
      if (event.target === searchPanel && searchForm.classList.contains('closing')) {
        finishSearchClose();
      }
    });

    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape' && searchForm.classList.contains('active')) {
        setSearchState(false, false, true);
      }
    });

    document.addEventListener('pointerdown', (event) => {
      if (!searchForm.classList.contains('active')) {
        return;
      }
      if (!searchForm.contains(event.target)) {
        closeSearchIfEmpty();
      }
    });

    searchForm.addEventListener('focusout', () => {
      window.setTimeout(() => {
        if (!searchForm.contains(document.activeElement)) {
          closeSearchIfEmpty();
        }
      }, 0);
    });

    window.addEventListener('resize', () => {
      if (window.innerWidth <= 720) {
        return;
      }
      closeSearchIfEmpty();
    });
  });

  const updateScrollableTables = () => {
    document.querySelectorAll('#article_content table').forEach((table) => {
      const isScrollable = table.scrollWidth > table.clientWidth;
      if (isScrollable) {
        if (!table.hasAttribute('tabindex')) {
          table.tabIndex = 0;
          table.dataset.scrollTabindex = 'true';
        }
        if (!table.hasAttribute('aria-label')) {
          table.setAttribute('aria-label', '文章内容表格，可横向滚动查看');
          table.dataset.scrollLabel = 'true';
        }
        return;
      }
      if (table.dataset.scrollTabindex) {
        table.removeAttribute('tabindex');
        delete table.dataset.scrollTabindex;
      }
      if (table.dataset.scrollLabel) {
        table.removeAttribute('aria-label');
        delete table.dataset.scrollLabel;
      }
    });
  };

  updateScrollableTables();
  window.addEventListener('resize', updateScrollableTables);
});
