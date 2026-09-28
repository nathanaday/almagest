
# Staying Grounded on the Idea

The basic pitch to keep us grounded:

- In this agentic world, we are asked to multitask, but multitasking has yet to be fun for me
	- You either over-delegate to agents who are more willing to do it on your behalf, at the cost of being left out of the loop on whats happening (sending me a notification for human input at a point in time doesn't mean anything to me if I haven't been following the work until then)
	- Or you expend incredible energy keeping your documents from becoming stale while constantly kicking off new tasks
- The goal of Atlas is to make multitasking easier for people who still want to control the documentation, design, and deliverables
	- All your documents are in one vault
- Atlas is for people who want to iterate, revise, and work on long-term tasks. This requires a different approach compared to firing off slop and forgetting you even made it
	- Once you kick off a project, you're stuck with it. You can of course retire it, but you won't lose visibility of it unless you want to

---
# Version 1 Feedback

> We have been bouncing the version numbers around for a bit, so to clarify, Version 1 is the current (`3f3736dcaeb10a7093c25f3d30446376d3c022ce`) version of the Atlas plugin.

## What is working well

#### Thread Stubs, Callouts, Etc.

Having a documented thread lifecycle is helpful for seeing movement through features and picking up long work between sessions. It enforces a consistent documentation format rather than an agent dumping a new markdown file per decision, which are doomed to become stale since they are not (easily) associated with a point in time or thread of work.

#### Wiki Ingest

Ingesting sources into the wiki knowledge base has been a useful way to make sure I don't lose track of something.


## What I don't like

#### Path-Finding Work and Console Switching

I still find myself getting confused and disoriented when working in one atlas project that itself contains many codebase repo's, which then connects to another hub project. In my own head, I know which repo I want to open and work on, but I need to do some path-finding work to figure out where my agent should be called.

This comes from a belief that I should always be starting my claude agents in the repo that I want to work on. Perhaps this doesn't need to be true (tbd feature number). 

#### No live status or notifications

Maybe not a huge deal, but it's that affect of "what are my agents doing right now"? And it's the one thing I liked most about paperclip. You can still start agents in all your 


#### Messy .git nesting

In an effort to add many repo's to another atlas project, and version control everything, we inevitably run into nested git levels or having to make decisions about what should be version controlled and what should not.

