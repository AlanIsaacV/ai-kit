# Every path written by `install` is derived from $(CURDIR), so the install
# follows the clone it was run from and there is nothing to configure.
REPO := $(CURDIR)
BIN := $(REPO)/bin/memory
BIN_DIR := $(HOME)/.local/bin
BIN_LINK := $(BIN_DIR)/memory
SKILLS_DIR := $(HOME)/.claude/skills
SKILL_LINK := $(SKILLS_DIR)/ai-kit
CODEX_SKILLS_DIR := $(HOME)/.agents/skills

.PHONY: build install check

build:
	go build -C tools/memory -o "$(BIN)" ./cmd/memory

# Nothing here depends on build: the check reads text files and installs
# nothing, so it runs on a clone with no Go toolchain on it.
check:
	"$(REPO)/scripts/check-frontmatter"

install: build
	@mkdir -p "$(BIN_DIR)" "$(SKILLS_DIR)" "$(CODEX_SKILLS_DIR)"
# -n keeps ln from following an existing link to the clone and creating the new
# one inside it; -f replaces it instead. Together they make a second run a no-op.
# Neither helps when the path is a real directory — not a symlink to one, which
# -n already covers: there ln would put the new link *inside* it and still exit
# 0, reporting an install the hosts cannot see. So that case is refused before
# ln runs, leaving nothing inside the offending path, and every placement that
# does run is read back rather than assumed.
#
# A refusal does not stop the run. Everything that can be linked is, and the
# failures are reported together at the end, so one run names every path that
# has to be removed instead of only the first, and re-running after removing
# them places exactly what was skipped.
#
# Claude Code loads the clone as one plugin; Codex registers one skill at a
# time, so it gets a link per directory under skills/. The set is read off the
# tree rather than listed here, so a skill added later installs itself — the
# link, at least: it also needs its own agents/openai.yaml committed, or Codex
# lists it with no display name and no blurb.
	@failed=; \
	link() { \
	  if [ -d "$$1" ] && [ ! -L "$$1" ]; then \
	    echo "error: $$1 is a directory, not a symlink to this clone; nothing was linked there." >&2; \
	    failed=1; \
	    return; \
	  fi; \
	  ln -sfn "$$2" "$$1"; \
	  if [ "$$(readlink "$$1")" = "$$2" ]; then \
	    echo "$$1 -> $$2"; \
	  else \
	    echo "error: $$1 could not be replaced with a symlink to this clone." >&2; \
	    failed=1; \
	  fi; \
	}; \
	link "$(BIN_LINK)" "$(BIN)"; \
	link "$(SKILL_LINK)" "$(REPO)"; \
	for skill in "$(REPO)"/skills/*/; do \
	  [ -d "$$skill" ] || continue; \
	  name=$$(basename "$$skill"); \
	  link "$(CODEX_SKILLS_DIR)/ai-kit-$$name" "$(REPO)/skills/$$name"; \
	done; \
	[ -z "$$failed" ] || { \
	  echo "error: the install is incomplete. Remove the paths named above and run it again; everything else is already in place." >&2; \
	  exit 1; \
	}
	@case ":$$PATH:" in \
	  *":$(BIN_DIR):"*) ;; \
	  *) echo "warning: $(BIN_DIR) is not on PATH, so memory will not resolve by name until it is." ;; \
	esac
