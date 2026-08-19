// Markdown rendering for stored markdown (job notes, posting
// descriptions). One seam: marked for parsing, DOMPurify for
// sanitizing — posting descriptions are scraped from arbitrary web
// pages, so hostile HTML smuggled into a description must never
// reach {@html} unsanitized. Links open in a new tab (they point at
// external sites).

import { marked } from 'marked';
import DOMPurify from 'dompurify';

marked.setOptions({ gfm: true, breaks: true });

export function renderMarkdown(text) {
  if (!text) return '';
  try {
    const raw = marked.parse(String(text).replace(/\\n/g, '\n'));
    const clean = DOMPurify.sanitize(raw, {
      // Posting bodies need links and lists; no forms, iframes,
      // or event handlers ever. DOMPurify strips those anyway —
      // this allowlist documents intent and fails closed.
      ALLOWED_TAGS: [
        'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
        'p', 'br', 'hr', 'strong', 'em', 'b', 'i', 'u', 's', 'del',
        'ul', 'ol', 'li', 'blockquote', 'code', 'pre',
        'a', 'img', 'table', 'thead', 'tbody', 'tr', 'th', 'td',
        'sup', 'sub',
      ],
      ALLOWED_ATTR: ['href', 'title', 'alt', 'src'],
    });
    return clean.replace(/<a\s/g, '<a target="_blank" rel="noopener noreferrer" ');
  } catch {
    return '';
  }
}
