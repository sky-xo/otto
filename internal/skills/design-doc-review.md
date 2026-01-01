# Design Doc Critique

Review the design doc at `{{.DocPath}}`

## IMPORTANT CONTEXT

This is a **design doc for future implementation** - it describes what we're going to build, not what exists today. The current codebase is different because we haven't implemented this yet. Do NOT compare to current code or call differences "drift".

## Your Task

Critique the design document itself:

### 1. Internal Consistency

- Do all sections agree with each other?
- Are there contradictions (e.g., schema says X but text says Y)?
- Does the data model support all the described behaviors?

### 2. Completeness

- Are there gaps where the doc says "we will do X" but doesn't specify how?
- Is the schema complete for the described features?
- Are edge cases covered (failures, timeouts, concurrent access)?

### 3. Feasibility

- Are there hidden complexity bombs?
- Will the described approach actually solve the stated problems?
- Are there simpler alternatives we should consider?

### 4. Schema Review

- Does each table have appropriate primary keys?
- Are there missing columns needed for described behaviors?
- Is derived state (computed from other fields) sufficient, or do we need explicit fields?
- What about ordering, status tracking, timestamps?

### 5. Edge Cases

- What happens when things fail?
- Concurrent access patterns?
- Recovery after crashes/restarts?

## Output Format

Provide numbered findings with:
- The issue
- Why it matters
- Suggested fix (if applicable)

Be critical. Find problems. Propose fixes. Don't just validate.

Use `june say` for your analysis when done.
