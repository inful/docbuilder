// The copy buttons of the permalink.
//
// The theme has a copy button for the code blocks; this one copies the
// permalink of the page - the markdown link and the HTML link.

(function () {
  'use strict';

  var FLASH_MS = 1500;

  // The clipboard API needs a secure context. The fallback works
  // wherever the browser still supports execCommand.
  function writeToClipboard(text) {
    if (navigator.clipboard && window.isSecureContext) {
      return navigator.clipboard.writeText(text);
    }
    var area = document.createElement('textarea');
    area.value = text;
    area.setAttribute('readonly', '');
    area.style.position = 'absolute';
    area.style.left = '-9999px';
    document.body.appendChild(area);
    area.select();
    try {
      document.execCommand('copy');
    } finally {
      document.body.removeChild(area);
    }
    return Promise.resolve();
  }

  document.addEventListener('click', function (event) {
    var target = event.target;
    if (!target || !target.closest) {
      return;
    }
    var button = target.closest('button[data-docbuilder-copy]');
    if (!button) {
      return;
    }

    var icon = button.querySelector('i');
    var iconClass = icon ? icon.className : '';
    var label = button.getAttribute('title');

    writeToClipboard(button.getAttribute('data-docbuilder-copy')).then(function () {
      if (icon) {
        icon.className = 'fas fa-check';
      }
      button.setAttribute('title', 'Copied!');
      window.setTimeout(function () {
        if (icon) {
          icon.className = iconClass;
        }
        button.setAttribute('title', label);
      }, FLASH_MS);
    });
  });
})();
