#!/usr/bin/env python3

import os
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path

RUNNER = Path(__file__).resolve().parent / "throwaway-pg"
SEPARATOR = "\x1f"
DEFAULT_IMAGE = "docker.io/library/postgres:16-alpine"
PUBLISHED = "54321"

FAKE_PODMAN = """#!/bin/sh
for argument do printf '%s\\037' "$argument"; done >> "$FAKE_PODMAN_LOG"
printf '\\n' >> "$FAKE_PODMAN_LOG"
shift
case $1 in
image) exit "${FAKE_PODMAN_IMAGE_STATUS:-0}" ;;
run) echo 0123456789abcdef ;;
port) echo 127.0.0.1:""" + PUBLISHED + """ ;;
esac
exit 0
"""

FAKE_GO = """#!/bin/sh
[ "$1" = env ] || exit 1
shift
for variable do echo "/fake/go/$variable"; done
"""

EXPORTED = {
    "SENTRII_MEMORY_DATABASE_URL": "postgres://nobody@192.0.2.1:5432/sentrii_memory?sslmode=disable",
    "PGHOST": "192.0.2.1",
    "PGPASSWORD": "not-for-the-command",
    "SENTRII_CERBERUS_LIVE": "1",
    "GOOGLE_APPLICATION_CREDENTIALS": "/nonexistent/credentials.json",
}

PASSED = {"LANG": "C.UTF-8", "LC_ALL": "C", "TZ": "UTC", "TERM": "dumb"}


class Fixture(unittest.TestCase):
    def setUp(self):
        self.scratch = Path(tempfile.mkdtemp(prefix="throwaway-pg-test-")).resolve()
        self.addCleanup(self.remove_scratch)
        self.fakebin = self.scratch / "fakebin"
        self.fakebin.mkdir()
        for name, body in (("podman", FAKE_PODMAN), ("go", FAKE_GO)):
            path = self.fakebin / name
            path.write_text(body, encoding="utf-8")
            path.chmod(0o755)
        self.log = self.scratch / "podman.log"
        self.path = f"{self.fakebin}{os.pathsep}{os.environ.get('PATH', os.defpath)}"
        if shutil.which("podman", path=self.path) != str(self.fakebin / "podman"):
            raise RuntimeError("the fake podman does not come first on PATH; refusing to run the runner")
        self.rundir = self.scratch / "run"
        self.rundir.mkdir()

    def remove_scratch(self):
        for path in self.scratch.rglob("*"):
            if not path.is_symlink():
                path.chmod(0o700 if path.is_dir() else 0o600)
        shutil.rmtree(self.scratch)

    def environment(self, **extra):
        environment = dict(os.environ)
        environment.update(EXPORTED)
        environment.update(PASSED)
        environment["PATH"] = self.path
        environment["FAKE_PODMAN_LOG"] = str(self.log)
        environment.update(extra)
        return environment

    def run_runner(self, *command, arguments=("--env", "SENTRII_MEMORY_DATABASE_URL"), cwd=None, **extra):
        return subprocess.run(
            [str(RUNNER), *arguments, "--", *command],
            cwd=cwd or self.rundir,
            env=self.environment(**extra),
            capture_output=True,
            text=True,
            timeout=60,
        )

    def podman_calls(self):
        if not self.log.exists():
            return []
        return [line.split(SEPARATOR)[:-1] for line in self.log.read_text(encoding="utf-8").splitlines()]

    def subcommand(self, name):
        return [call for call in self.podman_calls() if len(call) > 1 and call[1] == name]

    def container_name(self):
        runs = self.subcommand("run")
        self.assertEqual(len(runs), 1, self.podman_calls())
        arguments = runs[0]
        return arguments[arguments.index("--name") + 1]


class EnvironmentTest(Fixture):
    def test_command_sees_only_the_allowlist_and_the_runners_own_dsn(self):
        result = self.run_runner("env")
        self.assertEqual(result.returncode, 0, result.stderr)
        seen = dict(line.split("=", 1) for line in result.stdout.splitlines())

        dsn = seen.get("SENTRII_MEMORY_DATABASE_URL", "")
        self.assertRegex(dsn, r"^postgres://postgres:[0-9a-f]+@127\.0\.0\.1:" + PUBLISHED + r"/postgres\?sslmode=disable$")
        self.assertNotEqual(dsn, EXPORTED["SENTRII_MEMORY_DATABASE_URL"])
        for name in EXPORTED:
            if name != "SENTRII_MEMORY_DATABASE_URL":
                self.assertNotIn(name, seen)

        expected = {
            "PATH": self.path,
            **PASSED,
            "GOCACHE": "/fake/go/GOCACHE",
            "GOMODCACHE": "/fake/go/GOMODCACHE",
            "GOPATH": "/fake/go/GOPATH",
            "GOPROXY": "off",
            "GOFLAGS": "-mod=readonly",
        }
        self.assertEqual(set(seen), set(expected) | {"SENTRII_MEMORY_DATABASE_URL", "HOME", "TMPDIR"})
        for name, value in expected.items():
            self.assertEqual(seen[name], value, name)
        self.assertNotEqual(seen["HOME"], os.environ.get("HOME"))
        self.assertNotEqual(seen["HOME"], seen["TMPDIR"])

    def test_home_and_tmpdir_are_empty_directories_when_the_command_starts(self):
        result = self.run_runner(
            "sh", "-c", 'echo "$HOME"; ls -A "$HOME"; echo --; echo "$TMPDIR"; ls -A "$TMPDIR"'
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        home, rest = result.stdout.split("--\n")
        home_lines, tmp_lines = home.splitlines(), rest.splitlines()
        self.assertEqual(len(home_lines), 1, home_lines)
        self.assertEqual(len(tmp_lines), 1, tmp_lines)
        self.assertNotEqual(home_lines[0], os.environ.get("HOME"))
        self.assertFalse(Path(home_lines[0]).exists())
        self.assertFalse(Path(tmp_lines[0]).exists())


class EnvFileRefusalTest(Fixture):
    def setUp(self):
        super().setUp()
        self.repo = self.scratch / "repo"
        self.rundir = self.repo / "a" / "b"
        self.rundir.mkdir(parents=True)
        subprocess.run(["git", "init", "-q", str(self.repo)], check=True, capture_output=True)

    def assert_refused(self, path):
        result = self.run_runner("true")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(str(path), result.stderr)
        self.assertEqual(self.podman_calls(), [], "podman was invoked before the refusal")

    def assert_ran(self):
        result = self.run_runner("true")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(self.subcommand("run")), 1)

    def test_a_readable_env_file_two_directories_up_is_refused(self):
        path = self.repo / ".env"
        path.write_text("DATABASE_URL=postgres://production\n", encoding="utf-8")
        self.assert_refused(path)

    def test_every_env_file_name_in_the_run_directorys_tree_is_refused(self):
        nested = self.rundir / "deeper"
        nested.mkdir()
        for name in (".env", ".env.local", "production.env", ".envrc"):
            with self.subTest(name=name):
                path = nested / name
                path.write_text("SECRET=1\n", encoding="utf-8")
                self.assert_refused(path)
                path.unlink()

    def test_an_env_file_is_refused_without_being_opened(self):
        path = self.rundir / ".env"
        os.mkfifo(path)
        self.assert_refused(path)

    def test_a_symlink_to_a_file_it_cannot_open_is_not_refused(self):
        if os.geteuid() == 0:
            self.skipTest("root can read a file with mode 000")
        secret = self.scratch / "secret"
        secret.write_text("SECRET=1\n", encoding="utf-8")
        secret.chmod(0)
        (self.repo / ".env").symlink_to(secret)
        self.assert_ran()

    def test_a_tracked_template_is_not_refused(self):
        template = self.rundir / ".env.example"
        template.write_text("DATABASE_URL=\n", encoding="utf-8")
        subprocess.run(["git", "-C", str(self.repo), "add", "a/b/.env.example"], check=True, capture_output=True)
        self.assert_ran()


class PodmanArgumentsTest(Fixture):
    def test_run_never_pulls_and_publishes_only_loopback(self):
        result = self.run_runner("true")
        self.assertEqual(result.returncode, 0, result.stderr)
        for call in self.podman_calls():
            self.assertEqual(call[0], "--cgroup-manager=cgroupfs", call)
        arguments = self.subcommand("run")[0]
        self.assertIn("--pull=never", arguments)
        self.assertFalse(any(re.match(r"--pull(=|$)", a) and a != "--pull=never" for a in arguments), arguments)
        self.assertNotIn("-P", arguments)
        self.assertNotIn("--publish-all", arguments)
        published = [arguments[i + 1] for i, a in enumerate(arguments) if a in ("-p", "--publish")]
        published += [a.split("=", 1)[1] for a in arguments if a.startswith("--publish=")]
        self.assertEqual(published, ["127.0.0.1::5432"])
        self.assertEqual(arguments[-1], DEFAULT_IMAGE)

    def test_each_run_gets_its_own_throwaway_pg_name(self):
        names = []
        for _ in range(2):
            self.log.unlink(missing_ok=True)
            result = self.run_runner("true")
            self.assertEqual(result.returncode, 0, result.stderr)
            names.append(self.container_name())
        for name in names:
            self.assertTrue(name.startswith("throwaway-pg-"), name)
        self.assertNotEqual(names[0], names[1])

    def test_image_override_is_used(self):
        image = "localhost/postgres:custom"
        result = self.run_runner("true", arguments=("--env", "DSN", "--image", image))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.subcommand("run")[0][-1], image)

    def test_a_missing_image_is_named_with_its_pull_command_and_nothing_starts(self):
        result = self.run_runner("true", FAKE_PODMAN_IMAGE_STATUS="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(DEFAULT_IMAGE, result.stderr)
        self.assertIn(f"podman pull {DEFAULT_IMAGE}", result.stderr)
        self.assertEqual(self.subcommand("run"), [])


class RemovalTest(Fixture):
    def assert_removed(self):
        name = self.container_name()
        self.assertIn(["--cgroup-manager=cgroupfs", "rm", "-f", name], self.podman_calls())

    def test_removed_when_the_command_exits_zero(self):
        result = self.run_runner("true")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_removed()

    def test_removed_when_the_command_exits_non_zero_and_its_status_is_kept(self):
        result = self.run_runner("sh", "-c", "exit 7")
        self.assertEqual(result.returncode, 7, result.stderr)
        self.assert_removed()

    def interrupt(self, sent, expected):
        marker = self.scratch / "started"
        process = subprocess.Popen(
            [str(RUNNER), "--env", "DSN", "--", "sh", "-c", 'touch "$0"; exec sleep 30', str(marker)],
            cwd=self.rundir,
            env=self.environment(),
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        try:
            deadline = time.monotonic() + 20
            while not marker.exists():
                if process.poll() is not None or time.monotonic() > deadline:
                    self.fail("the command never started under the runner")
                time.sleep(0.05)
            os.kill(process.pid, sent)
            self.assertEqual(process.wait(timeout=20), expected)
        finally:
            if process.poll() is None:
                process.kill()
                process.wait()
        self.assert_removed()

    def test_removed_on_sigint_with_the_conventional_status(self):
        self.interrupt(signal.SIGINT, 130)

    def test_removed_on_sigterm_with_the_conventional_status(self):
        self.interrupt(signal.SIGTERM, 143)


if __name__ == "__main__":
    unittest.main(argv=[sys.argv[0], *sys.argv[1:]], verbosity=1)
