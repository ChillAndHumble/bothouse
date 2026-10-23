"""当主注入含主角色工作手册、当次不含：python3 -I contrib/gate/test_lead_playbook.py（不加载模型）"""
import json, os, sys, tempfile, unittest
from pathlib import Path

_tmp = tempfile.mkdtemp()
os.environ["GATE_SETUP_DIR"] = _tmp
os.environ["GATE_LEAD_FILE"] = str(Path(_tmp) / "gate-lead.json")
os.environ["GATE_WIKI"] = "off"
for k in ("GATE_TEAM", "GATE_PLAYBOOK"):
    os.environ.pop(k, None)
sys.path.insert(0, str(Path(__file__).resolve().parent))
import ctxsvc, lead  # noqa: E402

MARK = "队友空着，就给她们安排活"


def has_playbook(bot):
    return any(MARK in (x.get("desc") or "") for x in ctxsvc.context({"bot": bot, "text": "在吗"})["wiki"])


class T(unittest.TestCase):
    def tearDown(self):
        Path(os.environ["GATE_LEAD_FILE"]).unlink(missing_ok=True)

    def test_default_lead_gets_playbook_subs_do_not(self):
        self.assertTrue(has_playbook(""))
        for k in lead.KEYS[1:]:
            self.assertFalse(has_playbook(k), k)

    def test_after_switch_new_lead_gets_it_old_lead_does_not(self):
        new = lead.KEYS[1]
        lead.set_lead(new)
        self.assertTrue(has_playbook(new))
        self.assertFalse(has_playbook(""))
        self.assertIn(lead.state()["at"], lead.playbook(new)["path"])  # 换主后 path 变，cc-connect 会再推一次

    def test_first_rule_is_assign_idle_teammates(self):
        for name in ("PLAYBOOK.zh-CN.md", "PLAYBOOK.md"):
            text = (Path(__file__).resolve().parent.parent / "lead" / name).read_text(encoding="utf-8")
            first = next(l for l in text.splitlines() if l.startswith("## "))
            self.assertTrue("队友空着" in first or "idle" in first, (name, first))


if __name__ == "__main__":
    unittest.main()
