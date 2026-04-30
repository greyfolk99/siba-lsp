package mcp

// helpText returns SIBA documentation for the given topic.
func helpText(topic string) string {
	switch topic {
	case "directives":
		return `# SIBA Directives

All directives are HTML comments: <!-- @keyword args -->

| Directive | Description |
|-----------|-------------|
| @doc name | Document name (identifier) |
| @template name | Template name (identifier, required) |
| @extends name | Inherit from a template (single inheritance) |
| @name id | Explicit heading identifier (overrides slug) |
| @default | Heading annotation: has default content |
| @const name = value | Immutable variable (no shadowing) |
| @let name = value | Mutable variable (shadowing allowed) |
| @if condition | Conditional block start |
| @endif | Conditional block end |
| @for item in collection | Loop block start |
| @endfor | Loop block end |

@template and @doc are mutually exclusive.
@template requires a name: <!-- @template api-spec -->`

	case "variables":
		return `# SIBA Variables

## Declaration
<!-- @const service-name = "payment-api" -->
<!-- @let version = "1.0" -->

| Keyword | Reassign | Shadowing |
|---------|----------|-----------|
| @const | No | Forbidden |
| @let | Yes | Allowed in child scope |

## Types (TS subset)
string, number, boolean, T[], { key: T }, T | U, null

## Access Control
(default) = public — accessible everywhere
private — current document only
protected — current + extending documents

## Scope
Heading hierarchy defines scope chain. Inner → outer lookup.`

	case "templates":
		return `# SIBA Templates

## Define a template
<!-- @template api-spec -->
# API Spec
## Endpoints       (required - no annotation)
<!-- @default -->
## Changelog       (has default content)

## Extend a template
<!-- @doc payment-api -->
<!-- @extends api-spec -->
# Payment API
## Endpoints
...

## Rules
- @template requires a name
- @template + @doc are mutually exclusive
- No annotation = required (must be in extending doc)
- @default = has default content (inherited if not overridden)
- Extending docs can add extra headings freely
- Single inheritance only (@extends one template)`

	case "references":
		return `# SIBA References

## Import (required for external access)
<!-- @import alias from ./path.md -->

## Local references
| Pattern | Meaning |
|---------|---------|
| {{variable}} | Local variable (scope chain) |
| {{obj.prop}} | Object property |
| {{#section}} | Current file symbol (heading/doc/template) |
| {{#parent/child}} | Nested symbol |

## External references (@import required)
| Pattern | Meaning |
|---------|---------|
| {{alias.variable}} | Module-level variable from imported file |
| {{alias#symbol}} | Symbol from imported file |
| {{alias#parent/child}} | Nested symbol from imported file |

## Escape
\{{literal}} → outputs {{literal}}

Note: direct {{doc-name.variable}} without @import is not supported.`

	case "control":
		return `# SIBA Control Flow

## Conditional
<!-- @if env == "production" -->
## Production Config
...
<!-- @endif -->

Operators: ==, !=, >, <, >=, <=
Single variable = truthy check.

## Loop
<!-- @for endpoint in endpoints -->
### {{endpoint.name}}
{{endpoint.description}}
<!-- @endfor -->

## Scope
@if/@for blocks create their own scope.
Variables declared inside are not visible outside.
Parent scope variables are readable inside.

## Directives use bare variable names (no {{}}):
<!-- @if env == "production" -->  (not {{env}})`

	case "packages":
		return `# SIBA Packages

Go-module style. Git URLs as package names.

## module.toml
[module]
name = "github.com/user/my-docs"
version = "1.0.0"

[dependencies]
"github.com/user/templates" = "v1.2.0"

[scripts]
prerender = "echo starting"
postrender = "deploy.sh"

## CLI
siba init              Initialize project
siba get <pkg> [ver]   Add dependency
siba tidy              Remove unused deps
siba run <script>      Run script

## Cache
~/.siba/cache/{url}@{version}/`

	case "types":
		return `# SIBA Type System

TS subset. Inferred from values, or explicit.

## Supported types
| Type | Example |
|------|---------|
| string | "hello" |
| number | 8080, 3.14 |
| boolean | true, false |
| T[] | ["a", "b"] |
| { key: T } | { name: "x", port: 80 } |
| T | U | string | number (union) |
| null | null |

## Explicit type declaration
<!-- @const port: number = 8080 -->
<!-- @const tags: string[] = ["auth"] -->

## Type-only declaration (template contract)
<!-- @const service-name: string -->

Type mismatches produce diagnostics at check/render time.`

	default:
		return `# SIBA — Structured Ink for Building Archives

Markdown module system. Headings are structure, HTML comments are annotations.

## Quick Reference

<!-- @template api-spec -->     Define a template
<!-- @doc payment-api -->       Name a document
<!-- @extends api-spec -->      Inherit from template
<!-- @const name = "value" -->  Declare constant
<!-- @let name = "value" -->    Declare mutable variable
{{variable}}                    Reference a variable
{{doc-name.variable}}           Cross-document reference
<!-- @if cond --> ... @endif    Conditional
<!-- @for x in xs --> ... @endfor  Loop

## CLI
siba init          Initialize project
siba check file.md Check for errors
siba render file.md Render to stdout
siba render        Render workspace → _render/{version}/
siba check --json  JSON output (for tooling)

## Help Topics
Use siba_help with topic: directives, variables, templates, references, control, packages, types`
	}
}
