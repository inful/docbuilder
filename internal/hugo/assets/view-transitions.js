/**
 * View Transitions API - Enhanced Navigation
 * The browser handles cross-page transitions automatically via CSS @view-transition rule.
 * This script is only needed if you want to customize transition behavior or handle edge cases.
 */

(function() {
  'use strict';

  // Check for View Transitions API support
  if (!document.startViewTransition) {
    console.log('View Transitions API not supported - using default navigation');
    return;
  }

  console.log('View Transitions enabled - cross-page transitions handled by CSS');
})();
