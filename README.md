# sca-cli

Lightweight repository scanner for detecting package managers and extracting basic dependency lists per manifest file.

## Features

- Clone a git repo (shallow) or analyze an existing checkout
- Detect common package managers (Go, Node, Python, Maven, Gradle, Composer, Ruby, Rust, Swift)
- Extract dependencies from go.mod, package.json, pom.xml, build.gradle (basic parsing)
- Produce pretty CLI output or JSON output; dependencies are grouped by ecosystem and by manifest path

## Prerequisites

- go 1.18+
- git available in PATH
- (optional) mvn installed if you want deeper Maven resolution (future)

## Build

- Initialize module (recommended):
  ```sh
  go mod init github.com/joshdotcc/sca-cli
  go build -o sca-cli .
  ```

## Run

- Clone and analyze a repository (default target dir `./repo`):
  ```sh
  ./sca-cli https://github.com/user/repo
  ```

- Clone into a specific directory:
  ```sh
  ./sca-cli https://github.com/user/repo -dir /tmp/myrepo
  ```

- Analyze an existing checkout (skip clone):
  ```sh
  ./sca-cli -dir /path/to/checkout -skip-clone
  ```

- Output JSON to stdout:
  ```sh
  ./sca-cli https://github.com/user/repo -output json
  ```

- Write JSON to a file (shorthand -o):
  ```sh
  ./sca-cli https://github.com/user/repo -o output.json
  ```

## Developer / troubleshooting

- If you see `go: cannot find main module`, either run the package with all files or create a module:
  ```sh
  go run . <repo> -o out.json
  ```
  or
  ```sh
  go mod init github.com/yourname/sca-cli
  ```

- To run without creating a module:
  ```sh
  go run *.go <repo> -o out.json
  ```

## Notes about Maven and versions

- The scanner extracts versions it can read from manifest files. Many Maven projects use property placeholders (e.g. `${h2.version}`) defined in parent POMs or BOMs that may not be present locally. The tool currently omits placeholder versions rather than attempting remote resolution.

- For accurate Maven effective versions, run Maven (if available) to evaluate properties or generate an effective POM. Adding an option to run `mvn help:effective-pom` per module is a planned improvement.

## Next steps / improvements

- Better Gradle/Kotlin DSL handling and reading of lockfiles for exact versions
- Lockfile parsing for npm/yarn/poetry/pip
- Option to call Maven to compute effective POMs and resolve property placeholders
- Move parsers into packages and add unit tests for parsing logic

## Contributing

- Open PRs or issues. Add unit tests under `testdata/` when adding parsers.

## License

- MIT