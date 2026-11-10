# 记忆列表

## 模拟组合键验证GUI

- 范围: 全局
- 关键词: keybd_event, Ctrl+Alt+Down, keyboard_press, 多光标验证
- 更新时间: 2026/11/5 22:02:48

实机验证 guis 组合键（如 Ctrl+Alt+↓ 加光标）时，computer_control 的 keyboard_press 带 modifiers 不生效，改用 PowerShell 的 Add-Type 声明 user32 keybd_event 逐个按下/抬起（SetForegroundWindow 先行）才能触发。
## write_lines会截断文件

- 范围: 全局
- 关键词: batch_file_operation, write_lines, overwrite, 截断, 文件丢失
- 更新时间: 2026/11/5 22:38:09

batch_file_operation 的 write_lines 在 overwrite=true 时会把文件从 start_line 之后整体截断（而不是只替换等量行），会丢掉后续内容；局部改代码应改用 replace_text 或 skill_file_operation replace_text。
