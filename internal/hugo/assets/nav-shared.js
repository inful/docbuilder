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

  // The theme binds to the sidebar's contents on DOMContentLoaded, which
  // happens before the fragment arrives. These are the theme's functions
  // that need the real navigation: the menu scrollbar (without it the
  // sidebar does not scroll), the collapsible-section handlers, the
  // switchers in the sidebar footer, the touch handlers, and the scroll to
  // the active entry.
  var THEME_INIT = [
    'initMenuScrollbar',
    'initSwipeHandler',
    'handleVariantSwitcher',
    'handleLanguageSwitcher',
    'handleHistoryClearer',
    'scrollToPositions'
  ];

  function initTheme() {
    for (var i = 0; i < THEME_INIT.length; i++) {
      var fn = window[THEME_INIT[i]];
      if (typeof fn !== 'function') continue;
      if (THEME_INIT[i] === 'initMenuScrollbar') {
        // Mounting a second scrollbar on the same element would fight the
        // first one; the theme's own call found no element to mount on.
        var wrap = document.getElementById('R-content-wrapper');
        if (wrap && wrap.classList.contains('ps')) continue;
      }
      try {
        fn();
      } catch (e) {
        /* one broken control must not take the rest of the sidebar down */
      }
    }
  }

  function init() {
    var slot = document.getElementById(SLOT_ID);
    if (!slot) return;
    var src = slot.getAttribute('data-nav-src');
    if (!src) return;
    fetch(src)
      .then(function (r) {
        if (!r.ok) throw new Error('nav fragment: ' + r.status);
        return r.text();
      })
      .then(function (html) {
        var doc = new DOMParser().parseFromString(html, 'text/html');
        var aside = doc.querySelector('#R-sidebar');
        if (!aside) return;
        var host = slot.parentElement;
        host.innerHTML = aside.innerHTML;
        applyActiveState(host, currentPath());
        initTheme();
      })
      .catch(function () {
        /* no nav without the fragment; the page still renders */
      });
  }

  // Wait for the theme to have initialised before injecting. The theme
  // binds to the sidebar's contents on DOMContentLoaded, so injecting
  // before that would make it bind to the real navigation and leave the
  // re-run above binding everything a second time.
  if (document.readyState === 'loading' || document.readyState === 'interactive') {
    document.addEventListener('DOMContentLoaded', init, { once: true });
  } else {
    init();
  }
})();
