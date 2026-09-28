
Important! This should be treated as a new project. It should be built from scratch using lessons from Atlas V1 and select components as a guide. Backwards compatibility is NOT required. For all intents and purposes, it is a brand new plugin.


# Core Design Guidance

## Everything is a document

The idea is your obsidian vault contains a document for every idea, thread, plan, and fact. There are no obfuscated databases, hidden files, or internal mechanics. The agents work in plain view.

In this latest version, the unifying goal is more like "how do we closely integrate an agent plugin with obsidian". The high level goal is to get total visibility on project knowledge, agent sessions, and tasks (threads) using obsidian documents.

#### Threads

Anything the agent changes anything in a repository, it is a thread. If no thread exists for the task at hand, it must create one first and update it. Why?
- We need this for the agent session visibility view - the contract is that if an agent is working on something, there is a thread attached
- It aligns with the principle that everything is a document, including agent sessions and changes

Exception: for pure questions and exploration, no thread is required. 

#### Sessions

All agent sessions get a schema-enforced markdown document in the sessions/ directory. It seems better to accomplish this using agent hooks so it is more reliable. The agent can then fill in some details about the session (description, summary, progress) and link to the appropriate thread.

What it is not: a full transcript of the session. The session document is just a front-page of a session so the user can see "what is running now" and "what have we run". 

Subagents spawned from sessions should get their own session document with a link to the parent repository.



#### Repository Links

One of the hard parts about V1 was keeping track of the relationship between a wiki and a repository. Sometimes I would just include the entire repo in the same directory, but in other approaches it was a virtual link. 

I believe the difficulty stems in part from a lack of visibility--a single source of truth--where you could see: "this vault is working with these repo's"

Having a structured markdown document serving as the link itself will be helpful.




# Features


#### Easier navigation

Start the agent in the atlas vault. This loads the context required to use the atlas skills, mcp, and plugins. You can work on any of your repositories from this session since the atlas context chain let's you reach your repository's context.



#### Clearer Agent Activity

All agents working on a task register their activity using the plugin hooks. All tasks 


#### Consolidated Knowledge Base

Let's move on from this idea of linking kb's together and mirroring them. It's definitely functional but it's just not intuitive enough. It's good to have thought through it, as it could become a future feature if it seems necessary.


#### More Obsidian Integration

Create an obsidian plugin to add the custom colors, themes, callouts.

Tag the atlas agent on items @agent (maybe the plugin will highlight this)

Refresh events

Stretch? Create a dynamic sidebar with agent status and harness output, click on them to view their summary or chat with them. they can view your current document too.

I am resisting building a standalone web app for viewing and managing. First, there are plenty of good tools that do this. Second, this is not supposed to be a tool with a UI and UX learning curve. It's designed to work with your notes. 

Wire threads together in the canvas to indicate the order you want to phase everything. Attach notes, references, concerns, etc. in the thread document trailer


---
# Technicals


## Stack

- Agent plugin (MCP binary, skills)
- Obsidian plugin

Create a simple install experience:
- `npx install atlas-obsidian`
- May have to search and install the obsidian plugin separately


## Context and Entity Chain

- Vault Level (+ Atlas Context)
	- Area (+ Area context) ^ N
		- Repository (+ Repository context)

The context graph is only conceptual. There is no physical directory structure that needs to adhere or obey it, and there really cannot be one, since the leaf nodes are repositories that may exist anywhere on the file system and with their own repository system.

The purpose of the context graph is to make sure an atlas agent can begin in the atlas vault and find their way to the correct project for context. 

Examples:

- User: "I want to work on the p3 cloud front end"
- Agent navigation: 
	- `vault -> work (area) -> p3 (area) -> p3-cloud (repository)`

- User: "I want to work on a feature that affects the remote update system for all p3 services"
- Agent navigation: 
	- `vault -> work (area) -> p3 (area) -> p3-cloud (repository)`
	- `                                 |-> p3-edge (repository)`
	- `                                 |-> p3-vertex (repository)`


- User: "I want to make sure none of my software projects are affected by CEV vulnerability XYZ"
- Agent navigation:
	- `vault -> work (area) -> (all repo's)`
	- `vault -> software (area) -> (all repo's)`
	- `vault -> school (area) -> (all repo's)`


There is only one **vault** level. It is the highest namespace. Atlas does support any number of vaults, but a vault is self-contained. There is no connection to other vaults. This allows for some level of isolation, in case the user has two projects that ought not overlap by design (e.g. work, personal). 

A vault can have any number of **areas** (including 0) and at any level in the tree. An area is an appropriately broad label to apply to a cluster of projects (or other areas). It's user preference whether the vault space is broken into many nested areas (for easy routing, mapping) or to keep things super broad (simpler). 

>[!tip] Onboarding
>An onboarding script let's you set a preference if you want the agent to segment your project or if you want to do it yourself.
>"How do you want to organize your vault into areas?"
>1. I prefer a lot of areas for max organization
>2. I like to keep things simple
>3. I don't know yet, or I'll create them myself

The leaf nodes of the context graph is always a repository. These are intended to be .git repositories. They can exist anywhere on the file system. They have their own `AGENTS.md` or `CLAUDE.md` that an atlas agent will load and respect.







## Vault Components

```

/
- inbox/
- scratchpad/
- sessions/
- threads/
- wiki/

```



#### Inbox

Use the ingest skill to match the concept to the right destination:
- Threads
- Wiki
	- Concept
	- Policy
	- Entity


#### Wiki

```
/
/concepts
/entities
/meta (? still needed ?)
/policies
/sources
hot
index
log

```

One knowledge base for the entire vault. 

The knowledge base should be organized according to the context graph.

From the ground up:
- Map the repositories
- In the parent (area or vault), map the relationships between repositories
- Recurse until the top...

Begin by mapping the leaf level repositories in isolation. Then, an agent needs to compare the leaf level repositories for two tasks:
- Create a new entity to link similar concepts in two children wiki's
- Upgrade an entity from a child wiki into the parent

Some of this was already created for V1...

Query, Lint, Ingest engine should still work


Four main wiki page types:

1. Concept
	1. ...

2. Entity
	1. ...

3. Policy
	1. A claim about how things should be and why;
	2. A description involving convention, practice, or requirements

4. Source
	1. Ingested document summary / front page
#### Sessions

A structured (with schema) markdown file per atlas agent session. The agent should describe at a high level what it is working on, the status (running, done), and link to any relevant documents (like a thread)

#### Scratchpad

An area to type up any notes or ideas, has no schema enforcement or ingestion path, just for the user.



## Retire / Deprecate

- Drop "signals" - we don't use this (unless the agent seems to need it for functionality?)
- Drop "Phases" in the threads system - we have a different idea for this long term but are not ready for it yet
- Drop "Plans" in the threads system - we are now using "tasks" since one spec can spawn multiple tasks

