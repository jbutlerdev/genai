# GenAI

This repository contains a wrapper for common Generative AI providers.

It also provides tools for use with generative AI.


## Providers



### Currently Supported

- Gemini

### Planned

- Anthropic
- OpenAI
- Ollama
- Azure

## Tools

Tools are provided by category. You can choose to pass a single tool or a category of tools to a model.

### Current Categories

- File Operations
  - `read_file`
  - `write_file`
  - `delete_file`
  - `list_files`
  - `pwd`

- GitHub
  - `get_issues`
  - `create_issue`
  - `update_issue`
  - `delete_issue`

### Planned Categories

- Slack
  - `send_message`
  - `get_messages`
  - `delete_message`
- Code
  - `lint`
  - `format`
  - `test`
