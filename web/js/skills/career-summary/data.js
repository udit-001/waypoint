{
  "questions": [
    { "key": "style", "label": "Summary style", "type": "select", "options": ["Standard", "Impact-Focused", "Technical", "Executive", "Entry-Level"], "default": "Standard" },
    { "key": "focus", "label": "Emphasize", "type": "select", "options": ["Skills", "Experience", "Achievements", "Balanced"], "default": "Balanced" },
    { "key": "length", "label": "Length", "type": "select", "options": ["Brief (1 sentence)", "Short (2-3 sentences)", "Detailed (paragraph)"], "default": "Short (2-3 sentences)" },
    { "key": "includeContact", "label": "Include contact info?", "type": "checkbox", "default": false }
  ],
  "templates": {
    "standard": {
      "structure": ["title", "years-experience", "core-skills", "achievements", "goal"]
    },
    "impact": {
      "structure": ["hook", "key-result", "value-proposition", "closing"]
    },
    "technical": {
      "structure": ["name-title", "competencies", "tools", "experience-highlights", "value-proposition"]
    },
    "executive": {
      "structure": ["leadership-brand", "strategic-impact", "team-scale", "vision-statement"]
    },
    "entryLevel": {
      "structure": ["education", "relevant-skills", "internship-projects", "motivation", "potential"]
    }
  },
  "rules": {
    "maxLengthPerVersion": 300,
    "includeContactInfo": false,
    "includeTargetRole": true,
    "defaultStyle": "Standard",
    "defaultFocus": "Balanced",
    "briefMaxChars": 120,
    "shortMaxChars": 280,
    "detailedMaxChars": 500
  }
}
