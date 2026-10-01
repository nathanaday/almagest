---
name: thread-verify
description: "Verify a thread whose tasks are done: send a read-only agent to check the work against each requirement of the spec, file its report as a verification, and give each finding its outcome. Use for verify this, check the work, review against the spec, is it done, handle the findings, re-verify. Doing the tasks is thread-run; the wiki change that closes a verified thread is thread-close."
---

# thread-verify

A thread is verified by evidence, not by a claim. One round checks the done work against
every requirement of the spec and records what it found. The agent that did the work
does not judge it: a read-only agent does. Findings are expected. Each one gets an
outcome, and an outcome that changes the tasks or the spec leads to the next round.

Tools: `thread` (load, verify, finding, tasks, stub), `change`. Agents:
[thread-audit](../../agents/thread-audit.md). References:
[threads.md](../atlas/references/threads.md).

## Procedure

1. `thread` load with `thread`. `thread` verify refuses a thread with an open task:
   finish or drop each one first ([thread-run](../thread-run/SKILL.md)).
2. Send [thread-audit](../../agents/thread-audit.md) with the thread's id. It reads the
   spec, the tasks, and the commits, runs the checks, and returns one result per
   requirement and its findings. Verify a small thread yourself only when the user
   asks: follow the agent's procedure.
3. `thread` verify with `thread` and the report: `scope`, `results` (one per
   requirement: `pass` or `fail`, with the evidence), `findings`, `notes`. File what the
   agent returned. Do not turn a `fail` into a `pass`, and do not drop a finding.
4. Read `state.next`. With no finding and every requirement passing, the thread is
   verified.
5. Give each open finding one outcome, with `thread` finding. A task is for work the
   spec already requires. More than the spec requires is the user's decision.
   - a requirement fails, a rule is broken, or the work broke something → `outcome:
     task`, with `new_task` (its text and the requirements it serves) and `repository`.
     Then back to [thread-run](../thread-run/SKILL.md), and a new round after it;
   - it asks for more than the spec requires (one more test, a clearer README, a case
     the spec does not name) → ask the user: do it now (a `task`), put it in the spec (a
     `spec` change), keep it for later (a `stub`), or leave it (`accepted`);
   - the spec is wrong or lacks a requirement → ask the user. After the yes, revise the
     spec ([thread-spec](../thread-spec/SKILL.md)), then `outcome: spec`. The round is
     stale now; add the tasks the new spec needs, and verify again;
   - it is work for another thread → `outcome: stub`, with `text` or `link`;
   - it is knowledge (a fact about a tool, a result, a pitfall) → propose the wiki
     change now ([wiki-save](../wiki-save/SKILL.md)), then `outcome: knowledge` with
     `link` set to the change;
   - the user accepts it as it is → ask the user; then `outcome: accepted` with the
     `reason` in the user's words.
6. A requirement that fails needs a task that fixes it, or a change to the spec. It
   never needs a second opinion from the same evidence.
7. Repeat from step 1 until the thread is verified.

## Gate

The outcomes `spec` and `accepted` wait for the user's yes. Show the findings as a list,
each with the outcome you recommend. The outcomes `task`, `stub`, and `knowledge` need
no yes of their own; the wiki change has its own gate.

A run with no user present gives the outcome `task` only where a requirement fails, a
rule is broken, or the work broke something. It puts every other finding into one
stub, for the user to decide, and says so. It does not grow the thread on its own.

## Hand off

[thread-close](../thread-close/SKILL.md), when the thread is verified;
[thread-run](../thread-run/SKILL.md), when a finding became a task.
