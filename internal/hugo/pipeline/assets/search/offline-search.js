// The server-side search client.
//
// The theme's own offline search downloads the index of the whole site
// and searches it in the browser: on a large site that index is
// megabytes, it is fetched and parsed on every page view, and the
// search index is built on the reader's machine.
//
// This client asks the server instead. The index stays on the server,
// the browser receives the results and nothing else, and the search
// stays fast however large the site is.
//
// It replaces the theme's assets/js/offline-search.js and renders into
// the same markup, so the theme's search box and its styles are
// unchanged.

(function () {
  'use strict';

  const ENDPOINT = '/api/search';
  const DEBOUNCE_MS = 150;

  function ready(fn) {
    if (document.readyState !== 'loading') {
      fn();
    } else {
      document.addEventListener('DOMContentLoaded', fn);
    }
  }

  ready(function () {
    const searchInputs = Array.from(
      document.querySelectorAll('.td-search input'),
    );
    if (searchInputs.length === 0) {
      return;
    }

    const timers = new Map();

    //
    // Search
    //

    const search = async (input) => {
      const query = input.value.trim();
      if (query === '') {
        hide(input);
        return;
      }

      const limit = parseInt(input.dataset.offlineSearchMaxResults, 10) || 10;
      let results;
      try {
        const response = await fetch(
          ENDPOINT +
            '?q=' +
            encodeURIComponent(query) +
            '&limit=' +
            encodeURIComponent(limit),
        );
        if (!response.ok) {
          throw new Error('HTTP status ' + response.status);
        }
        const data = await response.json();
        results = data.results || [];
      } catch (error) {
        console.error('Could not search:', error);
        return;
      }

      render(input, query, results);
    };

    const scheduleSearch = (input) => {
      const pending = timers.get(input);
      if (pending) {
        clearTimeout(pending);
      }
      timers.set(
        input,
        setTimeout(() => {
          timers.delete(input);
          search(input);
        }, DEBOUNCE_MS),
      );
    };

    //
    // Render the results
    //

    const render = (targetInput, query, results) => {
      // Dispose the popover of the previous search.
      const existing = bootstrap.Popover.getInstance(targetInput);
      if (existing !== null) {
        existing.dispose();
      }

      const html = document.createElement('div');

      const header = document.createElement('div');
      Object.assign(header.style, {
        display: 'flex',
        justifyContent: 'space-between',
        marginBottom: '1em',
      });

      const headerTitle = document.createElement('span');
      headerTitle.textContent = 'Search results';
      headerTitle.style.fontWeight = 'bold';

      const closeButton = document.createElement('span');
      closeButton.classList.add('td-offline-search-results__close-button');

      header.append(headerTitle, closeButton);
      html.append(header);

      const searchResultBody = document.createElement('div');
      Object.assign(searchResultBody.style, {
        maxHeight: `calc(100vh - ${
          targetInput.getBoundingClientRect().top + 180
        }px)`,
        overflowY: 'auto',
      });
      html.append(searchResultBody);

      // The query language of the server is worth telling the reader
      // about: a search that cannot be narrowed is a search that
      // returns too much. A reader who already writes the language
      // does not need the reminder.
      if (!usesQueryLanguage(query)) {
        const hint = document.createElement('p');
        hint.classList.add('text-body-secondary', 'small', 'mt-3', 'mb-0');
        hint.textContent =
          'Tip: OR, NOT, "an exact phrase" and title: narrow the search.';
        html.append(hint);
      }

      if (results.length === 0) {
        const noResults = document.createElement('p');
        noResults.textContent = `No results found for query "${query}"`;
        searchResultBody.append(noResults);
      } else {
        results.forEach((result) => {
          const entry = document.createElement('div');
          entry.classList.add('mt-4');

          const entryRef = document.createElement('small');
          entryRef.classList.add('d-block', 'text-body-secondary');
          entryRef.textContent = result.url;

          const entryLink = document.createElement('a');
          entryLink.classList.add('d-block');
          entryLink.style.fontSize = '1.2rem';
          entryLink.href = result.url;
          entryLink.textContent = result.title;

          const entryExcerpt = document.createElement('p');
          entryExcerpt.textContent = result.excerpt;

          entry.append(entryRef, entryLink, entryExcerpt);
          searchResultBody.append(entry);
        });
      }

      targetInput.addEventListener(
        'shown.bs.popover',
        () => {
          document
            .querySelectorAll('.td-offline-search-results__close-button')
            .forEach((button) => {
              button.addEventListener('click', () => {
                targetInput.value = '';
                targetInput.dispatchEvent(new Event('change', { bubbles: true }));
              });
            });
        },
        { once: true },
      );

      const popover = new bootstrap.Popover(targetInput, {
        content: html,
        html: true,
        customClass: 'td-offline-search-results',
        placement: 'bottom',
      });
      popover.show();
    };

    const hide = (targetInput) => {
      const existing = bootstrap.Popover.getInstance(targetInput);
      if (existing !== null) {
        existing.dispose();
      }
    };

    //
    // The query language
    //

    // A query that already uses the query language needs no reminder
    // of it: the reader knows OR, NOT, a phrase and a field filter.
    const usesQueryLanguage = (query) => {
      if (query.includes('"')) {
        return true;
      }
      if (/\b(AND|OR|NOT)\b/.test(query)) {
        return true;
      }
      return /\b(title|body|tags|categories):/.test(query);
    };

    //
    // Register the handlers
    //

    searchInputs.forEach((input) => {
      input.addEventListener('input', () => {
        scheduleSearch(input);
      });

      input.addEventListener('change', () => {
        scheduleSearch(input);
      });

      // Prevent the form around the search box from reloading the page
      // when the reader presses enter.
      const form = input.closest('form');
      if (form) {
        form.addEventListener('submit', (event) => {
          event.preventDefault();
          event.stopPropagation();
        });
      }
    });
  });
})();
