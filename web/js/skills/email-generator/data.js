{
  "questions": [
    { "key": "tone", "label": "Email Tone", "type": "select", "options": ["Formal", "Casual", "Creative", "Concise"], "default": "Formal" },
    { "key": "includeSalary", "label": "Include salary expectations?", "type": "checkbox", "default": false },
    { "key": "focusArea", "label": "Primary focus", "type": "select", "options": ["Skills", "Experience", "Education", "Mixed"], "default": "Mixed" },
    { "key": "includePersonalNote", "label": "Add a personal note / hook?", "type": "checkbox", "default": true }
  ],
  "templates": {
    "application": {
      "subject": "Application for {{position}} at {{company}}",
      "greetings": ["Dear Hiring Team at {{company}}", "Hi {{company}} Team", "Hello {{company}}", "To the {{company}} Recruitment Team"],
      "body": "I am writing to express my interest in the {{position}} position at {{company}}.",
      "closings": ["Best regards", "Sincerely", "Cheers", "Yours faithfully"]
    },
    "followUp": {
      "subject": "Follow-Up: {{position}} Application",
      "body": "I wanted to follow up on my application for the {{position}} position."
    },
    "thankYou": {
      "subject": "Thank You - {{position}} Interview",
      "body": "Thank you for the opportunity to interview for the {{position}} position."
    },
    "networking": {
      "subject": "Connecting: {{position}} Interest at {{company}}",
      "greetings": ["Hi {{contactName}}", "Hello {{contactName}}", "Dear {{contactName}}"],
      "body": "I came across your profile and was impressed by your work at {{company}}.",
      "closings": ["Looking forward to connecting", "Best", "Cheers"]
    },
    "referralRequest": {
      "subject": "Referral Request: {{position}} at {{company}}",
      "body": "I am writing to respectfully request a referral for the {{position}} position at {{company}}."
    },
    "offerAcceptance": {
      "subject": "Offer Acceptance: {{position}} at {{company}}",
      "body": "I am delighted to accept the offer for the {{position}} position at {{company}}."
    },
    "rejectionResponse": {
      "subject": "Thank You — {{position}} at {{company}}",
      "body": "Thank you for informing me about your decision regarding the {{position}} position."
    }
  },
  "rules": {
    "maxSubjectLength": 78,
    "includeSignature": true,
    "includePhone": true,
    "defaultTone": "Formal",
    "maxPersonalNoteLength": 200
  }
}
