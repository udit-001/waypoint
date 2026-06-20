# Exa Search Patterns

If `exa` MCP connected. Enrich jobs with company/people intel.

## Company

```
exa_web_search_advanced_exa { query: "category:company <company>", numResults: 5 }
exa_web_search_exa { query: "<company> funding investors team", numResults: 5 }
exa_web_search_exa { query: "<company> engineering culture values", numResults: 5 }
```

Competitors:
```
exa_web_search_advanced_exa { query: "category:company companies like <company>", numResults: 8 }
```

## People

Hiring managers, recruiters, team leads:
```
exa_web_search_advanced_exa { query: "category:people engineering at <company>", numResults: 10 }
exa_web_search_advanced_exa { query: "category:people recruiter hiring at <company>", numResults: 10 }
exa_web_search_advanced_exa { query: "category:people <role> at <company>", numResults: 10 }
```

Non-LinkedIn supplement:
```
exa_web_search_exa { query: "<company> team page about us", numResults: 5 }
```

Deduplicate by LinkedIn URL or name + company.

## Save

Personalize cover letters, emails, interview prep. Save via `jobs update <id> --contact "..."` and `--notes`.
