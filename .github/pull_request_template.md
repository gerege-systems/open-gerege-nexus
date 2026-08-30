## Description
Please include a summary of the change and which issue is fixed. Include relevant motivation and context.

Fixes # (issue)

## Type of Change
- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] New App Module (adds a new business module)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update

## Checklist
- [ ] My code follows the style guidelines of this project.
- [ ] I have performed a self-review of my own code.
- [ ] I have commented my code, particularly in hard-to-understand areas.
- [ ] I have added unit tests (`*_test.go`) that prove my fix is effective or that my feature works.
- [ ] New and existing unit tests pass locally with my changes (`cd backend && go test ./...`).
- [ ] Next.js frontend builds cleanly (`cd frontend && npm run build`).

## The ecosystem's contract

Only if `backend/pkg/nexus/testdata/api.txt` is in this diff. Every distribution
repository compiles against that package and pins it by tag, so a change to it
is a change to code that is not in this repository and cannot be fixed from it —
see [`docs/MODULES.md`](../docs/MODULES.md) §1.

- [ ] The description above says, in words, **what a caller has to do about it**.
      Not what changed — the diff says that. What somebody with a working
      distribution has to change, or that they have to change nothing.
- [ ] Anything removed or renamed went through `// Deprecated:` and one major
      cycle first, and the description names the version it goes in.

The golden file's failure message already asks for this. The checkbox is what
brings the request as far as the pull request, where it is read.

## Authors & Credits
- Contributor: @username
- Developed with: **Gerege Systems Development Team & Gemini AI**
