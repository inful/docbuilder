// Shared-mode sidebar: the nav is rendered once (into /sidebar.html) and injected
// here, so it is not duplicated into every page. The active entry is matched
// by href in the browser, which is what lets one fragment serve every page.
(function () {
  'use strict';

  var SLOT_ID = 'R-sidebar-menu';

  function normalize(path) {
    return path.charAt(path.length - 1) === '/' ? path : path + '/';
  }

  function currentPath() {
    return normalize(decodeURIComponent(window.location.pathname));
  }

  function ownCheckbox(li) {
    // the section toggle is a direct child; a descendant's toggle belongs to
    // a nested section and must not be checked by this entry
    for (var i = 0; i < li.children.length; i++) {
      var child = li.children[i];
      if (child.tagName === 'INPUT' && child.type === 'checkbox') return child;
    }
    return null;
  }

  function applyActiveState(root, path) {
    var links = root.querySelectorAll('a[href]');
    var i;
    for (i = 0; i < links.length; i++) {
      var href = links[i].getAttribute('href');
      if (!href || href.charAt(0) === '#' || /^[a-z]+:/i.test(href)) continue;
      var url;
      try {
        url = new URL(href, window.location.href);
      } catch (e) {
        continue;
      }
      if (normalize(url.pathname) !== path) continue;

      var li = links[i].closest('li');
      if (!li) continue; // the logo points at the home page and is not in the tree
      li.classList.add('active');

      var parent = li.parentElement ? li.parentElement.closest('li') : null;
      while (parent) {
        parent.classList.add('parent');
        var box = ownCheckbox(parent);
        if (box) box.checked = true;
        parent = parent.parentElement ? parent.parentElement.closest('li') : null;
      }
    }
  }

  function initTheme() {
    // The theme binds to the sidebar's contents on DOMContentLoaded, which
    // happens before the fragment arrives. Re-run the parts that need the
    // real navigation: the menu scrollbar (without it the sidebar does not
    // scroll at all), the swipe handlers, and the scroll to the active entry.
    if (typeof window.initMenuScrollbar === 'function') window.initMenuScrollbar();
    if (typeof window.initSwipeHandler === 'function') window.initSwipeHandler();
    if (typeof window.scrollToPositions === 'function') window.scrollToPositions();
  }

  // The fragment is the same on every page of a build, so the browser can
  // keep it and skip the download on every navigation. The cache is keyed
  // by the build id and the fragment URL: a new build (or another site on
  // the same origin) invalidates it.
  var CACHE_KEY = 'docbuilder.nav';

  function readCache(build, src) {
    try {
      var raw = window.localStorage.getItem(CACHE_KEY);
      if (!raw) return null;
      var entry = JSON.parse(raw);
      if (!entry || entry.build !== build || entry.src !== src) return null;
      return typeof entry.html === 'string' ? entry.html : null;
    } catch (e) {
      return null; // storage disabled or corrupt entry: fetch instead
    }
  }

  function writeCache(build, src, html) {
    try {
      window.localStorage.setItem(CACHE_KEY, JSON.stringify({
        build: build,
        src: src,
        html: html
      }));
    } catch (e) {
      /* over quota or storage disabled: the fetch path still works */
    }
  }

  function render(html, slot, themeInitialised) {
    var doc = new DOMParser().parseFromString(html, 'text/html');
    var aside = doc.querySelector('#R-sidebar');
    if (!aside) return;
    var host = slot.parentElement;
    host.innerHTML = aside.innerHTML;
    applyActiveState(host, currentPath());
    if (themeInitialised) initTheme();
  }

  // The theme binds to the sidebar on DOMContentLoaded. This script runs
  // before that (it is deferred, and the theme's script sits in the
  // footer), so a navigation restored from the cache is in place before the
  // theme looks at it - the theme then binds to the real navigation and
  // nothing has to be re-run. After DOMContentLoaded the theme has already
  // bound to the placeholder, so the injected navigation needs the theme's
  // sidebar initialisation re-run.
  var themeReady = document.readyState === 'complete';
  document.addEventListener('DOMContentLoaded', function () {
    themeReady = true;
  });

  function init() {
    var slot = document.getElementById(SLOT_ID);
    if (!slot) return;
    var src = slot.getAttribute('data-nav-src');
    if (!src) return;
    var build = slot.getAttribute('data-build') || '';

    var cached = readCache(build, src);
    if (cached !== null) {
      // Restore the navigation before the theme initialises, so the theme
      // binds to it itself and nothing has to be re-run afterwards.
      render(cached, slot, themeReady);
      return;
    }

    // No cached fragment: fetch it. The injection happens after the theme
    // has initialised against the placeholder, so the theme's own handlers
    // have to be re-run against the real navigation.
    fetch(src)
      .then(function (r) {
        if (!r.ok) throw new Error('nav fragment: ' + r.status);
        return r.text();
      })
      .then(function (html) {
        writeCache(build, src, html);
        render(html, slot, true);
      })
      .catch(function () {
        /* no nav without the fragment; the page still renders */
      });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
