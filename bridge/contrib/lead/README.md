# 一句话切主角色（作者本机原样搬运）

- `../gate/lead.py`：识别「码农，你当主」「换回主管」「让笔杆兜底」这类话（规则，不调模型；问句不算），
  当前主角色存 `GATE_LEAD_FILE`（默认 `~/.cc-connect/gate-lead.json`），gate 每次判定实时读。
  团队名单和别名取 gate 的团队文件（`gate-team.json` / `GATE_TEAM`），成员可加 `"id"`（平台用户 ID，认 `<@id>`）。
- `../gate/gate.py`：`route()` 先认切换，再按当前主角色分流（主走主的规则，其余走 sub 规则）；
  `POST /lead {"action": "get"|"set", "who": ...}` 给角色自己查/自己当主；`/context` 切换后给每个角色附一次主/次身份段。
- `lead`：命令行客户端，`lead`（查）、`lead me`（按 `CC_PROJECT` 认自己）、`lead 码农`。

需要装 gate（同作者本机）：没装 gate 时不生效。

## 主角色工作手册（谁当主谁拿到）

- `PLAYBOOK.zh-CN.md`（原文）/ `PLAYBOOK.md`（英文译本）：主角色怎么带团队并行干活。
  gate 的 `/context` 只给当前主角色附这份手册（`../gate/lead.py` 的 `playbook()`），当次的不附；换主后新主的下一轮收到一次。
  `GATE_PLAYBOOK=<文件>` 换成英文版或自己的版本，`off` = 不附。gate 部署到别的目录时，把本目录一起带上或设 `GATE_PLAYBOOK`。
- 手册里用到的脚本（都从 gate 已有的配置读，见 `_team.py`，不写死名字和 ID）：
  - `t10 <pid>`：后台活满 10 分钟连子进程一起停。
  - `d10 <队友名字/别名/ID> "<简称>"`：10 分钟后队友没交回，就以调用者（`CC_PROJECT`，即当主角色）的身份在当前频道（`CC_SESSION_KEY`）发「暂停」；队友 ID 从团队文件的 `"id"` 读。到点由 `d10-fire` 判断。
  - `ws-handoff <交接文件> "<任务>"` / `ws-forward "<摘要>"`：把大活交给「工作区」频道里的主角色、之后转结果。
    工作区频道 ID 读 `WS_CHANNEL` 或团队文件顶层 `"workspace"`；都没有就在当前频道同分类下新建一个。
    Discord token 读 `DISCORD_BOT_TOKEN` 或 `<GATE_SETUP_DIR>/secrets.env`。
