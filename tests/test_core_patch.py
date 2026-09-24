#!/usr/bin/env python3
"""Host-only checks: python3 tests/test_core_patch.py --core-repo /path/to/mlx."""

import argparse
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


PRISTINE = "1f8e74e3f12f31365464a6867c6579f0e9b29d85"
PATCHED = "b76656e61d0aed0cd9fb74ae7554ad08429de97e"
ROOT = Path(__file__).resolve().parents[1]
CORE_REPO = None


class CorePatchTest(unittest.TestCase):
    def setUp(self):
        scratch = Path.home() / "tmp"
        scratch.mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(prefix="mlx-core-patch-", dir=scratch)
        self.addCleanup(self.temp.cleanup)
        self.source = Path(self.temp.name) / "core"
        subprocess.run(
            ["git", "clone", "--shared", "--no-checkout", str(CORE_REPO), str(self.source)],
            check=True, capture_output=True,
        )
        self.git("checkout", "--detach", PRISTINE)
        self.recipe = Path(self.temp.name) / "cmake"
        shutil.copytree(ROOT / "cmake", self.recipe)

    def git(self, *args):
        return subprocess.check_output(
            ["git", "-C", str(self.source), *args], stderr=subprocess.PIPE
        )

    def apply(self):
        return subprocess.run(
            ["cmake", "-DMLX_SOURCE_DIR=" + str(self.source),
             "-P", str(self.recipe / "patch-mlx.cmake")],
            capture_output=True, text=True, timeout=30,
        )

    def snapshot(self):
        # Include untracked fixtures and modes, excluding clone metadata.
        return {
            str(p.relative_to(self.source)): (
                p.lstat().st_mode,
                os.readlink(p) if p.is_symlink() else hashlib.sha256(p.read_bytes()).hexdigest(),
            )
            for p in self.source.rglob("*")
            if ".git" not in p.relative_to(self.source).parts and (p.is_file() or p.is_symlink())
        }

    def reject(self, reason):
        before = self.snapshot()
        result = self.apply()
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn(reason, result.stderr)
        self.assertEqual(self.snapshot(), before, "rejection changed source files")

    def test_apply_exact_tree_and_idempotence(self):
        result = self.apply()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        before = self.snapshot()
        result = self.apply()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("already applied", result.stdout)
        self.assertEqual(self.snapshot(), before)
        # Tree equality covers every tracked byte, path, mode and symlink,
        # rather than only the eight files checked by the application recipe.
        self.git("add", "-u")
        self.assertEqual(self.git("write-tree"), self.git("rev-parse", PATCHED + "^{tree}"))

    def test_wrong_revision(self):
        self.git("checkout", "--detach", PATCHED)
        self.reject("requires exact MLX v0.32.2 source")

    def test_dirty_patch_file(self):
        with (self.source / "mlx/event.h").open("a") as f:
            f.write("\n// unreviewed edit\n")
        self.reject("differs from pristine or fully patched")

    def test_mixed_patch(self):
        (self.source / "mlx/event.h").write_bytes(self.git("show", PATCHED + ":mlx/event.h"))
        self.reject("differs from pristine or fully patched")

    def test_previous_patch_only(self):
        previous = self.git("show", "8af4c14ae1cd89c741e409defb101f51bcc53df9:mlx/event.h")
        (self.source / "mlx/event.h").write_bytes(previous)
        for name in ["mlx/backend/gpu/eval.h", "mlx/random.cpp", "tests/random_tests.cpp"]:
            (self.source / name).write_bytes(self.git("show", "8af4c14ae1cd89c741e409defb101f51bcc53df9:" + name))
        self.reject("differs from pristine or fully patched")

    def test_unrelated_dirty_file(self):
        with (self.source / "README.md").open("a") as f:
            f.write("\nunreviewed edit\n")
        self.reject("unrelated changes")

    def test_staged_changes(self):
        with (self.source / "mlx/event.h").open("a") as f:
            f.write("\n// staged edit\n")
        self.git("add", "mlx/event.h")
        self.reject("unchanged index")

    def test_untracked_source(self):
        (self.source / "unreviewed.cpp").write_text("// unreviewed\n")
        self.reject("untracked files")

    def test_mode_change(self):
        p = self.source / "mlx/event.h"
        p.chmod(p.stat().st_mode | 0o111)
        self.reject("file mode or type changes")

    def test_wrong_patch_digest(self):
        with (self.recipe / "mlx-v0.32.2.patch").open("a") as f:
            f.write("\n")
        self.reject("checksum mismatch")

    def test_dirty_after_application(self):
        result = self.apply()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        with (self.source / "README.md").open("a") as f:
            f.write("\nunreviewed edit\n")
        self.reject("unrelated changes")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--core-repo", required=True, type=Path)
    args, rest = parser.parse_known_args()
    CORE_REPO = args.core_repo.resolve()
    unittest.main(argv=[__file__, *rest])
