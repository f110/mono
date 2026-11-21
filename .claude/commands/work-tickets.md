---
description: Pick up tickets of the given components from the Issue tracker and work on each one with its own subagent
argument-hint: "[component ...] (default: Issue Codexel)"
model: sonnet
---

# Goal

Collect the tickets the agent should work on from the Issue tracker (MCP `issue`) and process each ticket with its own background subagent in its own jj workspace. The human is usually away, so keep the work moving without waiting for answers, but stay inside the scope the human gave.

Components: `$ARGUMENTS`. If empty, use `Issue` and `Codexel`.

# Hard rules

- No interaction with external services: no push, no PR, nothing on github.com, no cluster operations, no changes to other repositories (e.g. ops). Record such work as remaining tasks in the ticket instead.
- Scope is the given components only. Do not start tickets of other components. The only exception is an agent-assigned ticket that blocks a ticket this sweep is actually working on (not one that blocks a human-assigned or idle ticket).
- One subagent per ticket. Parallel subagents each get their own jj workspace in a sibling directory. Never edit files in the default workspace.
- Never rewrite commits that are in review, run a revert, abandon changes, or forget workspaces you did not create in this session without asking the human.

# 1. Collect tickets

1. `who_am_i` → the agent actor. `list_actors` → the human is the actor whose id is the agent's `owner_id`. `list_components` → component ids.
2. `list_tickets` with `(status = OPEN OR status = IN_PROGRESS) AND (component = A OR component = B)`. Quote names that contain `-`, e.g. `component = "protoc-ddl"`.
3. `get_ticket` for every candidate and classify:
    - Assigned to the agent: work on it.
    - Assigned to the human: work on it only if the newest comment is a human reply posted after the agent's hand-off. Otherwise it is waiting for the human; leave it untouched.
    - IN_REVIEW tickets are left alone.
    - For tickets you will work on, follow `relations`/`backlinks`. If a blocker is DONE, the ticket can proceed. An agent-assigned blocker of a ticket you work on is in scope.
4. Check existing workspaces with `jj workspace list` and `ls ..`. Comments may mention a workspace from a previous run.
5. Tell the human, in a short table, which tickets you start and which you skip with the reason. Then start the subagents without waiting.

# 2. Start one subagent per ticket

Use the `Agent` tool with `subagent_type: general-purpose` and `model: opus`; it runs in the background. Pass everything the subagent needs, because it starts cold. Fill in the following template for each ticket:

```
Work on ticket #<N> "<title>" (Component: <component>) in the Issue tracker. Repository: <repo root> (jj).
Read the ticket with get_ticket first, including all comments. <Summary of the previous state and of the human's latest reply, if any.>

Rules:
- Follow ~/.claude/CLAUDE.md and the repository's AGENTS.md (jj, Japanese output, English commit messages, Bazel, make update-deps / make gen).
- Workspace: create `../mono-<N>` from trunk() with `jj workspace add ../mono-<N> --name mono-<N> -r 'trunk()'`, or reuse <existing workspace> after moving it onto the latest trunk(). Work only inside it.
  <If a previous workspace directory is gone but its change remains: apply only that change's diff (`jj diff -r <change>`) onto trunk; do not copy whole files.>
- No external interaction (push, PR, github.com, clusters, other repositories).
- At the start set the ticket to IN_PROGRESS and the assignee to the agent (id <agent id>).
- Decisions: settle them yourself from docs/ui-conventions.md, the design docs under docs/, existing implementations and past examples. Never design something that deviates from the conventions. Record each decision and its basis in a ticket comment. Stop and hand back only when there is no basis at all and changing it later would be very costly. In that case comment the facts and concrete options, set the assignee to the human (id <human id>) and finish.
- Ticket comments: facts only, no commit IDs, no self-justification.
- If the package has tests, use Red/Green TDD. Build and test with Bazel. Update design docs when the behaviour changes. For UI changes, start the app and check the behaviour if possible.
- Commit when done. Message: English title only (a body only for non-obvious context). Trailers in this order: `Fixes: #<N>` (or `Refs: #<N>` if work remains), then `jj trailer --agent-session` (Agent, Agent-Session), then `Assisted-by: Claude:<model id>`. No Co-Authored-By.
- After committing, comment the result and the decisions, set the status to IN_REVIEW and the assignee to the human.
- Finally report briefly: what you did, the decisions, the change ID, the workspace, and anything still open.
```

# 3. While subagents run

- Do not poll their output files; completion is notified.
- When a subagent reports, check its commit: `jj log -r <change> --no-graph -T 'description'` and `jj diff -r <change> --stat`. Relay the report to the human in a few lines: result, decisions, what still needs human checking.
- When the human replies on a ticket later, resume the same subagent with `SendMessage` (it keeps its context) and include the reply and the decision policy. Do not start a new subagent for the same ticket.
- If the human removes a ticket from scope, stop its subagent with `TaskStop`, restore the ticket's status/assignee to what it was before this session, and remove only the empty workspace you created for it.

# 4. Wrap-up

1. Check whether commits made in parallel conflict with each other, especially when they touch the same files:
   ```
   jj new --no-edit <change A> <change B> -m tmp-merge-check
   jj resolve --list -r 'description(exact:"tmp-merge-check\n")'
   jj abandon 'description(exact:"tmp-merge-check\n")'
   ```
   Also look for cross-ticket gaps (e.g. a key added by one ticket missing from a help dialog added by another).
   Report conflicts and gaps and ask before rebasing or amending commits under review. If the human approves, let the subagent that owns the later ticket rebase and resolve, keeping its message and trailers.
2. Report a final table: ticket, result (committed / handed back / skipped), change ID, workspace, what the human needs to check. Also list leftover workspaces and changes.
