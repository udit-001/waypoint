{
  "questions": [
    { "key": "focusArea", "label": "Focus area", "type": "select", "options": ["Technical Skills", "Soft Skills", "Both"], "default": "Both" },
    { "key": "minScore", "label": "Minimum match score target (%)", "type": "range", "min": 0, "max": 100, "step": 10, "default": 50 },
    { "key": "includeActionVerbs", "label": "Include action verb suggestions?", "type": "checkbox", "default": true },
    { "key": "industry", "label": "Target industry (optional)", "type": "text", "default": "" }
  ],
  "templates": {
    "analysis": {
      "sections": ["matchScore", "matchedKeywords", "missingKeywords", "recommendations", "quickWins", "actionVerbs"]
    },
    "gapAnalysis": {
      "sections": ["technicalMatch", "softSkillsMatch", "domainMatch", "overallRating"]
    },
    "detailedReport": {
      "sections": ["executiveSummary", "categoryBreakdown", "keywordDensity", "competitorComparison", "improvementPlan"]
    }
  },
  "rules": {
    "minScoreThreshold": 50,
    "maxRecommendations": 8,
    "actionVerbs": ["Achieved", "Built", "Delivered", "Designed", "Developed", "Improved", "Implemented", "Launched", "Led", "Optimized", "Scaled", "Spearheaded", "Streamlined", "Transformed"],
    "commonKeywords": ["JavaScript", "TypeScript", "Python", "Java", "Go", "Rust", "C++", "Ruby", "PHP", "C#", "Kotlin", "Swift", "Scala",
      "React", "Angular", "Vue", "Svelte", "Next.js", "Node.js", "Express", "Django", "Flask", "Spring Boot", "FastAPI", "Rails",
      "SQL", "PostgreSQL", "MySQL", "MongoDB", "Redis", "Elasticsearch", "Cassandra", "DynamoDB", "Firebase", "Supabase",
      "AWS", "Azure", "GCP", "Docker", "Kubernetes", "CI/CD", "Git", "GitHub Actions", "Terraform", "Ansible", "Jenkins",
      "REST API", "GraphQL", "gRPC", "Microservices", "Event-Driven", "Kafka", "RabbitMQ",
      "Agile", "Scrum", "Kanban", "Jira", "Confluence", "Notion",
      "Machine Learning", "AI", "Data Science", "Deep Learning", "NLP", "Computer Vision", "TensorFlow", "PyTorch",
      "Communication", "Leadership", "Problem Solving", "Teamwork", "Mentoring", "Cross-functional", "Strategic Planning",
      "Unit Testing", "Integration Testing", "E2E Testing", "Jest", "Cypress", "Playwright",
      "System Design", "Architecture", "Performance Optimization", "Security", "Accessibility", "SEO"]
  }
}
