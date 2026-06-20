# Resume Optimizer

Scores resume vs a job posting, finds keyword gaps, suggests fixes.

## Options

- **focus**: Technical | Soft | Both
- **min score target %**: 0–100
- **action verbs**: bool
- **industry**: optional

## Analyses

- **quick**: matchScore · matchedKeywords · missingKeywords · recommendations · quickWins · actionVerbs
- **gap**: technicalMatch · softSkillsMatch · domainMatch · overallRating
- **detailed**: execSummary · categoryBreakdown · keywordDensity · competitorComparison · improvementPlan

## Steps

1. `waypoint jobs get <id>` — posting in `url` or `notes`
2. `waypoint profile show --json` — pull skills, experience
3. Extract keywords from posting; compare against profile
4. Score per keyword bucket; calculate match %
5. Report ≤8 recommendations + quick wins

**Done when**: every keyword bucket scored, match % calculated, recommendations reference specific missing keywords, action verbs provided if requested.

## Keyword buckets

**Technical**: langs (JS, TS, Python, Go, Rust) · frameworks (React, Vue, Next.js, Django) · DBs (Postgres, Mongo, Redis) · cloud (AWS, K8s, Terraform) · arch (REST, GraphQL, microservices, Kafka) · practices (Agile, CI/CD, TDD) · AI/ML · testing (Jest, Cypress, Playwright).

**Soft**: leadership, communication, collaboration, mentoring, stakeholder management.

## Action verbs

Achieved · Built · Delivered · Designed · Developed · Improved · Implemented · Launched · Led · Optimized · Scaled · Spearheaded · Streamlined · Transformed.
