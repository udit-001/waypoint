# Exa Search Patterns

Use when `exa` MCP is connected. Enrich jobs with company/people intel before generating content.

## Company

```
exa_web_search_advanced_exa { query: "category:company <company>", numResults: 5 }
exa_web_search_exa { query: "<company> funding round investors team", numResults: 5 }
exa_web_search_exa { query: "<company> engineering culture values about", numResults: 5 }
```

Competitors/alternatives:
```
exa_web_search_advanced_exa { query: "category:company companies like <company>", numResults: 8 }
```

## People

Find hiring managers, recruiters, team leads:
```
exa_web_search_advanced_exa { query: "category:people engineering at <company>", numResults: 10 }
exa_web_search_advanced_exa { query: "category:people recruiter hiring at <company>", numResults: 10 }
exa_web_search_advanced_exa { query: "category:people <role> at <company>", numResults: 10 }
```

Supplement with non-LinkedIn:
```
exa_web_search_exa { query: "<company> team page employees about us", numResults: 5 }
```

Deduplicate by LinkedIn URL (canonical) or name + current company (fallback).

## Usage

Use findings to personalize cover letters, emails, interview prep. Save contacts via `jobs update <id> --contact "..."` and intel to `--notes`.
