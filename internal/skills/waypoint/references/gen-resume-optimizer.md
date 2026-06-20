# Resume Optimizer

## Options
- **focus**: Technical | Soft | Both
- **min score target %**: 0–100
- **action verbs**: bool
- **industry**: optional

## Analyses
- **quick**: matchScore · matchedKeywords · missingKeywords · recommendations · quickWins · actionVerbs
- **gap**: technicalMatch · softSkillsMatch · domainMatch · overallRating
- **detailed**: execSummary · categoryBreakdown · keywordDensity · competitorComparison · improvementPlan

## Keyword buckets
**Technical**: langs (JS, TS, Python, Go, Rust) · frameworks (React, Vue, Next.js, Django) · DBs (Postgres, Mongo, Redis) · cloud (AWS, K8s, Terraform) · arch (REST, GraphQL, microservices, Kafka) · practices (Agile, CI/CD, TDD) · AI/ML · testing (Jest, Cypress, Playwright)

**Soft**: leadership, communication, collaboration, mentoring, stakeholder management

## Action verbs
Achieved · Built · Delivered · Designed · Developed · Improved · Implemented · Launched · Led · Optimized · Scaled · Spearheaded · Streamlined · Transformed

## Steps
1. Extract keywords from posting; compare against profile skills
2. Score per keyword bucket; calculate match %
3. Report ≤8 recommendations + quick wins

## Done when
- Every keyword bucket scored
- Match % calculated
- Recommendations reference specific missing keywords
- Action verbs provided if requested
