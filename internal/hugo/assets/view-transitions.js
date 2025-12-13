/**
 * View Transitions API - Smooth Navigation
 * Intercepts same-origin link clicks and uses View Transitions API for smooth page changes
 * Falls back gracefully for browsers without support
 */

(function() {
  'use strict';

  // Check for View Transitions API support
  if (!document.startViewTransition) {
    console.log('View Transitions API not supported - using default navigation');
    return;
  }

  // Cache for parsed documents to avoid re-parsing
  const docCache = new Map();

  /**
   * Fetch and parse a document
   */
  async function fetchDocument(url) {
    const cacheKey = url.toString();
    if (docCache.has(cacheKey)) {
      return docCache.get(cacheKey);
    }

    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`Failed to fetch ${url}: ${response.statusText}`);
    }

    const html = await response.text();
    const parser = new DOMParser();
    const doc = parser.parseFromString(html, 'text/html');
    
    docCache.set(cacheKey, doc);
    return doc;
  }

  /**
   * Navigate to a new page with view transition
   */
  async function navigateWithTransition(url) {
    try {
      const newDoc = await fetchDocument(url);
      
      // Start view transition
      const transition = document.startViewTransition(async () => {
        // Update document title
        document.title = newDoc.title;
        
        // Replace body content
        document.body.replaceWith(newDoc.body.cloneNode(true));
        
        // Re-run scripts (if needed by theme)
        // Note: This is a simple approach; more complex apps may need script re-evaluation
        
        // Update URL
        history.pushState(null, '', url);
      });

      await transition.finished;
    } catch (error) {
      console.error('Navigation with transition failed:', error);
      // Fallback to standard navigation
      window.location.href = url;
    }
  }

  /**
   * Check if a link should use view transitions
   */
  function shouldTransition(link) {
    if (!link || !link.href) return false;
    
    try {
      const url = new URL(link.href);
      
      // Only transition same-origin links
      if (url.origin !== location.origin) return false;
      
      // Skip links that open in new tabs
      if (link.target === '_blank') return false;
      
      // Skip download links
      if (link.hasAttribute('download')) return false;
      
      // Skip links with rel=external
      if (link.rel === 'external') return false;
      
      // Skip anchor-only links (same page)
      if (url.pathname === location.pathname && url.hash) return false;
      
      return true;
    } catch (e) {
      return false;
    }
  }

  /**
   * Handle click events
   */
  document.addEventListener('click', (e) => {
    const link = e.target.closest('a');
    
    if (!shouldTransition(link)) return;
    
    e.preventDefault();
    navigateWithTransition(link.href);
  });

  /**
   * Handle browser back/forward navigation
   */
  window.addEventListener('popstate', () => {
    if (!document.startViewTransition) {
      return;
    }

    document.startViewTransition(async () => {
      try {
        const newDoc = await fetchDocument(location.href);
        document.title = newDoc.title;
        document.body.replaceWith(newDoc.body.cloneNode(true));
      } catch (error) {
        console.error('Popstate transition failed:', error);
        window.location.reload();
      }
    });
  });

  console.log('View Transitions enabled');
})();
