# Quick Fresh-Eyes Review

Fast self-review using structured checklists. No external agents. You review your own work with deliberate discipline.

**Time commitment:** 2-5 minutes

## Process

### Step 1: Announce

Say: "Starting quick fresh-eyes review of [N] files. This will take 2-5 minutes."

### Step 2: Determine Scope

Review scope: {{REVIEW_SCOPE}}

Read the files/diff to review.

### Step 3: Security Checklist

| Vulnerability | What to Check |
|---------------|---------------|
| **SQL Injection** | All database queries use parameterized statements, never string concatenation |
| **XSS** | All user-provided content is escaped before rendering in HTML |
| **Path Traversal** | File paths are validated, `../` sequences rejected or normalized |
| **Command Injection** | Shell commands don't include unsanitized user input |
| **IDOR** | Resources are access-controlled, not just unguessable IDs |
| **Auth Bypass** | Every protected endpoint checks authentication and authorization |

### Step 4: Logic Checklist

| Error Type | What to Check |
|------------|---------------|
| **Off-by-one** | Array indices, loop bounds, pagination limits |
| **Race conditions** | Concurrent access to shared state, async operations |
| **Null/undefined** | Every `.` chain could throw; defensive checks present? |
| **Type coercion** | `==` vs `===`, implicit conversions |
| **State mutations** | Unexpected side effects on input parameters? |
| **Error swallowing** | Empty catch blocks, ignored promise rejections |

### Step 5: Business Rules Checklist

| Check | Questions |
|-------|-----------|
| **Calculations** | Do formulas match requirements exactly? Currency rounding correct? |
| **Conditions** | AND vs OR logic correct? Negations applied properly? |
| **Edge cases** | Empty input, single item, maximum values, zero values? |
| **Error messages** | User-friendly? Leak no sensitive information? |
| **Default values** | Sensible defaults when optional fields omitted? |

### Step 6: Performance Checklist

| Issue | What to Check |
|-------|---------------|
| **N+1 queries** | Loops that make database calls should be batched |
| **Unbounded loops** | Maximum iterations, timeout protection |
| **Memory leaks** | Event listeners removed, streams closed, references cleared |
| **Missing indexes** | Queries filter/sort on indexed columns? |
| **Large payloads** | Pagination implemented? Response size bounded? |

### Step 7: Fix Immediately

For each issue found:
1. Fix it now
2. Add test if not covered
3. Re-run tests

### Step 8: Report

```
## Quick Fresh-Eyes Complete

**Files reviewed:** [list]

**Issues found and fixed:** [N]
- [file:line] - [description of issue and fix]
- ...

**Tests:** [pass/fail status after fixes]
```
