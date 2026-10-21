# bothouse 开源准备核验（2026-10-21）

本轮由龙娘接续，未启动子 Agent。当前内容为待审核分支，未发布、未部署。

## 变更

- 从 conest #8/#9 同步自建平台多角色的点名 ID、自动交回与人设队伍行。
- 修复旧队长 ID 改动后已有队员仍交给旧 ID 的问题，保留手填的团队外交回对象。
- 同步用户指定的 MIT、中英软件首页和宣传片脚本草稿；不带回旧首页存档和未制作的视频。
- 修正首页与团队文档中已经移除的 3 轮／20 次接力限制说明；目标、状态和架构按软件定位整理。
- 修正中文终端测试被宿主 LC_ALL 覆盖的问题。

## 回归证据

`TestApplyWizardCloudWebTeamLeadIDChangeUpdatesEarlierMembers` 在补丁前失败：

```text
sub still hands back to <old ID>; want current lead 222222222222222222
```

补丁后同一测试通过，覆盖已有队员、新队员、手填交回对象及重复配置。

## 本机执行结果

| 核验 | 结果 |
| --- | --- |
| `make hubsrc` | 通过 |
| `go test -tags no_web -p 1 -count=1 ./...` | 47 个有测试的包通过 |
| `go test -tags no_web -p 1 -count=1 ./core -run TestCUJ` | 通过 |
| `go vet -tags no_web ./core ./cmd/cc-connect` | 通过 |
| `GOOS=windows GOARCH=amd64 go build -tags no_web ./...` | 通过 |
| pnpm 10 锁定依赖安装、`npm run build` | 通过；Vite 提示产物分块较大 |
| Camoufox 网页向导 | 选自建平台、填写测试 hub、配置两个角色并保存；交回 ID 与两个人设队伍行正确，前端异常 0 |
| 编译后的 `setup apply --json ... --config ...` | 对网页保存的配置修改队长 ID，再添加队员、重复运行两次；共 3 个角色，旧队员和新队员都指向新队长 |
| 新首页与本轮目标、状态、架构、许可文档的本地链接 | 已核对 |

Go 使用项目的测试沙箱；网页与 CLI 使用隔离配置、测试 WebSocket 握手服务及未登录的 HOME。页面明确显示 `agent_not_logged_in`，保存配置不等于角色已经能回答。

## 未完成

- 本仓库尚无 Release，不能宣称新仓库的一行命令安装通过。
- 未用本次分支重跑真实模型的派活、交回、汇总。
- Windows 仅交叉构建，真实桌面的启动窗口行为未验证。
- 原 conest #9 的 UI preview 曾因 GitHub artifact 存储配额耗尽而失败；未删除历史产物或绕过检查。
- 视频等待用户看稿后才制作；仓库继续保持私有。

本轮新增补丁仍需独立审核，测试通过不代表最终验收。
