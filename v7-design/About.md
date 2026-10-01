
This is a major design revision to the wiki structure and design concepts for this project.

The new changes build on lessons learned from the existing versions. This is by no means an exhaustive specification, but just some new design principles and ideas. Our task is to turn this into a detailed spec for building.

The guiding principle remains: **everything is a document, no hidden knowledge**. That is why I like this tool and would use it personally: I want the knowledge to be easy to build up, query, and have it transparently available. Linking a repository means creating a repository document. Opening a new thread means creating a thread document. Etc. There is no obfuscated, internal database behind the scenes, just your vault.

---

### Separation of Concerns

Some of the complication in the current version comes from trying to manage three main areas at once:
- Durable Storage
- Easy Navigation
- Agent tasks (search, skills)

So this has led to a blended ecosystem of tools where the separation of concerns are roughly split up:
- Obsidian vault (markdown files)
- Obsidian plugin + vault
- Plugin (Go CLI, MCP, skills)

But these tools all evolved seperately as we worked out the high level design, and now I'd like to become a bit stricter about what each tool does, make sure it stays narrow, and make sure the tool does it well.


### Area 1 - Durable Storage

By durable storage, we mean the contents of the wiki/ - this is the knowledge base for a vault.

We have organized the current vault layout to be easy for human navigation: we added a threads/ folder, areas/ folders in the wiki, obsidian bases, canvases... these are all nice but they could be better.

The problem with the recent areas/ structure: I recently suggested we build this new feature where wiki items and thread items are stored in dedicated areas/ subfolders for easier navigation, but this is a another case where the storage of items became confused with the navigation of these items. This was an oversight on my part, and we are going to remove the concept of filing items by folder/ categories.

Storage concerns are limited to:
- Making sure documents have all the information required for lookup
- Making sure the linking is not too easy to break
- Making sure derived views and lookups are accurate
- Making sure version control is clean and intuitive.

A new proposed structure is to keep **one single** document schema for everything. 

```
wiki/
- documents/ (all documents)
- assets/ (all raw ingested files from source)
```

What we have really been trying to capture this entire time is that a document has many different "types".

Document types:
- **source** - description of a source material and embedding of the material if applicable (pdf, image, docx), similar to clicking on a wikipedia asset image and you see the caption, source, image, description, date, etc.
- **repository** - a file with a callout card and data indicating it is a repository link that the agent can work with; should refresh with current git status if applicable; include brief description of the repository
- **topic** - a typical wiki article entry with information, links, sources, etc. We are also thinking of supporting topic types that resemble the old categories from the legacy wiki: concepts, entities, policies
- **stub** - a planted topic; in the past, this was the first step of turning into a thread spec specifically, but now a stub can turn into **anything**, a spec, an article, a source; it's truly just any hastily planted "let's do this someday" item
- **spec** - record of planning an action or change in the repository, or a description of how an important repository aspect or codebase should function - they get their own treatment beyond a typical topic article
- **event** - a record of something that happened in the vault; this is similar to the receipt entity in threads that served as an indication that some spec was executed; we are making this more flexible by allowing an event to mark states like "opened", "work started", "work continued", "work completed", "work re-opened", "work completed", so that by linking events to the relevant specs or documents you can construct a history of what the agents did

So each wiki document adopts a format that specifies:

```
---
id: vlt-972yvz
type: (one type)
name: // consider dropping? shouldnt the doc title alone serve this?
description: "brief description for search and query hits"
created: YYYY-MM-DD HH:MM:SS
updated: YYYY-MM-DD HH:MM:SS
last-refreshed: YYYY-MM-DD HH:MM:SS
categories: [list of one or many] 
---
```

The layout inside the document is unique to each type, so it's not necessary to define a layout option.

Still use good judgement and design to make the layout of each document type beautiful, intuitive, and feature rich!

Notable changes from previous versions: 
- we are removing the concept of "threads" as their own entity, now technically any document can be a thread of work. All documents can begin as stubs, and agents can help turn them into work in a repo, specs, articles, etc.


### Area 2 - Efficient Lookup and Ingestion

Now the task is still to let agents accurately ingest, categorize, and query the knowledge base. The tooling needs to take advantage of the categories, description, etc. to build a list of document hits for some query. These skills already seem quite good and mature, so we just need to adapt them to the new document schema style.

However, what should be nice now is that all searches are confined to "documents/" and the document type, category, and state can be provided as extra arguments to narrow down the search.


### Area 3 - Navigation and Display

The documents/ folder is design to store durable knowledge without being concerned about its ease of human navigation. We would like to still provide the intuitive, simple navigation as a set of derived views. Derived views are nice because they are based on the document sources and we can change the presentation layer over time as needed without major changes to the source structure

The derived views must be generated by code (using the plugin) and kept fresh, then displayed in obsidian. These are **not** copies of the source doc, they are just higher level views that let you find the source doc you want to open

The plugin already creates many of the nice to have functionalities we may consider using:
- Opening a folder opens a base page
- Base pages filter the documents you want

So some ideas for derived views:
- Tree style navigation that lets you drill down into categories and topics, let's you navigate base pages until you find the document you want
- View for showing all #todo items and stubs
- View for showing event timeline 
- Graph view + filtering colors (which we already have and looks great, some modifications may be required)






