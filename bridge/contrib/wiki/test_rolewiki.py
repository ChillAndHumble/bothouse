#!/usr/bin/env python3
"""Tests of the per-role wiki pieces the setup wizard uses: export.py kinds (claudecode, dsh default home, a type with
no reader = diary only), the on/off switch and --list, setup_wiki.py from the plain template (main and role wiki).
Run: python3 -I bridge/contrib/wiki/test_rolewiki.py"""
import json, os, re, subprocess, sys, tempfile, unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent


class RoleWiki(unittest.TestCase):
    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp())
        self.home = self.tmp / "home"
        self.home.mkdir()
        self.bots = self.tmp / "bots.json"
        self.env = dict(os.environ, HOME=str(self.home), WIKI_BOTS=str(self.bots), WIKI_TRANSCRIPTS=str(self.tmp / "tr"),
                        LLM_WIKI_SRC=str(HERE / "template"), WIKI_DIR=str(self.home / "wiki"))
        self.env.pop("DSH_HOME", None)

    def run_py(self, script, *args):
        return subprocess.run([sys.executable, "-I", str(HERE / script), *args], env=self.env, capture_output=True,
                              text=True, check=True).stdout

    def role(self, name, kind):
        wd = self.home / "roles" / name
        wd.mkdir(parents=True)
        all_ = json.loads(self.bots.read_text()) if self.bots.exists() else {}
        all_[name] = {"name": name, "project": name, "kind": kind, "workdir": str(wd)}
        self.bots.write_text(json.dumps(all_))
        return wd

    def test_claudecode_kind(self):
        wd = self.role("juzi", "claudecode")
        d = self.home / ".claude" / "projects" / re.sub(r"[^A-Za-z0-9]", "-", str(wd))
        d.mkdir(parents=True)
        lines = [{"type": "user", "timestamp": "2026-10-22T01:00:00.000Z", "message": {"role": "user", "content": "你好橘"}},
                 {"type": "assistant", "timestamp": "2026-10-22T01:00:01.000Z", "message": {"role": "assistant", "content": [
                     {"type": "text", "text": "在呢"}, {"type": "tool_use", "name": "Bash", "input": {"command": "ls"}}]}},
                 {"type": "user", "message": {"role": "user", "content": [{"type": "tool_result", "content": "x"}]}},
                 {"type": "user", "isSidechain": True, "message": {"role": "user", "content": "subagent"}}]
        (d / "abcdef12-3456.jsonl").write_text("\n".join(json.dumps(x, ensure_ascii=False) for x in lines))
        self.run_py("export.py", "juzi")
        out = list((self.tmp / "tr" / "juzi" / "claude").glob("*.jsonl"))
        self.assertEqual(len(out), 1)
        recs = [json.loads(l) for l in out[0].read_text().splitlines()]
        self.assertEqual([r["type"] for r in recs], ["user", "assistant", "assistant"])
        self.assertEqual(recs[0]["message"]["content"], "你好橘")
        self.assertEqual(recs[2]["message"]["content"][0]["input"]["command"], "ls")

    def test_dsh_kind_default_home_and_diary_only(self):
        wd = self.role("whale", "dsh")
        s = self.home / ".dsh" / "sessions" / "x" / "sid1"
        s.mkdir(parents=True)
        recs = [{"type": "session", "id": "sid1", "cwd": str(wd), "delegationDepth": 0},
                {"type": "user/message", "time": 1761094800000, "data": {"source": {"kind": "user"}, "content": [{"type": "text", "text": "查一下天气"}]}},
                {"type": "assistant/message", "time": 1761094801000, "data": {"message": {"content": [{"type": "text", "text": "晴"}]}}}]
        (s / "session.v4.jsonl").write_text("\n".join(json.dumps(r, ensure_ascii=False) for r in recs) + "\n")
        wd2 = self.role("cat", "gemini")  # no reader of its own: the diary only
        (wd2 / "diary.md").write_text("2026-10-22 10:00 今天学会了一个新梗\n")
        out = self.run_py("export.py", "--count")
        self.assertIn("whale {'codex': 0, 'agy': 0, 'claude': 0, 'dsh': 1, 'diary': 0}", out)
        self.assertIn("cat {'codex': 0, 'agy': 0, 'claude': 0, 'dsh': 0, 'diary': 1}", out)
        self.run_py("export.py", "whale")
        f = next((self.tmp / "tr" / "whale" / "dsh").glob("*.jsonl"))
        self.assertIn("查一下天气", f.read_text())

    def test_switch_and_list(self):
        self.role("whale", "codex")
        self.role("cat", "codex")
        self.run_py("export.py", "--switch", "cat", "wiki", "off")
        self.assertEqual([l.split()[0] for l in self.run_py("export.py", "--list").splitlines()], ["whale"])
        self.run_py("export.py", "--switch", "cat", "wiki", "on")
        self.assertEqual(len(self.run_py("export.py", "--list").splitlines()), 2)
        (self.home / "roles" / "whale" / "wiki").mkdir()
        self.run_py("export.py", "--switch", "whale", "dream", "off")
        self.assertIs(json.loads((self.home / "roles" / "whale" / "wiki" / "config.json").read_text())["dream"], False)

    def test_setup_from_template(self):
        self.env["WIKI_ENGINES"] = json.dumps({"ingest_engine": "haiku", "night_engine": "sonnet", "bogus": 1})
        self.run_py("setup_wiki.py", "--main", str(self.home / "wiki"), "主管")
        self.assertTrue((self.home / "wiki" / "bin" / "ingest.py").exists())
        self.assertIn("exists", self.run_py("setup_wiki.py", "--main", str(self.home / "wiki"), "主管"))
        self.role("whale", "dsh")
        self.run_py("setup_wiki.py", "whale")
        w = self.home / "roles" / "whale" / "wiki"
        cfg = json.loads((w / "config.json").read_text())
        self.assertEqual((cfg["ingest_engine"], cfg["night_engine"]), ("haiku", "sonnet"))
        self.assertNotIn("bogus", cfg)
        self.assertTrue((w / ".gitignore").exists() and (w / "experiences" / "README.md").exists())
        self.assertIn("exists", self.run_py("setup_wiki.py", "whale"))
        # the shared tool accepts the role wiki's config.json (no unknown keys)
        r = subprocess.run([sys.executable, "-I", "-c", "import runpy,sys; sys.argv=['ingest.py','--help']; "
                            f"runpy.run_path({str(w / 'bin' / 'ingest.py')!r}, run_name='__main__')"],
                           env=self.env, capture_output=True, text=True)
        self.assertNotIn("unknown key", r.stdout + r.stderr)

    def test_setup_null_engines_and_half_made(self):
        self.env["WIKI_ENGINES"] = "null"  # Go's json.Marshal of a nil map
        self.run_py("setup_wiki.py", "--main", str(self.home / "wiki"), "主管")
        self.role("whale", "dsh")
        w = self.home / "roles" / "whale" / "wiki"
        self.run_py("setup_wiki.py", "whale")
        (w / "config.json").unlink()  # a crash after the template copy, before config.json
        self.assertNotIn("exists", self.run_py("setup_wiki.py", "whale"))
        self.assertTrue((w / "config.json").exists())


if __name__ == "__main__":
    unittest.main()
