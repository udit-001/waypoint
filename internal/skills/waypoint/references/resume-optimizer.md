# Resume optimizer

Scores resume vs a job posting, finds keyword gaps, suggests fixes.

## Options
- focus: Technical|Soft|Both (Both)
- min score target %: 0–100 (50)
- action verbs: bool (yes)
- industry (optional)

## Analyses
- **quick**: matchScore · matchedKeywords · missingKeywords · recommendations · quickWins · actionVerbs
- **gap**: technicalMatch · softSkillsMatch · domainMatch · overallRating
- **detailed**: execSummary · categoryBreakdown · keywordDensity · competitorComparison · improvementPlan

## Steps
1. `waypoint get <id>` — posting in `url`/`notes`
2. read profile/resume
3. extract keywords, compare, score per category
4. report ≤8 recs + quick wins

## Keyword buckets
langs (JS, TS, Python, Go, Rust…) · frameworks (React, Vue, Next.js, Django…) · DBs (Postgres, Mongo, Redis…) · cloud (AWS, K8s, Terraform…) · arch (REST, GraphQL, microservices, Kafka…) · practices (Agile, CI/CD, TDD) · AI/ML · soft (leadership, comms) · testing (Jest, Cypress, Playwright).

Action verbs: Achieved, Built, Delivered, Designed, Developed, Improved, Implemented, Launched, Led, Optimized, Scaled, Spearheaded, Streamlined, Transformed.

Output: match % + category breakdown.
