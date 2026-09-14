## ADDED Requirements

### Requirement: Rationale comment text SHALL span continuation lines
A rationale-tagged comment (NOTE:, WHY:, HACK:, TODO:, FIXME:, WARNING:) whose text continues on immediately-following comment lines using the same line-comment prefix SHALL have its full, joined text captured in the Rationale node, not just the text on the tagged line.

#### Scenario: NOTE comment continues on the next line
- **WHEN** a Go file contains
  ```go
  // NOTE: Uses stateless JWT for horizontal scaling,
  // avoiding sticky sessions across replicas.
  ```
- **THEN** a Rationale node SHALL be created with kind="NOTE" and text="Uses stateless JWT for horizontal scaling, avoiding sticky sessions across replicas."

#### Scenario: Continuation stops at a blank or differently-prefixed line
- **WHEN** a rationale-tagged comment line is followed by a blank line, a non-comment line, or a new comment that doesn't continue it (e.g. a fresh tag or unrelated text)
- **THEN** only the lines up to that boundary SHALL be included in the Rationale node's text
