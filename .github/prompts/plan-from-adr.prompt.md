---
description: "Create a structured implementation plan with phases, tasks, requirements, and validation criteria"
name: plan-from-adr
argument-hint: "Describe the feature, refactoring, or upgrade you want to plan"
agent: agent
tools:
  ['read/problems', 'read/readFile', 'agent', 'edit/createDirectory', 'edit/createFile', 'edit/editFiles', 'search', 'todo']
---
# Create Implementation Plan from ADR

You are creating a detailed implementation plan based on the Architectural Decision Record (ADR) located at: **${input:ADRPath}**.

## Inputs and Output

**Input**
- ADR path: **${input:ADRPath}**

**Output**
- Create a new plan file **next to the ADR** under `docs/adr/`.
- Filename format: `adr-[adr-number]-implementation-plan.md`
  - Example: for `adr-019-daemon-public-frontmatter-filter.md`, create `adr-019-implementation-plan.md`.

## Your Task

1. **Analyze the ADR document** to understand:
   - The architectural decision being made
   - The context and problem statement
   - The proposed solution and its implications
   - Any alternatives considered and their trade-offs

2. **Write the implementation plan** as a practical, step-by-step tracking tool that can be executed in order.

## Required Planning Rules

### Strict TDD

Plan and execute work using strict TDD (test-first).

### Acceptance Criteria Reminder

- All tests must pass
- All `golangci-lint` issues must be fixed

### Per-Step Validation and Progress Tracking

After each step in the plan, you must:

- Verify that all tests pass
- Verify that `golangci-lint` reports no issues
- Update the plan file to reflect progress
- Commit changes with a message that follows the Conventional Commits format

### Handling Ambiguities

If you encounter ambiguities or need to make decisions not covered in the ADR, document them in the plan file with clear justifications.

## Safety and Correctness Constraints

- Do not write code before stating assumptions.
- Do not claim correctness you haven't verified.
- Do not handle only the happy path.
- Under what conditions does this work?