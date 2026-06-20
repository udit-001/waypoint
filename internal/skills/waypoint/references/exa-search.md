# Exa Search Patterns

If `exa` MCP connected. Enrich jobs with company/people intel.

## Query tips

Exa uses embeddings, not keywords. **Describe the page you want**, not the fact you need.

| Bad | Good |
|-----|------|
| `"Google"` | `"category:company Google tech company Mountain View"` |
| `"person at Stripe"` | `"category:people senior engineer at Stripe"` |

- Write natural grammatical phrases, not boolean operators
- Specific entity → `numResults: 5`, narrow filter → `10`, broad discovery → `15`
- Don't exceed 25. Need more coverage? Run different angles at 10-15
- Word order matters — `"Python async web scraping"` vs `"web scraping async Python"` can return different results. Use both for coverage
- If 0 results: make query longer/more specific, or try different angle (not synonym swap)

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

## News & hiring signals

Company news, layoffs, hiring announcements:
```
exa_web_search_exa { query: "category:news <company> announcement", numResults: 15 }
exa_web_search_exa { query: "<company> hiring layoffs firing freeze 2026", numResults: 10 }
exa_web_search_exa { query: "<company> new office expansion growth team", numResults: 10 }
```

Reactions/sentiment on company events:
```
exa_web_search_exa { query: "<company> reaction analysis commentary", numResults: 12 }
exa_web_search_exa { query: "<company> criticism concerns issues culture", numResults: 10 }
```

## Hidden relationships

Find connections that aren't explicitly listed. Direct queries ("X clients") return articles, not connections. Use indirect signals:

Start with subject's own platforms:
```
exa_web_search_exa { query: "<company> case study customer success story", numResults: 5 }
exa_web_fetch_exa { urls: ["https://<company>.com/customers", "https://<company>.com/partners"] }
```

Indirect signals — who knows who:
```
exa_web_search_exa { query: "<person> conversation interview podcast guest", numResults: 8 }
exa_web_search_exa { query: "<person> worked with collaborated team together", numResults: 10 }
exa_web_search_exa { query: "<company> testimonial recommend switched from", numResults: 10 }
```

Duration markers (high confidence — people don't fabricate decades):
```
exa_web_search_exa { query: "<person> <company> years longtime known since", numResults: 10 }
```

## Deep-reading

Search snippets often have enough (name, title, company). Fetch full page when:
- Snippet mentions what you need but lacks the value
- Need multiple fields from one rich source (team page, case study)
- Making a judgment call (is this genuine opinion or generic content?)

```
exa_web_fetch_exa { urls: ["<url1>", "<url2>"], maxCharacters: 5000 }
```

## Save

Personalize cover letters, emails, interview prep. Save via `jobs update <id> --contact "..."` and `--notes`.
