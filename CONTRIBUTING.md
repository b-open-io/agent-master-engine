# Contributing to Agent Master Engine

We welcome contributions to the Agent Master Engine! This document provides guidelines for contributing to the project.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/your-username/agent-master-engine.git`
3. Create a feature branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Test your changes: `go test ./...`
6. Commit your changes: `git commit -m "Add your feature"`
7. Push to your fork: `git push origin feature/your-feature-name`
8. Create a Pull Request

## Development Guidelines

### Code Style

- Follow standard Go conventions and formatting (`gofmt`)
- Use meaningful variable and function names
- Keep functions focused and concise
- Add comments for exported functions and complex logic

### Architecture Principles

- **Keep the core generic** - No platform-specific code in the main engine
- **Use interfaces** - Prefer interfaces over concrete types for extensibility
- **Separation of concerns** - Platform-specific logic goes in the `presets` package
- **Test with real data** - Use the test vectors in `testdata/` for realistic testing

### Testing

- Write unit tests for new functionality
- Use the existing test vectors in `testdata/` when possible
- Test both generic and preset-based usage patterns
- Run tests with race detection: `go test -race ./...`

### Documentation

- Update relevant documentation in `docs/` for user-facing changes
- Add examples for new features
- Keep the README.md up to date
- Internal development notes go in `internal-docs/` (not committed to git)

## Types of Contributions

### Bug Fixes
- Include a clear description of the bug
- Add a test case that reproduces the issue
- Ensure the fix doesn't break existing functionality

### New Features
- Discuss major features in an issue first
- Follow the generic design principles
- Add comprehensive tests and documentation
- Consider adding preset configurations for common use cases

### New Destinations
- Implement the `Destination` interface
- Add comprehensive error handling
- Include usage examples
- Consider thread safety

### New Presets
- Add to the `presets` package
- Include validation rules and sanitization
- Provide clear documentation
- Test with real-world configurations

## Pull Request Guidelines

- Provide a clear description of the changes
- Reference any related issues
- Include tests for new functionality
- Update documentation as needed
- Ensure all tests pass
- Keep commits focused and atomic

## Code Review Process

1. All submissions require review
2. Maintainers will review for:
   - Code quality and style
   - Test coverage
   - Documentation completeness
   - Adherence to architecture principles
3. Address feedback promptly
4. Squash commits before merging if requested

## Questions?

- Open an issue for bugs or feature requests
- Start a discussion for architectural questions
- Check existing issues and documentation first

Thank you for contributing to Agent Master Engine! 