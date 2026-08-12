# Every path written by `install` is derived from $(CURDIR), so the install
# follows the clone it was run from and there is nothing to configure.
REPO := $(CURDIR)
BIN := $(REPO)/bin/memory
BIN_DIR := $(HOME)/.local/bin
BIN_LINK := $(BIN_DIR)/memory
SKILLS_DIR := $(HOME)/.claude/skills
SKILL_LINK := $(SKILLS_DIR)/ai-kit
CODEX_SKILLS_DIR := $(HOME)/.agents/skills

.PHONY: build install

build:
	go build -C tools/memory -o "$(BIN)" ./cmd/memory

install: build
	@mkdir -p "$(BIN_DIR)" "$(SKILLS_DIR)" "$(CODEX_SKILLS_DIR)"
# -n keeps ln from following an existing link to the clone and creating the new
# one inside it; -f replaces it instead. Together they make a second run a no-op.
	@ln -sfn "$(BIN)" "$(BIN_LINK)"
	@ln -sfn "$(REPO)" "$(SKILL_LINK)"
# -n only helps when the path is already a symlink. Against a real directory ln
# puts the new link *inside* it and still exits 0, which would report an install
# the hosts cannot see, so both placements are read back instead of assumed.
	@[ "$$(readlink "$(BIN_LINK)")" = "$(BIN)" ] || { \
	  echo "error: $(BIN_LINK) exists and is not a symlink to this clone; ln left its link inside it. Remove that path, then re-run." >&2; \
	  exit 1; \
	}
	@[ "$$(readlink "$(SKILL_LINK)")" = "$(REPO)" ] || { \
	  echo "error: $(SKILL_LINK) exists and is not a symlink to this clone; ln left its link inside it. Remove that path, then re-run." >&2; \
	  exit 1; \
	}
	@echo "$(BIN_LINK) -> $(BIN)"
	@echo "$(SKILL_LINK) -> $(REPO)"
# Claude Code loads the clone as one plugin; Codex registers one skill at a time,
# so it gets a link per directory under skills/. The set is read off the tree
# rather than listed here, so a skill added later installs itself. Each link is
# read back for the same reason the two above are.
	@for skill in "$(REPO)"/skills/*/; do \
	  [ -d "$$skill" ] || continue; \
	  name=$$(basename "$$skill"); \
	  link="$(CODEX_SKILLS_DIR)/ai-kit-$$name"; \
	  ln -sfn "$(REPO)/skills/$$name" "$$link"; \
	  [ "$$(readlink "$$link")" = "$(REPO)/skills/$$name" ] || { \
	    echo "error: $$link exists and is not a symlink to this clone; ln left its link inside it. Remove that path, then re-run." >&2; \
	    exit 1; \
	  }; \
	  echo "$$link -> $(REPO)/skills/$$name"; \
	done
	@case ":$$PATH:" in \
	  *":$(BIN_DIR):"*) ;; \
	  *) echo "warning: $(BIN_DIR) is not on PATH, so memory will not resolve by name until it is." ;; \
	esac
