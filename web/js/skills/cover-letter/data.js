{
  "questions": [
    { "key": "tone", "label": "Cover Letter Tone", "type": "select", "options": ["Formal", "Casual", "Creative", "Executive"], "default": "Formal" },
    { "key": "length", "label": "Length", "type": "select", "options": ["Short", "Medium", "Detailed"], "default": "Medium" },
    { "key": "highlightSkill", "label": "Skill to emphasize (optional)", "type": "text", "default": "" },
    { "key": "includeEducation", "label": "Include education section?", "type": "checkbox", "default": true }
  ],
  "templates": {
    "formal": {
      "greeting": "Dear Hiring Manager",
      "structure": ["address-block", "date", "salutation", "introduction", "experience", "skills", "education", "closing", "signature"],
      "tone_adjectives": ["proven", "established", "demonstrated", "seasoned"]
    },
    "casual": {
      "greeting": "Hi there",
      "structure": ["intro", "highlights", "fit", "culture", "closing"],
      "tone_adjectives": ["passionate", "enthusiastic", "driven", "curious"]
    },
    "creative": {
      "greeting": "To the {{company}} Team",
      "structure": ["hook", "story", "skills", "vision", "call-to-action"],
      "tone_adjectives": ["innovative", "bold", "imaginative", "dynamic"]
    },
    "executive": {
      "greeting": "Dear {{company}} Leadership Team",
      "structure": ["title-reference", "strategic-intro", "leadership", "vision", "cultural-fit", "closing"],
      "tone_adjectives": ["strategic", "visionary", "impact-driven", "transformational"]
    }
  },
  "rules": {
    "maxLength": 500,
    "includeContactInfo": true,
    "includeEducation": true,
    "maxShortLength": 250,
    "maxDetailedLength": 800,
    "defaultTone": "Formal",
    "defaultLength": "Medium"
  }
}
