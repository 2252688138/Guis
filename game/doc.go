// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// Package game 为 guis 提供轻量 3D 游戏开发能力：纯 Go 软件光栅化 3D 引擎，
// 不依赖 GPU 后端、不依赖 cgo，六端（Windows/Linux/macOS/iOS/Android/WASM）行为一致。
//
// # 快速开始
//
// 最小可跑示例（放进 guis 应用的页面里）：
//
//	eng := game.NewEngine(640, 360)
//	mat := game.DefaultMaterial().WithTexture(game.CheckerTexture(8, game.RGB(200, 210, 230), game.RGB(90, 110, 140)))
//	cube := game.NewMeshNode("cube", game.NewBox(1, 1, 1), mat)
//	eng.Scene.Add(cube)
//	eng.Scene.Add(game.NewMeshNode("ground", game.NewPlane(10, 10, 1), game.DefaultMaterial()))
//	eng.Scene.AddSun(game.V3(-0.4, -1, -0.6), 0.9)
//	ui.NewApp(ui.Options{Title: "3D", Width: 900, Height: 600}, func() ui.Node {
//		return ui.Stack(1).Children(ui.Viewport3D(&ui.Viewport3DOpts{Src: eng}))
//	})
//
// # 核心概念
//
//   - Scene / Node：场景树，节点自带网格与材质，可嵌套，每帧重算世界矩阵。
//   - Camera：透视或正交相机，提供 LookAt / 视锥剔除 / 屏幕射线。
//   - Renderer：软件光栅化（Z-Buffer / 透视校正 / 逐像素 Phong / 近平面临剪）。
//   - Loop：固定步长（默认 1/60）更新循环，带追赶上限，避免长时间卡顿雪崩。
//   - Input：键鼠 + 触摸状态表，提供 Keys/Pressed/Released 与 Axis。
//   - Sprite2D：在 3D 之上叠加 HUD 精灵 / 瓦片地图。
//
// # 2D 与 HUD
//
// Sprite2D + SpriteBatch 直接画到 *image.RGBA 帧上：Camera2D（缩放/旋转/震屏）、
// Node2D/SpriteAnim（帧动画 + 事件帧）、NineSlice、Parallax、TileCollision、
// 位图字体与内置 5×7 字体（BuiltinFont）可组合出一套完整 2D 场景。
// Tiled 地图：LoadTiledMap / LoadTiledJSON（含对象层、动画瓦片、无限地图，见 tiled_ext.go）。
//
// # 文本与字体
//
// BuiltinFont 只含 ASCII；需要中文或平滑字形时用 LoadTTF / LoadTTFFile 把
// TTF/OTF 烘焙成位图字体（等宽格推进，字符集按需给，例如 "分数生命0123"）。
// .ttc 字体集合不支持，请改用 .ttf/.otf。
//
// # 操作手感
//
//   - Tween / Timeline：时间轴补间，支持延迟、循环、乒乓与结束回调（tween.go）。
//   - 缓动曲线：EaseLinear / EaseOutCubic / EaseOutBack / EaseOutElastic / EaseOutBounce 等 20+ 条。
//   - StateMachine：状态 + 转移条件 + 事件，适合角色与菜单流程。
//   - Actions / Gamepad / Input：键鼠触摸状态表、动作映射、手柄（Windows 后端，其他平台空实现）。
//
// # 物理与音频
//
// Body 是“竖直圆柱 + AABB”的运动学刚体（重力/阻力/最大速度/贴地判定）；
// Character 在其上提供 MoveAndSlide 与跳跃，CollisionGrid / TileCollision 负责
// 静态场景解析，NavGrid 提供网格寻路。没有刚体间动力学与摩擦材质，需要时自行叠加。
// 音频：LoadSound 按内容嗅探 WAV / MP3 / OGG，Mixer 负责分组音量与混音，
// MusicStream 支持流式播放，AudioSpatial 提供距离衰减。
// 输出后端按平台选：Windows 走 waveOut、Linux 走 OSS（/dev/dsp）、浏览器走 WebAudio，
// 其余平台（macOS / Android / 其他 Unix）走「外部命令管道」——
// OpenPlayerWithCommand 把 S16LE 裸流写进 aplay/paplay/sox/ffplay 的 stdin，
// 也可用环境变量 GUIS_AUDIO_CMD 指定命令；iOS 因沙箱限制返回 NopPlayer（静音）。
// 用 StatusReporter 的 Status().OK 判断当前是否真的能出声。
//
// # 模型导入
//
//	LoadOBJ / LoadOBJBytes       Wavefront OBJ（v/vt/vn/f，多边形自动三角化）
//	LoadGLTFFile / LoadGLTFBytes glTF 2.0 与 GLB（内嵌 base64 / BIN chunk / 外部 .bin）
//	LoadSkin / MarshalSkin      自带骨骼蒙皮格式
//
// glTF 会按节点层级把世界变换烘焙进顶点，primitive 转成独立网格，
// baseColorFactor / emissiveFactor / doubleSided 映射到材质；只支持三角形图元（mode 4），
// 不支持动画、蒙皮、贴图与 KHR 扩展。
//
// # 存档与回放
//
// SaveStore 支持版本迁移、CRC32 校验与异或混淆；Recorder/Replay 记录确定性输入
// 以便复现。SceneJSON / SaveScene 负责把场景网格与节点写回 JSON。
//
// # GPU 渲染
//
// gpu.go 提供跨后端 GPU 抽象：Device / Buffer / Texture / Pipeline / RenderGraph，
// 一次提交（Begin → 录制命令 → Submit）即可跑完整个场景。
// 内置两个后端：soft（多线程 tile 光栅化，纯 Go，任何平台可用）与
// gl（Windows 上通过 purego 动态加载 opengl32.dll，离屏 FBO 渲染后回读像素）。
// 用 NewGPU(WithGPUBackend("auto")) 自动探测，硬件不可用时回退 soft；
// 用 NewGPUScene(g, w, h) 把 Scene 渲染成 *image.RGBA。
// 能力与统计见 GPU.Caps() / GPU.Stats()，两后端对同一场景的结果一致（有测试保证）。
//
// # 大型游戏支撑
//
//   - ECS：NewECSWorld + Add/Get/Query1..3 + System("pre"/"update"/"post")，
//     稀疏集存储，实体句柄带代号，删除后旧句柄自动失效；SyncToScene 可把
//     Transform + Renderable 实体同步进 Scene。
//   - 作业系统：NewJobs(n).Do / Run / ParallelReduce 并行处理独立任务；
//     Scheduler 按毫秒预算把大任务摊到多帧。
//   - 可见性：Octree（八叉树范围 / 球 / 视锥查询）、OcclusionCuller（软件遮挡剔除）、
//     CollectVisible（视锥剔除收集节点），配 Node.WorldBounds 使用。
//   - 地形：Heightmap（双线性高度 + 法线）、ChunkMesh（步长 LOD）、
//     Terrain.Update 按相机距离切换块 LOD，NoiseTerrain 生成可复现噪声地形。
//   - 批量绘制：InstanceBatch 直接接渲染管线（Node.Instances / Renderer 合批），
//     几百上千个重复物体只算1 次提交；MergeMeshes 做静态合批。
//   - 流式加载：LoadAsync 去重异步加载，Streaming 按相机位置加载/卸载世界分块，
//     MaxMountPerFrame 限制每帧挂载数、Stats() 看加载/卸载计数。
//   - 材质：ShadePBR（Cook-Torrance：金属度 / 粗糙度）+ 切线空间法线贴图 /
//     金属度粗糙度打包图 / AO 图，切线由三角形世界坐标与 UV 推导。
//   - 阴影：ShadowMap（单张）或 CSMShadow（级联，按视锥分级、级间混合）。
//
// # 性能
//
// 软件光栅化的开销与内部渲染分辨率 × 像素数成正比，建议 480×270 ~ 960×540；
// Renderer.Threads 默认等于 GOMAXPROCS，按屏幕行分带并行光栅化。
// 用 Renderer.Stats() / Engine.FrameStats() 观察 Triangles / Pixels / Ms。
//
// # 已知限制
//
// 不支持自定义 GLSL 着色器（着色模型固定为 Unlit/Flat/Gouraud/Phong/Toon/PBR），
// PBR 也没有 IBL 预滤波；半透明仍按三角形深度粗略排序。
// 骨骼蒙皮是 CPU 每帧重算顶点，适合低模角色（1000 顶点约 0.1 ms），高模请配合 LOD；
// 暂无 GPU 蒙皮与网络同步。
// 音频实际发声：Windows（winmm）、Linux（OSS，仅 /dev/dsp 存在时）、
// 浏览器（WASM + WebAudio）；macOS / iOS / Android 仍返回静音播放器。
// 联机只有 netplay 包的局域网直连（无中继 / 匹配 / 回滚网络）。
// HUD 文字请用 ui 现有文本控件叠加。
//
// # 测试
//
// 测试集中在 game/test 子包（package game_test），只使用导出 API；
// 内部的摇杆死区、手柄位映射、帧缓冲尺寸等细节由 export_testing.go 的 Test* 桥接暴露，
// 那些 Test* 符号仅供测试使用，不属于稳定公开 API。
package game
