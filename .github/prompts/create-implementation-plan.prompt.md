---
description: "Create a structured implementation plan with phases, tasks, requirements, and validation criteria"
name: create-plan
argument-hint: "Describe the feature, refactoring, or upgrade you want to plan"
agent: agent
tools:
  ['read/problems', 'read/readFile', 'agent', 'edit/createDirectory', 'edit/createFile', 'edit/editFiles', 'search', 'todo']
---

# Create Implementation Plan

You are creating a detailed implementation plan for: **${input:PlanPurpose}**

## Your Task

1. **Analyze the workspace** using #tool:workspace to understand:
   - Existing architecture and patterns in `internal/` directories
   - Current testing strategies in `test/` and `*_test.go` files
   - Coding conventions from `.github/copilot-instructions.md` and `docs/style_guide.md`
   - Similar existing implementations or patterns

2. **Create a new implementation plan file** in the `/plan/` directory following these specifications:
   - Filename: `[purpose]-[component]-[version].md`
   - Purpose prefixes: `upgrade|refactor|feature|data|infrastructure|process|architecture|design`
   - Example: `feature-auth-module-1.md`, `refactor-lint-workflow-1.md`
   - Version: Start with `-1.md`, increment if similar plans exist

3. **Generate the plan content** using the template structure below

## Plan Content Requirements

Your implementation plan must:
- Use **explicit, unambiguous language** with zero interpretation required
- Structure content as **machine-parseable formats** (tables, lists, structured data)
- Include **specific file paths**, function names, and exact implementation details
- Define all variables, constants, and configuration values explicitly
- Provide **complete context** within each task description
- Use **standardized prefixes** for all identifiers (REQ-, TASK-, SEC-, CON-, etc.)
- Include **validation criteria** that can be automatically verified
- Break work into **discrete, atomic phases** with measurable completion criteria

## Phase Architecture Rules

- Each phase must have **measurable completion criteria**
- Tasks within phases must be **executable in parallel** unless dependencies are specified
- All task descriptions must include specific file paths, function names, and exact implementation details
- No task should require human interpretation or decision-making
- Tasks should reference existing code patterns from the workspace when applicable

## Status Guidelines

Set initial status based on the plan's purpose:
- `Planned` (blue) - For future work not yet started
- `In progress` (yellow) - If implementing something already partially complete
- Status color codes: `Completed` (brightgreen), `In progress` (yellow), `Planned` (blue), `Deprecated` (red), `On Hold` (orange)

## Mandatory Template Structure

**Create the implementation plan file using this EXACT template structure:**

```markdown
---
goal: [Concise title describing the implementation plan's goal - derived from ${input:PlanPurpose}]
version: 1
date_created: [Current date in YYYY-MM-DD format]
last_updated: [Same as date_created]
owner: [Leave as "AI Generated" or specify if known]
status: 'Planned'
tags: [Relevant tags based on purpose: feature, refactor, upgrade, chore, architecture, migration, bug, test]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

[Write a 2-3 sentence introduction explaining:
- What this plan aims to achieve
- Why this work is needed
- High-level approach or strategy]

## 1. Requirements & Constraints

[List all requirements and constraints. Analyze workspace patterns to identify:
- Coding standards from copilot-instructions.md
- Existing patterns in similar components
- Testing requirements
- Performance constraints
- Security considerations]

- **REQ-001**: [Specific functional requirement with measurable criteria]
- **REQ-002**: [Another requirement]
- **SEC-001**: [Security requirement if applicable]
- **CON-001**: [Technical constraint, e.g., "Must maintain backward compatibility with existing API"]
- **GUD-001**: [Guideline to follow, e.g., "Follow Go naming conventions from STYLE_GUIDE.md"]
- **PAT-001**: [Pattern to follow, e.g., "Use same error handling pattern as internal/git/"]

## 2. Implementation Steps

[Create 2-5 logical phases. Each phase should be independently completable.]

### Phase 1: [Descriptive Phase Name]

**GOAL-001**: [Clear, measurable goal for this phase]

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-001 | [Specific task with file paths, function names, exact changes needed] | | |
| TASK-002 | [Another specific task - be explicit about what needs to change] | | |
| TASK-003 | [Include test creation/updates as separate tasks] | | |

### Phase 2: [Next Phase Name]

**GOAL-002**: [Clear, measurable goal for this phase]

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-004 | [Continue numbering sequentially across phases] | | |
| TASK-005 | [Each task should be atomic and testable] | | |

## 3. Alternatives

[List alternative approaches considered and why they were rejected:]

- **ALT-001**: [Alternative approach] - Rejected because [specific reason]
- **ALT-002**: [Another alternative] - Not chosen because [specific reason]

## 4. Dependencies

[List dependencies discovered from workspace analysis:]

- **DEP-001**: [External library/framework] - Version X.Y.Z required
- **DEP-002**: [Internal component] - Must be updated before this work
- **DEP-003**: [Tooling requirement] - e.g., "Requires golangci-lint v1.50+"

## 5. Files

[List all files that will be created, modified, or deleted:]

- **FILE-001**: `path/to/file.go` - Create new file for [purpose]
- **FILE-002**: `path/to/existing.go` - Modify to add [specific functionality]
- **FILE-003**: `path/to/test_file.go` - Create unit tests for [component]
- **FILE-004**: `docs/feature.md` - Add documentation

## 6. Testing

[Specify testing requirements based on workspace testing patterns:]

- **TEST-001**: Unit tests for [component] in `path/to/component_test.go`
  - Test case: [specific scenario]
  - Expected: [specific outcome]
- **TEST-002**: Integration test in `test/integration/feature_test.go`
  - Scenario: [end-to-end workflow]
  - Validation: [what to verify]
- **TEST-003**: Golden test for [output validation]
  - Golden file: `testdata/golden/expected_output.json`

## 7. Risks & Assumptions

[Identify potential risks and state assumptions:]

- **RISK-001**: [Specific risk] - Mitigation: [how to address]
- **RISK-002**: [Performance concern] - Mitigation: [monitoring strategy]
- **ASSUMPTION-001**: [Technical assumption] - Verify by [how to validate]
- **ASSUMPTION-002**: [Dependency assumption] - Document if incorrect

## 8. Related Specifications / Further Reading

[Link to related documentation found in workspace:]

- [Existing plan]: `/plan/related-plan-1.md`
- [Architecture doc]: `docs/explanation/architecture.md`
- [Style guide]: `docs/STYLE_GUIDE.md`
- [ADR]: `docs/adr/adr-xxx-relevant-decision.md`
```

## Final Steps

1. **Analyze** the workspace to gather context for ${input:PlanPurpose}
2. **Generate** the complete plan following the template EXACTLY
3. **Create** the file in `/plan/` with proper naming
4. **Validate** all sections are populated with specific, actionable content
5. **Present** the plan to the user for review

Now create the implementation plan file for: **${input:PlanPurpose}**
