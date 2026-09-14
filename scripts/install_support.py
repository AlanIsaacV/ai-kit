import json


def agent_definitions(repo, build_dir):
    for source in sorted((repo / "agents").glob("*.md")):
        lines = source.read_text(encoding="utf-8").splitlines()
        if not lines or lines[0] != "---" or "---" not in lines[1:]:
            raise ValueError(f"{source}: missing agent frontmatter")
        fields = dict(
            line.split(": ", 1)
            for line in lines[1:lines.index("---", 1)]
            if ": " in line
        )
        description = fields["description"]
        if description.startswith('"'):
            description = json.loads(description)
        elif description.startswith("'") and description.endswith("'"):
            description = description[1:-1].replace("''", "'")
        name = f"ai-kit-{source.stem}"
        instructions = (
            f"Read {json.dumps(str(repo / 'runtime.md'), ensure_ascii=False)} first for host-specific runtime instructions. "
            f"Then read {json.dumps(str(source), ensure_ascii=False)} and follow its agent contract. "
            "The Markdown body is authoritative. Ignore the Claude-only tools and model frontmatter; "
            "use the tools available in this Codex session and inherit its model. "
            "If either file cannot be read, report the missing path and stop."
        )
        values = {"name": name, "description": description, "developer_instructions": instructions}
        if source.stem in {"evaluate", "explore", "review"}:
            values["sandbox_mode"] = "read-only"
        content = "".join(f"{key} = {json.dumps(value, ensure_ascii=False)}\n" for key, value in values.items())
        yield name, build_dir / f"{name}.toml", content


def link_problem(link, target):
    if not link.is_symlink():
        return f"{link}: missing symlink or occupied by a real file/directory"
    try:
        if link.resolve(strict=True) != target.resolve(strict=True):
            return f"{link}: points to the wrong target (expected {target})"
    except (OSError, RuntimeError) as error:
        return f"{link}: broken link or missing target ({error})"
    return None
