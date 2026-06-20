# Exa Search Patterns

If `exa` MCP connected. Enrich jobs with company/people intel.

## Query tips

Exa uses embeddings not keywords. **Describe the page you want**, not the fact.

| Bad | Good |
|-----|------|
| `"Google"` | `"category:company Google tech Mountain View"` |
| `"person at Stripe"` | `"category:people senior engineer at Stripe"` |

- Natural phrases, not boolean operators
- Specific entity → `numResults: 5`, narrow filter → `10`, broad → `15`. Max 25
- Word order matters. Different phrasing → different results, use both
- 0 results → longer query or different angle (not synonym swap)

## Company

```
exa_web_search_advanced_exa { query: "category:company <company>", numResults: 5 }
exa_web_search_exa { query: "<company> funding investors team", numResults: 5 }
exa_web_search_exa { query: "<company> engineering culture values", numResults: 5 }
exa_web_search_advanced_exa { query: "category:company companies like <company>", numResults: 8 }
```

## People

```
exa_web_search_advanced_exa { query: "category:people engineering at <company>", numResults: 10 }
exa_web_search_advanced_exa { query: "category:people recruiter hiring at <company>", numResults: 10 }
exa_web_search_advanced_exa { query: "category:people <role> at <company>", numResults: 10 }
exa_web_search_exa { query: "<company> team page about us", numResults: 5 }
```

Deduplicate by LinkedIn URL or name+company.

## News & hiring signals

```
exa_web_search_exa { query: "category:news <company> announcement", numResults: 15 }
exa_web_search_exa { query: "<company> hiring layoffs freeze 2026", numResults: 10 }
exa_web_search_exa { query: "<company> expansion growth new office", numResults: 10 }
exa_web_search_exa { query: "<company> criticism concerns culture issues", numResults: 10 }
```

## Hidden relationships

Direct queries ("X clients") return articles not connections. Use indirect signals:

```
exa_web_search_exa { query: "<company> case study customer success story", numResults: 5 }
exa_web_fetch_exa { urls: ["https://<company>.com/customers", "https://<company>.com/partners"] }
exa_web_search_exa { query: "<person> conversation interview podcast guest", numResults: 8 }
exa_web_search_exa { query: "<person> worked with collaborated team together", numResults: 10 }
exa_web_search_exa { query: "<company> testimonial recommend switched from", numResults: 10 }
exa_web_search_exa { query: "<person> <company> years longtime known since", numResults: 10 }
```

## Deep-reading

Snippets often enough. Fetch full page when snippet lacks the value or you need a judgment call:

```
exa_web_fetch_exa { urls: ["<url1>", "<url2>"], maxCharacters: 5000 }
```

## Save into waypoint

Via `jobs update <id> --contact "..."` / `--notes`.
