# language-detection

## Purpose

Automatically detects the primary language(s) of a target repository, using marker files first and file-extension counting as a fallback, so the build pipeline can route to the correct extractor(s) without user configuration.

## Requirements

### Requirement: Marker-file detection SHALL follow a fixed priority order
The `LanguageDetector` SHALL check for language marker files in the repository root in this priority order: `go.mod` (Go), `pom.xml` (Java/Maven), `build.gradle` or `build.gradle.kts` (Java/Gradle), `tsconfig.json` (TypeScript), `package.json` without `tsconfig.json` (JavaScript), then `pyproject.toml`, `setup.py`, or `requirements.txt` (Python). Every marker present SHALL contribute its language to the result; when more than one marker matches, the repository SHALL be treated as multi-language.

#### Scenario: Go detected via go.mod
- **WHEN** a repository root contains `go.mod`
- **THEN** `Detect` includes Go among the detected languages

#### Scenario: Java detected via pom.xml
- **WHEN** a repository root contains `pom.xml`
- **THEN** `Detect` includes Java among the detected languages

#### Scenario: Java detected via Gradle build file
- **WHEN** a repository root contains `build.gradle` or `build.gradle.kts`
- **THEN** `Detect` includes Java among the detected languages

#### Scenario: TypeScript detected via tsconfig.json
- **WHEN** a repository root contains `tsconfig.json`
- **THEN** `Detect` includes TypeScript among the detected languages

#### Scenario: JavaScript detected via package.json without tsconfig.json
- **WHEN** a repository root contains `package.json` but no `tsconfig.json`
- **THEN** `Detect` includes JavaScript among the detected languages

#### Scenario: TypeScript takes priority over JavaScript
- **WHEN** a repository root contains both `package.json` and `tsconfig.json`
- **THEN** `Detect` includes TypeScript, not JavaScript, for that marker pair

#### Scenario: Python detected via marker files
- **WHEN** a repository root contains `pyproject.toml`, `setup.py`, or `requirements.txt`
- **THEN** `Detect` includes Python among the detected languages

#### Scenario: Multiple markers yield multi-language result
- **WHEN** a repository root contains both `go.mod` and `package.json`
- **THEN** `Detect` returns both Go and JavaScript in `DetectionResult.All`

#### Scenario: Marker files in subdirectories are ignored
- **WHEN** a marker file (e.g. `pom.xml`) exists only in a subdirectory and not the repository root
- **THEN** `Detect` does not consider that language detected on the basis of that file

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

### Requirement: DetectionResult SHALL expose primary language, all languages, and markers
`Detect` SHALL return a `DetectionResult` containing a `Primary` language, an `All` slice of every detected language, and a `Markers` map from language to the marker file(s) that triggered its detection.

#### Scenario: Markers map reflects triggering files
- **WHEN** a repository is detected as Java via `pom.xml` and TypeScript via `tsconfig.json`
- **THEN** `DetectionResult.Markers["java"]` includes `"pom.xml"` and `DetectionResult.Markers["typescript"]` includes `"tsconfig.json"`

### Requirement: Empty repository SHALL produce an error
`Detect` SHALL return an error when the repository contains no files at all, and SHALL NOT detect a programming language solely from `.sql` migration files.

#### Scenario: Empty repository
- **WHEN** `Detect` is called against a repository with no files
- **THEN** it returns an error rather than a zero-value `DetectionResult`

#### Scenario: Only SQL files present
- **WHEN** a repository contains only `.sql` migration files and no marker files or recognized source extensions
- **THEN** `Detect` does not report a programming language as detected on the basis of those files
