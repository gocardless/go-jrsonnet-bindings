# Contributing

## Issues

Search the existing issues before you open a new one.

For a bug, include your Go version, OS, target triple, and a Jsonnet snippet that reproduces the problem. If the result differs from go-jsonnet, show the output of both.

For a new feature, open an issue before you write any code, so that we can agree on the API first.

Don't report security vulnerabilities in a public issue. See [Security](#security).

## Development

[Prerequisites](README.md#prerequisites) lists the tools you need. To run all tests:

```bash
make test
```

This builds the Rust static library for your host, then runs the Rust tests, the Go tests, and the comparison with go-jsonnet in `conformance/`.

If you change the Rust FFI API, run `make header` and commit the updated `include/jrsonnet_go.h`. CI fails if the header is out of date.

## Pull requests

- Keep each pull request to one change.
- A bug fix needs a test that fails without the fix and passes with it.
- If your change affects output that go-jsonnet also produces, add a case to `conformance/`. A new known difference also goes in the README under "Differences from go-jsonnet".
- Update `README.md` when you change the public API.
- Use [Conventional Commits](https://www.conventionalcommits.org/) for the pull request title and commit messages, for example `fix: reject invalid UTF-8 in ext vars`.

A maintainer must approve CI runs on pull requests from people outside GoCardless, so checks may take a while to start.

## AI-assisted contributions

You may use AI tools to write code for this project. You are responsible for every line you submit, however it was written.

- Say in the pull request description which AI tools you used and what for. Autocomplete doesn't need a mention.
- Review all of the code yourself before you open the pull request.
- Understand the change. Be ready to explain any line, why you chose this approach over the alternatives, and how you tested it.
- Answer maintainers' questions yourself, without asking an AI tool.
- Write issues, pull request descriptions, and replies to reviewers yourself. Tools that fix grammar or translate are fine.
- Don't open issues or pull requests from an agent that runs without a person reviewing its work.

Maintainers may close pull requests that don't follow these rules.

These rules are based on the [llama.cpp contributing guide](https://github.com/ggml-org/llama.cpp/blob/master/CONTRIBUTING.md).

## Security

Report vulnerabilities as described in the [GoCardless security policy](https://github.com/gocardless/.github/blob/master/SECURITY.md).

## Code of conduct

Everyone who takes part in this project must follow the [Code of Conduct](CODE_OF_CONDUCT.md).
