# Project Conventions for Claude

## graphify

This project has a graphify knowledge graph at graphify-out/.

Rules:

- Before answering architecture or codebase questions, read graphify-out/GRAPH_REPORT.md for god nodes and community structure
- If graphify-out/wiki/index.md exists, navigate it instead of reading raw files
- After modifying code files in this session, ask user to run `python -m graphify update .` to keep the graph current (AST-only, no API cost)

## Build, Verify, Test

Do not run any of these commands, ask the user to run them instead. Please give the exact commands for the user to run.