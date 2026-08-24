import fcntl
import json
import os
import signal
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


TOOLS = Path(__file__).resolve().parents[1] / "tools"
sys.path.insert(0, str(TOOLS))

from run_record import RunLock, exclusive_run_lock


HOLDER = """
import sys
from pathlib import Path
sys.path.insert(0, sys.argv[1])
from run_record import exclusive_run_lock
run_dir = Path(sys.argv[2])
with exclusive_run_lock(run_dir, sys.argv[3], ["holder"]) as lock:
    lock.record_owner()
    print("ready", flush=True)
    sys.stdin.read(1)
"""


def tree_bytes(root: Path) -> dict[str, bytes]:
    return {
        str(path.relative_to(root)): path.read_bytes()
        for path in sorted(root.rglob("*"))
        if path.is_file()
    }


class RunLockTests(unittest.TestCase):
    def assert_second_invocation_rejected(self, first_kind: str, second_kind: str) -> None:
        with tempfile.TemporaryDirectory() as directory:
            run_dir = Path(directory) / "run"
            run_dir.mkdir()
            holder = subprocess.Popen(
                [sys.executable, "-c", HOLDER, str(TOOLS), str(run_dir), first_kind],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
            )
            try:
                assert holder.stdout is not None
                self.assertEqual(holder.stdout.readline().strip(), "ready")
                before = tree_bytes(run_dir)
                with self.assertRaisesRegex(ValueError, "run lock is held") as raised:
                    with exclusive_run_lock(run_dir, second_kind, ["second"]):
                        self.fail("a second invocation acquired the run lock")
                detail = str(raised.exception)
                self.assertIn(f'"lock_acquisition_pid": {holder.pid}', detail)
                self.assertIn(f'"pid": {holder.pid}', detail)
                self.assertIn(f'"kind": "{first_kind}"', detail)
                self.assertEqual(tree_bytes(run_dir), before)
            finally:
                if holder.stdin is not None:
                    holder.stdin.write("x")
                    holder.stdin.flush()
                    holder.stdin.close()
                holder.wait(timeout=10)
                stderr = holder.stderr.read() if holder.stderr is not None else ""
                self.assertEqual(holder.returncode, 0, stderr)
            with exclusive_run_lock(run_dir, second_kind, ["after"]):
                pass

    def test_second_batch_and_e2e_invocations_cannot_acquire_run_lock(self) -> None:
        self.assert_second_invocation_rejected("model-pool-variant-batch", "model-pool-variant-batch")
        self.assert_second_invocation_rejected("model-pool-end-to-end", "model-pool-end-to-end")

    def test_inherited_descriptor_holds_lock_until_child_is_reaped(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            run_dir = Path(directory) / "run"
            run_dir.mkdir()
            descriptor = os.open(run_dir, os.O_RDONLY | getattr(os, "O_DIRECTORY", 0))
            fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
            RunLock(run_dir, "model-pool-stage-child", ["parent"], descriptor).record_owner()
            child_pid = os.fork()
            if child_pid == 0:
                try:
                    signal.pause()
                finally:
                    os._exit(0)
            os.close(descriptor)
            try:
                with self.assertRaisesRegex(ValueError, "run lock is held") as raised:
                    with exclusive_run_lock(run_dir, "model-pool-end-to-end", ["resume"]):
                        self.fail("resume acquired a lock inherited by a live child")
                detail = str(raised.exception)
                self.assertIn(f'"lock_acquisition_pid": {os.getpid()}', detail)
                owner = json.loads((run_dir / "RUN_OWNER.json").read_text())
                self.assertEqual(owner["pid"], os.getpid())
                self.assertEqual(owner["kind"], "model-pool-stage-child")
            finally:
                os.kill(child_pid, signal.SIGTERM)
                waited_pid, status = os.waitpid(child_pid, 0)
                self.assertEqual(waited_pid, child_pid)
                self.assertTrue(os.WIFSIGNALED(status))
            with exclusive_run_lock(run_dir, "model-pool-end-to-end", ["resume"]):
                pass


if __name__ == "__main__":
    unittest.main()
