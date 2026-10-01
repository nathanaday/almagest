---
name: chord-create
description: "Split a goal that is too large for one thread into a chord: a set of threads, each one deliverable, with the order between them. Use for make a chord, plan this project, this is several pieces of work, break this goal down, what are the threads, put these threads in order, add a thread to the chord. One piece of work is thread-stub or thread-work; working through a chord is chord-work."
---

# chord-create

A chord is a goal that needs several threads: work in several repositories, or work
that runs over days, in an order. Each thread is one deliverable that can be specified,
done, and verified alone. The order is a graph: a thread may come after several, and
several may come after one. Two threads with no path between them can run in two
sessions at once.

Tools: `search`, `context`, `chord` (create, add, remove, order, load), `thread`
(stub, list). References: [threads.md](../atlas/references/threads.md).

## Procedure

1. Read the goal in the user's words. `search` the wiki for what it names, and
   `context` for each repository it touches, so the threads fit what exists.
2. `search` with `types: [stub, chord]` for threads and chords that cover part of the
   goal already. A thread that exists joins the chord; do not plant it twice.
3. Split the goal into threads. A thread:
   - delivers one thing a reviewer can check alone (a dataset, a trained model, a
     feature in one repository, a measured answer);
   - is small enough for one spec of a few requirements;
   - names the threads that must be verified before it can start, and no others. Add an
     order only where one thread needs what another delivers.
   Split where the work splits. Do not aim for a number of threads.
4. `chord` create with:
   - `title` and `text`: the goal, as what is true when every thread is closed;
   - `tags`: the chord's tags; each new stub takes them;
   - `threads`: for each new one `title`, `text` (what it delivers, in the user's words
     where they gave them), and `after`; for one that exists `thread` and `after`.
     `after` may name a thread of the same call by its title.
   One commit writes the chord and every stub. Each stub stays a stub: its spec comes
   when its turn comes ([thread-spec](../thread-spec/SKILL.md)).
5. To change a chord later: `chord` add (a stub joins), `chord` remove (a thread
   leaves), `chord` order (what each thread comes after). The user also redraws the
   order on the chord's canvas in Obsidian and saves it there; never edit the canvas
   file.

## Gate

Show the chord: its goal, and its threads in order, each with what it delivers and what
it comes after; say which can run at once. Link the chord and its canvas. Wait for the
user to agree or to change the order.

## Hand off

[chord-work](../chord-work/SKILL.md).
