# Contributing

Thanks for your interest in hrs.

## Getting started

```bash
git clone https://github.com/heuwels/hrs
cd hrs
go build -o hrs .
go test -race ./...
```

No CGo — a plain Go toolchain is all you need.

## Making changes

1. Fork the repo and create a branch from `master`
2. Make your changes
3. Run `go vet ./...` and `go test -race ./...`
4. Open a pull request

## What we're looking for

- Bug fixes with a test that reproduces the issue
- Performance improvements with benchmarks
- Documentation improvements

## What to avoid

- Large refactors without discussion first — open an issue
- New dependencies unless absolutely necessary
- Breaking changes to the CLI or API without a migration path

## Reporting bugs

Open an issue on GitHub with:
- What you expected
- What happened
- Steps to reproduce
- `hrs version` output and OS/arch

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
