document.addEventListener('DOMContentLoaded', () => {
  const menuToggle = document.querySelector('.menu-toggle');
  const navLinks = document.querySelector('.nav-links');

  if (menuToggle && navLinks) {
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
      if (event.key === 'Escape') {
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

    if (!searchToggle || !searchPanel || !searchInput) {
      return;
    }

    const searchCloseFallback = 260;
    let closeTimer;

    const finishSearchClose = () => {
      if (searchForm.classList.contains('active')) {
        return;
      }

      window.clearTimeout(closeTimer);
      searchForm.classList.remove('closing');
      searchTools?.classList.toggle('active', false);
      searchPanel.hidden = true;
    };

    const setSearchState = (open, shouldFocus = false) => {
      window.clearTimeout(closeTimer);
      searchToggle.setAttribute('aria-expanded', String(open));
      searchToggle.setAttribute('aria-label', open ? '收起搜索' : '展开搜索');

      if (!open) {
        searchForm.classList.remove('active');

        if (searchPanel.hidden) {
          finishSearchClose();
          return;
        }

        searchForm.classList.add('closing');
        closeTimer = window.setTimeout(finishSearchClose, searchCloseFallback);
        return;
      }

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

    searchPanel.addEventListener('transitionend', (event) => {
      if (event.target === searchPanel && searchForm.classList.contains('closing')) {
        finishSearchClose();
      }
    });

    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') {
        setSearchState(false);
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
});
