# 联机框架（netplay 包）

`guis/netplay` 是框架内的**局域网联机基础设施**：TCP 分帧传输、带版本的消息协议、
握手与心跳、房间广播、快照插值与帧同步。纯标准库、零 cgo、零第三方依赖。

- 包路径：`guis/netplay`
- 定位：**局域网直连**（不做中继 / 匹配 / 账号 / 反作式 / 回滚网络）
- 配套：单机游戏引擎见 `63-3d-game.md`，联机只是把「谁动了」改由网络广播

---

## 1. 三十行跑起来

服务端：

```go
srv, err := netplay.Listen("0.0.0.0:9000")
if err != nil {
	log.Fatal(err)
}
defer srv.Close()

srv.OnJoin = func(p *netplay.Peer) {
	log.Println("加入：", p.ID, p.RemoteAddr())
	p.Send(netplay.MsgChat, []byte("welcome"))
}
srv.OnLeave = func(p *netplay.Peer) { log.Println("离开：", p.ID) }

// 每 50ms 广播一次快照（真实项目里由游戏循环驱动）
for tick := uint32(1); ; tick++ {
	payload, _ := netplay.EncodeJSON(netplay.Snapshot{Tick: tick, Time: float64(tick) / 20})
	srv.Broadcast(netplay.MsgSnapshot, payload)
	time.Sleep(50 * time.Millisecond)
}
```

客户端：

```go
cli, err := netplay.Dial("192.168.1.10:9000", "player1", 3*time.Second)
if err != nil {
	log.Fatal(err)
}
defer cli.Close()
log.Println("我的玩家 ID：", cli.ID())

interp := netplay.NewInterpolator()
cli.Peer().On(netplay.MsgSnapshot, func(_ *netplay.Peer, m netplay.Message) {
	var s netplay.Snapshot
	if err := netplay.DecodeJSON(m.Payload, &s); err == nil {
		interp.Push(s)
	}
})
cli.Peer().On(netplay.MsgChat, func(_ *netplay.Peer, m netplay.Message) {
	log.Println("聊天：", string(m.Payload))
})
```

---

## 2. 传输层（Transport）

所有消息通道实现同一个接口，上层只看到「一条完整消息」，分帧与长度校验在内部完成：

```go
type Transport interface {
	io.Closer
	SendMessage(b []byte) error        // 并发安全
	RecvMessage() ([]byte, error)      // 只给 Peer 的读循环用
	RemoteAddr() string
}
```

| 构造 | 底层 | 语义 |
|---|---|---|
| `NewTCPTransport(conn)` | 4 字节大端长度前缀 | 有序、可靠，适合状态同步 / 控制消息 |
| `NewUDPTransport(conn, max)` | 一个 UDP 包 = 一条消息 | 不可靠、不保序，适合帧同步输入「丢了就丢」 |
| `DialTCP(addr, timeout)` | TCP | 返回已连接的 `Transport`（timeout ≤ 0 用 5 秒） |

- 单帧上限 `MaxFrameSize = 1 MiB`，超长发送返回 `ErrFrameTooLarge`；
- 接收端读到超过上限的长度字段直接报错断开（防止对端一个长度字段把内存拉爆）；
- 需要 TLS 时自己 `tls.Dial` 后用 `NewTCPTransport(conn)` 包一层，协议层不用改。

---

## 3. 消息协议

帧头 12 字节：

| 偏移 | 长度 | 字段 | 说明 |
|---|---|---|---|
| 0 | 2 | magic | `0x4750`（“GP”） |
| 2 | 1 | version | 当前 `1`，不匹配直接拒绝 |
| 3 | 1 | type | `MsgType` |
| 4 | 4 | seq | 发送方自增序号（仅诊断用，不做重传） |
| 8 | 4 | len | 载荷长度 |

消息类型：`MsgHandshake` `MsgHandshakeAck` `MsgSnapshot` `MsgInput` `MsgChat` `MsgPing` `MsgPong` `MsgJoin` `MsgLeave`（1–9）。

```go
frame, err := netplay.EncodeMessage(netplay.Message{Type: netplay.MsgChat, Payload: []byte("hi")})
msg, err := netplay.DecodeMessage(frame)   // 校验魔数 / 版本 / 长度
```

载荷格式由业务自定：默认提供 `EncodeJSON` / `DecodeJSON` 辅助，性能敏感时直接写二进制。

---

## 4. 握手与连接生命周期

1. 客户端 `Dial` → 发 `MsgHandshake{version, name}`；
2. 服务端校验版本：不匹配回 `MsgHandshakeAck{message:"协议版本不匹配"}` 并断开；
3. 匹配则分配玩家 ID（从 1 递增）回 `MsgHandshakeAck{version, id}`；
4. 服务端调用 `OnJoin`，并向全房间广播 `MsgJoin`；
5. 双方进入正常收发（`Peer` 读循环已启动）。

注意：

- **`Dial` / `Listen` 返回时握手已完成**，不要再调 `Peer.Start()`；
- 服务端握手等待 5 秒，超时断开（只影响握手阶段）；
- 新玩家加入前广播的消息它收不到（直连，无历史回放）—— 新加入后请主动发一次完整状态。

---

## 5. Peer：收发、分发、心跳

```go
p := cli.Peer()
p.On(netplay.MsgInput, func(peer *netplay.Peer, m netplay.Message) {
	log.Println("来自", peer.ID, "的输入", len(m.Payload), "字节")
})
p.Send(netplay.MsgInput, input)     // 并发安全，序号自动递增
p.RemoteAddr()                      // “1.2.3.4:5678”
p.LastRecv()                        // 最近一次收到消息的时间
```

心跳与超时（默认都关闭，需要自己开）：

```go
p.Heartbeat = 2 * time.Second   // 每 2s 发 MsgPing（对端自动回 MsgPong，不会进业务回调）
p.Timeout = 6 * time.Second     // 超时未收到任何消息即断开
p.OnDisconnect = func(p *netplay.Peer, err error) { log.Println("断开:", err) }
```

- `OnDisconnect` 只会回调一次（主动 `Close` 也会触发，`err` 为对端/本地错误）；
- 消息处理在 Peer 的读协程里同步执行，回调里不要做长时间阻塞操作。

---

## 6. Server / Client

```go
srv.Addr()                                   // 实际监听地址（addr 端口传 0 时用来取端口）
srv.Peers()                                  // 在线玩家快照
srv.Broadcast(netplay.MsgChat, []byte("hi")) // 返回成功发送人数
srv.BroadcastExcept(exceptID, netplay.MsgChat, payload)
srv.Close()                                  // 关闭监听并断开所有人

cli.ID()      // 自己的玩家 ID
cli.Peer()    // 自己的 Peer
cli.Close()
```

典型用法：服务端自身也跑一个 `game.Engine`，把权威状态按 tick 广播；客户端收到快照后插值渲染。

---

## 7. 同步：快照插值与帧同步

### 7.1 状态同步（服务器权威 + 客户端插值）

```go
interp := netplay.NewInterpolator()   // 默认保留 16 帧
interp.Push(netplay.Snapshot{Tick: 7, Time: 0.35, Data: stateBytes})

prev, next, alpha, ok := interp.Sample(now)   // now 通常是「当前时间 - 100ms」
if ok {
	// 用 alpha 在 prev.Data / next.Data 之间插值（位置 / 旋转……）
}
interp.Latest()                               // 最新一帧（追帧 / 初始化用）
interp.Len()
```

- 按 `Tick` 去重、按 `Time` 排序，超出 `MaxBuffered` 丢最旧的；
- `now` 早于最早快照 / 晚于最新快照时返回同一帧且 `alpha = 0`（不会越界）；
- 缓冲延时（interp delay）建议 ≥ 2 个 tick，否则网络抖动会造成回插。

### 7.2 帧同步（确定性强同步）

```go
ls := netplay.NewLockstep(2)      // 2 个玩家
ls.Submit(cli.ID(), inputBytes)   // 每个玩家每 tick 提交一次
if ls.Pending() == 0 {
	tick, inputs, ok := ls.Advance()   // ok=false 表示输入不齐
	if ok {
		// inputs 已按玩家 ID 排序（map[uint64][]byte），保证确定性执行顺序
		_ = tick
	}
}
```

帧同步要求游戏逻辑是确定性的（固定步长、无随机、无浮点差异），本包只负责「收齐输入后统一放行」。

---

## 8. 已知限制

- 仅局域网 / 直连，**没有**中继、NAT 穿透、匹配、房间列表、账号体系；
- TCP 传输没有重传队列与拥塞控制调优（就靠内核），抖动大的网络请自己加冗余与平滑；
- 序号 `seq` 只用于诊断，不做丢包检测与排序（UDP 上需自己处理乱序）；
- 快照 / 输入载荷默认用 JSON 辅助，带宽敏感场景请自己写紧凑二进制；
- 服务端为单进程内循环，玩家数到百级以上需要分片与限流（本包未提供）。

---

## 9. API 索引

- 传输：`Transport` `NewTCPTransport` `NewUDPTransport` `DialTCP` `MaxFrameSize` `ErrFrameTooLarge` `ErrClosed`
- 协议：`Message` `MsgType` `MsgHandshake` `MsgHandshakeAck` `MsgSnapshot` `MsgInput` `MsgChat` `MsgPing` `MsgPong` `MsgJoin` `MsgLeave` `EncodeMessage` `DecodeMessage` `EncodeJSON` `DecodeJSON` `Handshake` `HandshakeAck` `Magic` `Version`
- 连接：`Peer` `NewPeer` `Peer.Start` `Peer.Send` `Peer.On` `Peer.Close` `Peer.LastRecv` `Peer.RemoteAddr`（字段 `ID` `Timeout` `Heartbeat` `OnDisconnect`）
- 服务端：`Server` `Listen` `Server.Addr` `Server.Peers` `Server.Broadcast` `Server.BroadcastExcept` `Server.Close`（字段 `OnJoin` `OnLeave`）
- 客户端：`Client` `Dial` `Client.ID` `Client.Peer` `Client.Close`
- 同步：`Snapshot` `Interpolator` `NewInterpolator` `Interpolator.Push` `Interpolator.Sample` `Interpolator.Latest` `Interpolator.Len` `Lockstep` `NewLockstep` `Lockstep.Submit` `Lockstep.Pending` `Lockstep.Advance` `Lockstep.Tick` `Lockstep.Players`
