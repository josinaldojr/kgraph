## MODIFIED Requirements

### Requirement: Extension-count fallback SHALL apply when no marker matches
When no marker file is found, the `LanguageDetector` SHALL count files by extension (`.go`, `.java`, `.ts`/`.tsx`, `.js`/`.jsx`, `.py`) across the repository and SHALL select the language with the most matching files as the primary (and only) detected language. When two or more languages tie for the highest count, the detector SHALL break the tie deterministically by selecting the language that comes first in the fixed priority order used for marker detection (Go, Java, TypeScript, JavaScript, Python), rather than depending on map iteration order.

#### Scenario: Fallback picks the majority extension
- **WHEN** no marker file is present but the repository contains more `.py` files than any other recognized extension
- **THEN** `Detect` returns Python as the sole detected language

#### Scenario: No recognized files at all
- **WHEN** neither a marker file nor any file with a recognized extension is found
- **THEN** `Detect` returns an error

#### Scenario: Tied extension counts resolve deterministically
- **WHEN** no marker file is present and the repository contains an equal number of `.go` and `.py` files, with no other recognized extension present
- **THEN** `Detect` returns Go as the sole detected language, and repeated calls against the same file tree always return the same result
