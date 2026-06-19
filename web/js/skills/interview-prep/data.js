{
  "questions": [
    { "key": "interviewType", "label": "Interview type", "type": "select", "options": ["Phone Screen", "Technical", "Behavioral", "System Design", "Final Round", "Full Prep"], "default": "Full Prep" },
    { "key": "difficulty", "label": "Difficulty level", "type": "select", "options": ["Beginner", "Intermediate", "Advanced"], "default": "Intermediate" },
    { "key": "numQuestions", "label": "Number of questions", "type": "select", "options": ["5", "8", "10", "15"], "default": "10" },
    { "key": "includeAnswers", "label": "Include sample answers and talking points?", "type": "checkbox", "default": true }
  ],
  "templates": {
    "questions": {
      "sections": ["general", "technical", "behavioral", "role-specific", "research-checklist", "company-specific"]
    },
    "sampleAnswers": {
      "sections": ["introduction", "experience-pitch", "challenge-framework", "skills-to-highlight", "questions-to-ask"]
    },
    "phoneScreen": {
      "sections": ["self-intro", "motivation", "background-check", "logistics", "candidate-questions"]
    },
    "systemDesign": {
      "sections": ["requirements", "estimation", "data-model", "api-design", "scalability", "tradeoffs"]
    }
  },
  "rules": {
    "questionsPerLevel": { "Junior": 8, "Mid-Level": 10, "Senior": 12, "Lead": 15 },
    "includeCompanyResearch": true,
    "includeRoleSpecific": true,
    "maxQuestionsPerSection": 5,
    "defaultInterviewType": "Full Prep",
    "defaultNumQuestions": 10
  }
}
