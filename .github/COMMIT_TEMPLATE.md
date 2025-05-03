# <type>(<scope>): <short summary>
#   |       |             |
#   |       |             └─⫸ Summary in present tense. Not capitalized. No period at the end.
#   |       |
#   |       └─⫸ Commit Scope: Optional, can be anything specifying the scope of the commit change
#   |                          Examples: backend, ynab, ui, auth 
#   |
#   └─⫸ Commit Type: feat|fix|docs|style|refactor|perf|test|build|ci|chore
#                       |   |    |     |        |    |    |    |  |
#                       |   |    |     |        |    |    |    |  └─⫸ Other changes (e.g., package.json updates)
#                       |   |    |     |        |    |    |    |
#                       |   |    |     |        |    |    |    └─⫸ CI/CD configuration changes
#                       |   |    |     |        |    |    |  
#                       |   |    |     |        |    |    └─⫸ Build system changes
#                       |   |    |     |        |    |
#                       |   |    |     |        |    └─⫸ Tests
#                       |   |    |     |        |
#                       |   |    |     |        └─⫸ Performance improvements
#                       |   |    |     |
#                       |   |    |     └─⫸ Code style/formatting changes
#                       |   |    |
#                       |   |    └─⫸ Documentation changes
#                       |   |
#                       |   └─⫸ Bug fixes
#                       |
#                       └─⫸ Features
#
# Examples:
#   feat(ynab): add ability to sync with multiple accounts
#   fix(auth): prevent login timeout on inactive tabs
#   docs(readme): update installation instructions
#   style: standardize whitespace
#   refactor(db): improve query performance
#   perf(api): optimize response time
#   test(ynab): add tests for sync functionality
#   chore(deps): update dependencies
#
# Add BREAKING CHANGE in the commit body or footer to trigger a major release:
# Example:
# feat(api): change authentication flow
# 
# BREAKING CHANGE: Users will need to reauthenticate after this change 