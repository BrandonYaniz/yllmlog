# Rule System

Rules decide which lines are ignored, analyzed, grouped, escalated, summarized, or reported.

Rules are stored in SQLite and may include structured matchers and regex matchers.

## Matcher types

- contains
- prefix
- suffix
- regex
- field_equals
- field_contains
- rate_threshold
- sequence
- service_pattern

## Rule sources

- system_default
- llm_generated
- admin_created
- admin_override
- llm_modified

Admin-created and admin-modified rules take precedence over LLM-generated rules.
