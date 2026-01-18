# June Plugin Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform June into a Claude Code plugin with task-aware workflow skills.

**Architecture:** Add plugin structure (.claude-plugin/), create june-skills/ for custom skills, add Makefile targets to vendor superpowers and overlay customizations. Modify skills to use `june task` commands and add fresheyes integration.

**Tech Stack:** Go (CLI enhancement), Makefile (build process), Markdown (skills)

---

### Task 1: Add --note flag to task create

**Files:**
- Modify: `internal/cli/task.go:31-48`
- Modify: `internal/cli/task.go:95-160` (runTaskCreate function)
- Test: `internal/cli/task_test.go`

**Step 1: Write the failing test**

Add to `internal/cli/task_test.go`:

```go
func TestTaskCreateWithNote(t *testing.T) {
	// Use setupTestRepo helper (consistent with existing tests)
	cleanup := setupTestRepo(t)
	defer cleanup()

	cmd := newTaskCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"create", "Test task", "--note", "This is a note", "--json"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Parse JSON output to get task ID
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify note was set by listing the task
	out.Reset()
	cmd = newTaskCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list", result.ID, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Failed to list task: %v", err)
	}

	// Check output contains the note
	if !bytes.Contains(out.Bytes(), []byte("This is a note")) {
		t.Errorf("Expected note in output, got: %s", out.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestTaskCreateWithNote -v`
Expected: FAIL (--note flag not recognized)

**Step 3: Add --note flag to newTaskCreateCmd**

In `internal/cli/task.go`, modify `newTaskCreateCmd()`:

```go
func newTaskCreateCmd() *cobra.Command {
	var parentID string
	var note string
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "create <title> [titles...]",
		Short: "Create one or more tasks",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTaskCreate(cmd, args, parentID, note, outputJSON)
		},
	}

	cmd.Flags().StringVar(&parentID, "parent", "", "Parent task ID for creating child tasks")
	cmd.Flags().StringVar(&note, "note", "", "Set note on created task(s)")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	return cmd
}
```

**Step 4: Update runTaskCreate signature and implementation**

Modify `runTaskCreate` to accept and use the note:

```go
func runTaskCreate(cmd *cobra.Command, args []string, parentID, note string, outputJSON bool) error {
	// ... existing setup code ...

	for _, title := range args {
		id, err := generateUniqueTaskID(database)
		if err != nil {
			return fmt.Errorf("generate task ID: %w", err)
		}

		var notePtr *string
		if note != "" {
			notePtr = &note
		}

		task := db.Task{
			ID:        id,
			ParentID:  parentPtr,
			Title:     title,
			Status:    "open",
			Notes:     notePtr,
			RepoPath:  repoPath,
			Branch:    branch,
			CreatedAt: now,
			UpdatedAt: now,
		}
		// ... rest of function ...
	}
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/cli -run TestTaskCreateWithNote -v`
Expected: PASS

**Step 6: Run all tests**

Run: `make test`
Expected: All tests pass

**Step 7: Commit**

```bash
git add internal/cli/task.go internal/cli/task_test.go
git commit -m "feat(cli): add --note flag to task create"
```

---

### Task 2: Create plugin infrastructure

**Files:**
- Create: `.claude-plugin/plugin.json`
- Modify: `.gitignore`
- Modify: `Makefile`

**Step 1: Create plugin.json**

```bash
mkdir -p .claude-plugin
```

Create `.claude-plugin/plugin.json`:

```json
{
  "name": "june",
  "description": "Task-aware workflow skills with persistent state",
  "version": "1.0.0"
}
```

**Step 2: Update .gitignore**

Add to `.gitignore`:

```
.skill-cache/
```

**Step 3: Add build-skills target to Makefile**

Add to `Makefile`:

```makefile
# Superpowers vendoring
SUPERPOWERS_VERSION := v4.0.3
SUPERPOWERS_REPO := https://github.com/obra/superpowers

.PHONY: build-skills
build-skills:
	@# Fetch superpowers if not cached
	@[ -d .skill-cache/superpowers ] || git clone $(SUPERPOWERS_REPO) .skill-cache/superpowers
	@cd .skill-cache/superpowers && git fetch && git checkout $(SUPERPOWERS_VERSION)
	@# Clean and copy superpowers skills
	rm -rf skills/
	cp -r .skill-cache/superpowers/skills skills/
	@# Overlay June's custom skills (override)
	cp -r june-skills/* skills/
	@echo "Skills assembled: superpowers $(SUPERPOWERS_VERSION) + june overrides"

.PHONY: update-superpowers
update-superpowers:
	cd .skill-cache/superpowers && git fetch origin main && git log --oneline HEAD..origin/main
	@echo "Review changes above, then update SUPERPOWERS_VERSION and run 'make build-skills'"
```

**Step 4: Verify plugin.json is valid JSON**

Run: `cat .claude-plugin/plugin.json | jq .`
Expected: Valid JSON output

**Step 5: Commit**

```bash
git add .claude-plugin/plugin.json .gitignore Makefile
git commit -m "feat: add plugin infrastructure for Claude Code"
```

---

### Task 3: Create june-skills directory structure

**Files:**
- Create: `june-skills/writing-plans/SKILL.md`
- Create: `june-skills/executing-plans/SKILL.md`
- Create: `june-skills/subagent-driven-development/SKILL.md`
- Create: `june-skills/fresheyes/SKILL.md`
- Create: `june-skills/fresheyes/fresheyes-full.md`
- Create: `june-skills/fresheyes/fresheyes-quick.md`
- Create: `june-skills/review-pr-comments/SKILL.md`

**Step 1: Create directory structure**

```bash
mkdir -p june-skills/{writing-plans,executing-plans,subagent-driven-development,fresheyes,review-pr-comments}
```

**Step 2: Clone superpowers to .skill-cache (portable source)**

```bash
git clone https://github.com/obra/superpowers .skill-cache/superpowers
cd .skill-cache/superpowers && git checkout v4.0.3 && cd ../..
```

**Step 3: Copy superpowers skills as base for modification**

```bash
cp .skill-cache/superpowers/skills/writing-plans/SKILL.md june-skills/writing-plans/
cp .skill-cache/superpowers/skills/executing-plans/SKILL.md june-skills/executing-plans/
cp .skill-cache/superpowers/skills/subagent-driven-development/SKILL.md june-skills/subagent-driven-development/
```

**Step 4: Copy subagent-driven-development supporting files**

```bash
cp .skill-cache/superpowers/skills/subagent-driven-development/implementer-prompt.md june-skills/subagent-driven-development/
cp .skill-cache/superpowers/skills/subagent-driven-development/spec-reviewer-prompt.md june-skills/subagent-driven-development/
cp .skill-cache/superpowers/skills/subagent-driven-development/code-quality-reviewer-prompt.md june-skills/subagent-driven-development/
```

**Step 5: Clone fresheyes repo and copy skill**

```bash
git clone https://github.com/sky-xo/fresheyes /tmp/fresheyes-clone
cp /tmp/fresheyes-clone/skills/fresheyes/SKILL.md june-skills/fresheyes/
cp /tmp/fresheyes-clone/skills/fresheyes/fresheyes-full.md june-skills/fresheyes/
cp /tmp/fresheyes-clone/skills/fresheyes/fresheyes-quick.md june-skills/fresheyes/
rm -rf /tmp/fresheyes-clone
```

**Note:** If fresheyes repo is not yet public, copy from local path as fallback.

**Step 6: Create review-pr-comments skill**

The review-pr-comments skill is custom. Create it directly in june-skills/ by copying from the design doc or create fresh.

```bash
# If you have it locally:
cp ~/.claude/skills/review-pr-comments/SKILL.md june-skills/review-pr-comments/

# Otherwise, create it manually based on the skill content
```

**Step 7: Verify all files copied**

Run: `find june-skills -name "*.md" | wc -l`
Expected: At least 10 files (3 fresheyes + 4 subagent + 1 writing + 1 executing + 1 review-pr)

**Step 8: Commit**

```bash
git add june-skills/
git commit -m "feat: add june-skills directory with base skills"
```

---

### Task 4: Modify writing-plans for fresheyes integration

**Files:**
- Modify: `june-skills/writing-plans/SKILL.md`

**Step 1: Add header comment**

At the top of the frontmatter, add:

```yaml
---
name: writing-plans
description: Use when you have a spec or requirements for a multi-step task, before touching code
# Based on: superpowers v4.0.3
# Customization: Fresheyes review integration, june task persistence
---
```

**Step 2: Add Plan Review section before Execution Handoff**

Insert before the "## Execution Handoff" section:

```markdown
## Plan Review

After saving the plan:

**Step 1: Run quick fresheyes**

Always run a quick fresheyes self-review (baseline sanity check). Invoke the fresheyes skill with "quick" mode:

Use the Skill tool: `skill: "june:fresheyes", args: "quick on the plan"`

This runs a structured self-review checklist without spawning external agents.

**Step 2: Suggest based on plan size**

Check the plan line count and present options:

**If < 400 lines:**

"Plan saved to `docs/plans/<filename>.md` (N lines). Quick fresheyes complete.

Ready to proceed to execution?"

**If >= 400 lines:**

"Plan saved to `docs/plans/<filename>.md` (N lines). Quick fresheyes complete.

This is a larger plan. Recommend full fresheyes with 2x Claude for independent review.

1. **Run full fresheyes** (2x Claude, ~15 min) - recommended for plans this size
2. **Proceed to execution** - skip full review

Which approach?"

User can override either way.
```

**Step 3: Add Task Persistence section after Plan Review**

```markdown
## Task Persistence

After fresheyes review (if proceeding to execution), create June tasks:

```bash
# Create parent task for the plan
june task create "<Plan Title>" --json
# Returns: {"id": "t-xxxx"}

# Create child tasks for each numbered task in the plan
june task create "Task 1: <title>" --parent t-xxxx
june task create "Task 2: <title>" --parent t-xxxx
# ... for each task
```

Output to user:

```
Tasks created: t-xxxx (N children)

Run `june task list t-xxxx` to see task breakdown.
```

**Important:** The parent task ID (e.g., `t-xxxx`) must be provided to the execution skill (executing-plans or subagent-driven-development). Include it in the handoff message so the executor knows which task tree to read.
```

**Step 4: Commit**

```bash
git add june-skills/writing-plans/SKILL.md
git commit -m "feat(skills): add fresheyes review and june tasks to writing-plans"
```

---

### Task 5: Modify subagent-driven-development for model selection

**Files:**
- Modify: `june-skills/subagent-driven-development/SKILL.md`

**Step 1: Add header comment**

Update the frontmatter:

```yaml
---
name: subagent-driven-development
description: Use when executing implementation plans with independent tasks in the current session
# Based on: superpowers v4.0.3
# Customization: Model selection per step, conditional code quality review, june task persistence
---
```

**Step 2: Add Model Selection section after the title**

Insert after `# Subagent-Driven Development` heading, before "## When to Use":

```markdown
## Model Selection

| Step | Model | Rationale |
|------|-------|-----------|
| **Implementer** (simple task) | `haiku` | Fast, focused tasks with clear specs |
| **Implementer** (complex task) | `opus` | Ambiguous, multi-file, or architectural work |
| **Spec Reviewer** | `haiku` | Verification is pattern matching, not creative |
| **Code Quality Reviewer** | `opus` | Needs nuanced judgment on patterns/architecture |

**How to decide simple vs complex:**
- Simple: Single file, clear requirements, well-defined scope
- Complex: Multiple files, ambiguous requirements, architectural decisions

## Code Quality Review Guidelines

**Run code quality review for all tasks unless the change is purely mechanical.**

Skip code quality review ONLY when:
- Pure mechanical change with zero judgment calls
- Examples: import reordering, variable rename, formatting fix, moving unchanged code
- The implementer self-review is sufficient

**If there's any doubt → review.**

The final fresheyes review after all tasks provides a safety net for anything missed.
```

**Step 3: Update process to use june tasks**

Find the TodoWrite references and update:

Change:
```
"Read plan, extract all tasks with full text, note context, create TodoWrite"
```

To:
```
"Read plan, extract all tasks with full text, note context. The parent task ID should be provided by the user or found in the plan handoff (e.g., 'Tasks created: t-xxxx'). Read tasks with `june task list <parent-id> --json`."
```

Change:
```
"Mark task complete in TodoWrite"
```

To:
```
"Mark task complete: `june task update <task-id> --status closed --note 'Verified'`"
```

**Step 4: Commit**

```bash
git add june-skills/subagent-driven-development/SKILL.md
git commit -m "feat(skills): add model selection and june tasks to subagent-driven-development"
```

---

### Task 6: Modify executing-plans for june task integration

**Files:**
- Modify: `june-skills/executing-plans/SKILL.md`

**Step 1: Add header comment**

Update the frontmatter:

```yaml
---
name: executing-plans
description: Use when you have a written implementation plan to execute in a separate session with review checkpoints
# Based on: superpowers v4.0.3
# Customization: June task persistence instead of TodoWrite
---
```

**Step 2: Update Step 1 to use june tasks**

Change:
```markdown
4. If no concerns: Create TodoWrite and proceed
```

To:
```markdown
4. If no concerns: The parent task ID should be provided by the user or found in the plan handoff (e.g., 'Tasks created: t-xxxx'). Read tasks with `june task list <parent-id> --json` and proceed.
```

**Step 3: Update Step 2 for task status**

Add to the "For each task" section:

```markdown
For each task:
1. Mark as in_progress: `june task update <task-id> --status in_progress`
2. Follow each step exactly (plan has bite-sized steps)
3. Run verifications as specified
4. Mark as completed: `june task update <task-id> --status closed --note "Verified"`
```

**Step 4: Add Resume section**

Add new section after "## The Process":

```markdown
## Resuming After Compaction

If context was compacted, check task state:

```bash
june task list <parent-id>
```

Output shows:
```
t-a3f8  "Implement feature"  [in_progress]
  t-7bc2  "Add middleware"   [closed]
  t-9de1  "Write tests"      [in_progress]  ← resume here
  t-3fg5  "Update docs"      [open]
```

Find the first task with status `open` or `in_progress` and resume from there.
```

**Step 5: Commit**

```bash
git add june-skills/executing-plans/SKILL.md
git commit -m "feat(skills): add june task integration to executing-plans"
```

---

### Task 7: Build and verify skills assembly

**Step 1: Run build-skills**

```bash
make build-skills
```

Expected output:
```
Skills assembled: superpowers v4.0.3 + june overrides
```

**Step 2: Verify skill count**

```bash
ls skills/ | wc -l
```

Expected: 16 skills (14 from superpowers + fresheyes + review-pr-comments)

**Step 3: Verify june overrides applied**

```bash
grep "Based on: superpowers" skills/writing-plans/SKILL.md
```

Expected: Should show the June customization comment.

**Step 4: Verify fresheyes copied correctly**

```bash
ls skills/fresheyes/
```

Expected: SKILL.md, fresheyes-full.md, fresheyes-quick.md

**Step 5: Commit assembled skills**

```bash
git add skills/
git commit -m "feat: assemble skills from superpowers + june overrides"
```

---

### Task 8: Test plugin with Claude Code

**Step 1: Test plugin loads**

```bash
claude --plugin-dir .
```

Expected: Claude Code starts without plugin errors.

**Step 2: Verify skills are namespaced**

In Claude Code, type: `/june:`

Expected: Should show autocomplete for june:writing-plans, june:fresheyes, etc.

**Step 3: Test june task create with note**

```bash
./june task create "Test task" --note "Test note" --json
./june task list
```

Expected: Task created with note visible.

**Step 4: Commit any fixes**

```bash
git add -A
git commit -m "fix: address plugin testing issues"
```

---

### Task 9: Update documentation

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

**Step 1: Add plugin section to README**

Add after "Task Commands" section:

```markdown
## Claude Code Plugin

June is also a Claude Code plugin providing task-aware workflow skills.

### Installation

```bash
git clone https://github.com/sky-xo/june
cd june
claude --plugin-dir .
```

### Skills

June includes all superpowers skills plus customizations:

| Skill | Customization |
|-------|---------------|
| `june:writing-plans` | Fresheyes review integration, june task persistence |
| `june:executing-plans` | June task status tracking, resume after compaction |
| `june:subagent-driven-development` | Model selection (haiku/opus), conditional code quality review |
| `june:fresheyes` | Multi-agent code review (Claude/Codex/Gemini) |
| `june:review-pr-comments` | PR feedback workflow with approval gates |

### Building Skills

To rebuild after editing `june-skills/`:

```bash
make build-skills
```

To check for superpowers updates:

```bash
make update-superpowers
```
```

**Step 2: Update CLAUDE.md**

Add to "What is June?" section:

```markdown
June is also a Claude Code plugin. Run `claude --plugin-dir .` to use june:* skills.
```

**Step 3: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "docs: add plugin documentation"
```

---

## Summary

| Task | Description |
|------|-------------|
| 1 | Add --note flag to task create |
| 2 | Create plugin infrastructure |
| 3 | Create june-skills directory structure |
| 4 | Modify writing-plans for fresheyes integration |
| 5 | Modify subagent-driven-development for model selection |
| 6 | Modify executing-plans for june task integration |
| 7 | Build and verify skills assembly |
| 8 | Test plugin with Claude Code |
| 9 | Update documentation |

**Skills in June:**
- writing-plans (superpowers + fresheyes + june tasks)
- executing-plans (superpowers + june tasks)
- subagent-driven-development (superpowers + model selection + june tasks)
- fresheyes (custom)
- review-pr-comments (custom)

**Skills NOT in June (stay in ~/.claude/skills/):**
- design-review
- tool-scout
- webresearcher

**Estimated commits:** 9
