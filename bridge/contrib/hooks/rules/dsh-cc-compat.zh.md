DSH 兼容辅助文件，不独立注册为 hook。run.py 继续处理角色开关、工具名和 Stop 转录，并复用该文件的本轮调用/忙碌任务读取。会话日志使用随包的 dshlog.py，无作者本机依赖。命令：python3 dsh-cc-compat.py --sessions <DSH_HOME>/sessions <hook.py>。
