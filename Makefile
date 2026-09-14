# Installation links follow the clone at $(CURDIR); destination directories
# can be overridden on the make command line.
REPO := $(CURDIR)
BIN := $(REPO)/bin/memory
BIN_DIR := $(HOME)/.local/bin
BIN_LINK := $(BIN_DIR)/memory
SKILLS_DIR := $(HOME)/.claude/skills
SKILL_LINK := $(SKILLS_DIR)/ai-kit
CODEX_SKILLS_DIR := $(HOME)/.agents/skills
CODEX_AGENTS_DIR := $(if $(CODEX_HOME),$(CODEX_HOME),$(HOME)/.codex)/agents
CODEX_AGENT_BUILD_DIR := $(REPO)/bin/codex-agents

.PHONY: build install check doctor check-install

build:
	go build -C tools/memory -o "$(BIN)" ./cmd/memory

# Nothing here depends on build: the check reads text files and installs
# nothing, so it runs on a clone with no Go toolchain on it.
check:
	"$(REPO)/scripts/check-frontmatter"

doctor check-install:
	@"$(REPO)/scripts/check-install" --repo "$(REPO)" --bin "$(BIN)" --bin-link "$(BIN_LINK)" --claude-link "$(SKILL_LINK)" --skills-dir "$(CODEX_SKILLS_DIR)" --agents-dir "$(CODEX_AGENTS_DIR)" --build-dir "$(CODEX_AGENT_BUILD_DIR)"

install: build
	@mkdir -p "$(BIN_DIR)" "$(SKILLS_DIR)" "$(CODEX_SKILLS_DIR)"
# -n keeps ln from following an existing link to the clone and creating the new
# one inside it; -f replaces it instead. Together they make a second run a no-op.
# Neither protects an existing real file or directory: ln can replace a file
# or put the new link inside a directory and still report success. Both cases
# are refused before ln runs, and every placement is read back.
#
# A refusal does not stop the run. Everything that can be linked is, and the
# failures are reported together at the end, so one run names every path that
# has to be removed instead of only the first, and re-running after removing
# them places exactly what was skipped.
#
# Claude Code loads the clone as one plugin; Codex registers one skill at a
# time, so it gets a link per directory under skills/. The set is read off the
# tree rather than listed here, so a directory containing SKILL.md installs
# itself. Codex display metadata in agents/openai.yaml is optional.
	@failed=; \
	link() { \
	  if [ -e "$$1" ] && [ ! -L "$$1" ]; then \
	    echo "error: $$1 is a real file or directory; nothing was linked there." >&2; \
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
	  [ -f "$$skill/SKILL.md" ] || continue; \
	  name=$$(basename "$$skill"); \
	  link "$(CODEX_SKILLS_DIR)/ai-kit-$$name" "$(REPO)/skills/$$name"; \
	done; \
	"$(REPO)/scripts/install-codex-agents" --repo "$(REPO)" --agents-dir "$(CODEX_AGENTS_DIR)" --build-dir "$(CODEX_AGENT_BUILD_DIR)" || failed=1; \
	[ -z "$$failed" ] || { \
	  echo "error: the install is incomplete. Remove the paths named above and run it again; everything else is already in place." >&2; \
	  exit 1; \
	}
	@case ":$$PATH:" in \
	  *":$(BIN_DIR):"*) ;; \
	  *) echo "warning: $(BIN_DIR) is not on PATH, so memory will not resolve by name until it is." ;; \
	esac
