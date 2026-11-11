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

  // The theme scrolls the active entry into view after every page load, so
  // the navigation jumps on every navigation. The navigation is the same on
  // every page of a build, so a reader who scrolled it expects it to stay
  // where they left it: remember the entry they were looking at and put it
  // back.
  //
  // The anchor is an entry, not a pixel offset: the tree grows and shrinks
  // as sections open and close (the path to the current page is open), so
  // the same offset can show a different part of the tree on the next page.
  var SCROLL_KEY = 'docbuilder.nav.scroll';
  var scrolledByUser = false;

  function entryVisible(link) {
    // an entry inside a collapsed section is not on screen
    var li = link.closest ? link.closest('li') : null;
    li = li && li.parentElement ? li.parentElement.closest('li') : null;
    while (li) {
      var box = ownCheckbox(li);
      if (box && !box.checked) return false;
      li = li.parentElement ? li.parentElement.closest('li') : null;
    }
    return true;
  }

  function readScroll() {
    try {
      var raw = window.sessionStorage.getItem(SCROLL_KEY);
      if (!raw) return null;
      var anchor = JSON.parse(raw);
      return anchor && typeof anchor.index === 'number' ? anchor : null;
    } catch (e) {
      return null;
    }
  }

  function saveScroll() {
    // Only a position the reader chose counts. Without this the theme's
    // own scroll to the active entry would be saved and the navigation
    // would always jump to the current page.
    if (!scrolledByUser) return;
    var wrap = document.getElementById('R-content-wrapper');
    if (!wrap) return;
    var links = wrap.querySelectorAll('a[href]');
    var box = wrap.getBoundingClientRect();
    for (var i = 0; i < links.length; i++) {
      var r = links[i].getBoundingClientRect();
      if (r.bottom <= box.top || r.top >= box.bottom) continue;
      if (!entryVisible(links[i])) continue;
      try {
        window.sessionStorage.setItem(SCROLL_KEY, JSON.stringify({
          index: i,
          offset: Math.round(r.top - box.top)
        }));
      } catch (e) {
        /* storage disabled: the position is simply not remembered */
      }
      return;
    }
  }

  function keepScrollPosition() {
    var wrap = document.getElementById('R-content-wrapper');
    if (!wrap) return;

    // Any input on the navigation counts as the reader choosing a position:
    // the wheel, a touch, dragging the scrollbar rail (a pointer event on
    // the rail inside the wrapper), or the keyboard. The theme's own scroll
    // to the active entry is none of these, so it does not count.
    var markUser = function () {
      scrolledByUser = true;
    };
    wrap.addEventListener('wheel', markUser, { passive: true, once: true });
    wrap.addEventListener('touchmove', markUser, { passive: true, once: true });
    wrap.addEventListener('pointerdown', markUser, { once: true });
    wrap.addEventListener('keydown', markUser, { once: true });

    var anchor = readScroll();
    if (!anchor) return;

    // The same entry, wherever it ended up in the tree: the navigation is
    // the same on every page, so the index is stable.
    var target = wrap.querySelectorAll('a[href]')[anchor.index];
    if (!target) return;

    var restore = function () {
      if (!entryVisible(target)) return;
      var box = wrap.getBoundingClientRect();
      var r = target.getBoundingClientRect();
      wrap.scrollTop += (r.top - box.top) - anchor.offset;
    };

    // The theme scrolls the active entry into view a moment after the page
    // loads. Hold the reader's position until that has happened - setting
    // the scroll position from inside a scroll event does not stick.
    var done = false;
    var deadline = Date.now() + 5000;
    window.addEventListener('load', function () {
      window.setTimeout(function () {
        done = true;
      }, 300);
    });
    var hold = function () {
      if (done || scrolledByUser || Date.now() > deadline) return;
      restore();
      window.requestAnimationFrame(hold);
    };
    window.requestAnimationFrame(hold);
  }

  function render(html, slot, themeInitialised) {
    var doc = new DOMParser().parseFromString(html, 'text/html');
    var aside = doc.querySelector('#R-sidebar');
    if (!aside) return;
    var host = slot.parentElement;
    host.innerHTML = aside.innerHTML;
    applyActiveState(host, currentPath());
    keepScrollPosition();
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

  // The page is leaving: remember where the reader left the navigation.
  window.addEventListener('pagehide', saveScroll);
})();
