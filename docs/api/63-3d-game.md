# 3D 游戏开发（game 包）

`guis/game` 是框架内的**纯 Go 软件光栅化 3D 引擎**，不依赖 GPU 后端、不依赖 cgo，
六端（Windows / Linux / macOS / iOS / Android / WASM）行为完全一致。

通过 `ui.Viewport3D` 控件接入 UI，引擎每帧输出一帧 RGBA 像素，控件负责上传纹理并绘制。

- 包路径：`guis/game`
- 控件：`ui.Viewport3D(&ui.Viewport3DOpts{...})`
- 示例：`examples/game3d`
- 联机：`guis/netplay`，文档见 [`64-netplay.md`](64-netplay.md)

---

## 1. 三十行跑起来

```go
package main

import (
	"log"

	"guis/game"
	"guis/ui"
)

func main() {
	eng := game.NewEngine(640, 360) // 内部渲染分辨率

	// 场景：地面 + 旋转立方体
	eng.Scene.AddSun(game.V3(-0.4, -1, -0.6), 1.0)
	eng.Scene.SetAmbientIntensity(0.3)

	ground := game.NewMeshNode("ground", game.NewPlane(20, 20, 1),
		game.DefaultMaterial().WithShade(game.ShadeFlat))
	cube := game.NewMeshNode("cube", game.NewBox(1, 1, 1),
		game.DefaultMaterial().
			WithColor(game.RGB(240, 180, 90)).
			WithTexture(game.CheckerTexture(8, game.RGB(255, 255, 255), game.RGB(160, 120, 80))))
	cube.Pos = game.V3(0, 0.5, 0)
	eng.Scene.Add(ground, cube)

	// 相机：轨道环绕
	orbit := game.NewOrbit(eng.Camera, game.V3(0, 0.5, 0), 5)
	orbit.Pitch = 0.4
	orbit.Apply()

	// 每帧自转
	game.NewAnimator().Spin(cube, game.V3(0, 1, 0), 1.2)

	err := ui.Run(ui.Options{Title: "3D", Width: 900, Height: 600}, func() *ui.Node {
		return ui.Column(
			ui.Viewport3D(&ui.Viewport3DOpts{
				Src:       eng,
				AutoFocus: true,
				OnKey:     eng.HandleKey,
				OnMouse:   eng.HandleMouse,
				OnWheel:   eng.HandleWheel,
			}).Grow(1),
		)
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

---

## 2. 核心概念

### 2.1 场景与节点

`Scene` 持有根节点 + 相机 + 光源 + 全局环境（`Ambient` / `Fog` / `Background`）。
`Node` 可嵌套，每帧渲染前递归重算世界矩阵。

```go
group := game.NewNode("group").Add(a, b)   // 组合变换
sc.Add(group)
sc.Find("cube")                            // 按名字递归查找
sc.Each(func(n *game.Node) { })            // 遍历
```

节点变换三件套：`Pos` / `Rot`（欧拉角弧度）/ `Scl`；也可直接给四元数 `Quat`（优先于 `Rot`）。

### 2.2 相机

```go
cam := sc.Camera
cam.Pos = game.V3(0, 2, 6)
cam.Target = game.V3(0, 0, 0)
cam.FOVY = 60 // 度
cam.Ortho = true; cam.OrthoSize = 5 // 正交模式
cam.Frustum(aspect)                 // 视锥体（渲染器内部用它做物体级剔除）
cam.Ray(px, py, w, h)               // 屏幕像素 → 世界射线
```

控制器：`game.Orbit`（轨道：拖拽旋转 + 滚轮缩放）、`game.Fly`（第一人称：`Look(dx,dy)` + `Move(fwd,right,up,dt)`）。

### 2.3 材质与着色

```go
mat := game.DefaultMaterial()
mat.BaseColor = game.RGB(200, 120, 60)
mat.Texture = game.CheckerTexture(8, a, b)
mat.Shade = game.ShadePhong      // Unlit / Flat / Gouraud / Phong / Toon
mat.Specular, mat.Shininess = 0.6, 48
mat.Alpha = 0.6                  // <1 进入透明队列（按深度粗略排序）
mat.DoubleSided = true           // 关闭背面剔除
mat.Wireframe = true             // 线框模式
mat.Nearest = true               // 纹理最近邻采样（像素风）
```

光源：`Scene.AddSun(dir, intensity)`（平行光）、`Scene.AddPointLight(pos, range, intensity)`（点光，平方衰减）。
建议 ≤ 4 个光源（逐像素遍历）。

### 2.4 网格

内置：`NewBox` / `NewSphere` / `NewPlane` / `NewCylinder` / `NewTorus`；也可以 `game.LoadOBJ(path)`。
自建网格只需 `Positions` + `Indices`，法线缺失时调用 `RecomputeNormals()`。

```go
m := game.NewMesh(pos, idx)
m.RecomputeNormals()
m.RecomputeBounds()
m.Transformed(game.Rotate(game.V3(0,1,0), 1))  // 返回新网格
```

### 2.5 渲染器

```go
r := game.NewRenderer(640, 360)
r.Threads = 0            // 0 = GOMAXPROCS（按屏幕行分带并行）
r.Supersample = 2        // 2×2 超采样抗锯齿（1=关闭，2/3/4 可选；开销≈ss²）
r.BgColor = game.RGB(18, 20, 26)
img := r.Frame(scene)    // *image.RGBA，内部缓冲复用
st := r.Stats()          // Triangles / Culled / Pixels / Objects / Sprites / Ms
```

`Supersample > 1` 时内部帧缓冲按 `W*ss × H*ss` 光栅化，再盒式滤波降采样到 `W×H`，
输出尺寸不变、边缘无锯齿；`Stats().Pixels` 计的是采样点数量。

管线：顶点变换 → 近平面裁剪（Sutherland–Hodgman）→ 屏幕映射 → 背面剔除 → 重心坐标遍历 + Z-Buffer
→ 透视校正插值 → 片元着色（Phong/Toon/…）→ 透明队列排序绘制 → 2D 叠加层。

### 2.6 引擎与循环

```go
eng := game.NewEngine(640, 360)
eng.SetGame(myGame)      // 实现 game.Game：Init / Update(dt) / FixedUpdate(step) / Draw(sc)
eng.Paused = true
eng.Tick(dt)             // 推进逻辑（固定步长默认 1/60，追赶上限 8）
eng.Render()             // 只渲染
pix, w, h := eng.RenderViewport(ctrlW, ctrlH, dt)  // UI 用（实现 ui.ViewportSource）
eng.ClientSize()         // 最近一次传入的控件尺寸
eng.Pick(px, py, w, h)   // 屏幕射线拾取（内部渲染坐标）
eng.PickClient(px, py)   // 同上，但直接传控件像素坐标（自动按 KeepAspect 比例映射）
```

> `eng.KeepAspect = true`（默认）会按控件宽高比调整内部渲染高度，避免画面拉伸。

### 2.7 输入

```go
in := eng.Input
in.KeyDown("w")          // 按住
in.KeyPressed("space")   // 本帧刚按下
in.Axis("a", "d")        // -1 / 0 / 1
in.MouseX, in.MouseY
in.MouseDX, in.MouseDY   // 本帧位移
in.Wheel / in.Buttons
in.EndFrame()            // 帧末清一次性状态（引擎 Tick 默认自动调用）
```

键名：小写字母、`"arrowup"` / `"space"` / `"enter"` / `"shift"` / `"ctrl"` / `"f1"` / `"num0"` 等，
由 `ui.GameKeyName(vk)` 从虚拟键码映射（`ui.KeyName` 是展示用的大写名，两者不同）。

### 2.8 2D 叠加（HUD）

```go
batch := game.NewSpriteBatch()
r.SetSprite2D(batch)

batch.Add(game.Sprite{Pos: game.V2(10, 10), Size: game.V2(32, 32), Texture: uiTex})
// 瓦片地图
tm := game.NewTilemap(20, 15, 16, 16, game.NewAtlas(atlasTex, 16, 16, 8))
tm.Set(3, 4, 2)
r.SetSprite2D(tm)
```

文字不由本包绘制，请用 `ui.Text` 叠在 `Viewport3D` 之上（同窗口内更清晰、可交互）。

### 2.9 碰撞与拾取

```go
game.RayAABB(origin, dir, box)          // (t, hit)
game.RaySphere(origin, dir, sphere)
game.RayTriangle(o, d, a, b, c)
game.RayMesh(o, d, mesh, world)         // (t, triIndex, hit)
game.SphereAABB(sphere, box)            // (推出向量, hit)
box.Overlaps(other) / box.Union(other) / box.Contains(p) / box.Expanded(0.5)
```

### 2.10 动画

```go
an := game.NewAnimator()
an.MoveTo(node, game.V3(3, 0, 0), 1.5)   // EaseInOut
an.ScaleTo(node, game.V3(2, 2, 2), 0.5)
an.Spin(node, game.V3(0, 1, 0), 1.2)    // 持续自转
an.Update(dt)
an.Stop(node) / an.Clear()
```

---

## 3. ui 侧接入

```go
ui.Viewport3D(&ui.Viewport3DOpts{
	Src:        eng,                 // 实现 ui.ViewportSource
	Resolution: [2]int{640, 360},    // 未显式设尺寸时的默认尺寸
	AutoFocus:  true,                // 点击后自动取得键盘焦点
	OnKey:      eng.HandleKey,       // func(key string, down bool)
	OnMouse:    eng.HandleMouse,     // func(x, y float32, button int, down bool)，button=-1 仅移动
	OnWheel:    eng.HandleWheel,     // func(dy float32)
}).Grow(1)
```

接入要点：

- 控件是自绘控件，会每帧 `markAnim()` 持续重绘（游戏画面不会停）；
- 纹理按尺寸复用，只做 `UpdateTextureRGBA`，不每帧新建；
- 键盘需控件获得焦点（`AutoFocus` 或点击）；鼠标抬起通过 `OnDragEnd` 送达；
- 任何实现 `RenderViewport(w, h int, dt float32) (pix []byte, tw, th int)` 的对象都能当 `Src`，
  不一定要用 `game.Engine`。
- 多视口用 `Key` 区分状态与纹理：`ui.Viewport3D(&ui.Viewport3DOpts{Src: e2, Key: "minimap"})`；
  切换引擎/重开窗口时调 `ui.ResetViewport3D("minimap")` 释放纹理。

---

## 4. 性能建议

| 项 | 建议 |
| --- | --- |
| 内部渲染分辨率 | 480×270 ~ 960×540；越高越贵（像素数线性） |
| 抗锯齿 | 需要时用 `Supersample = 2`（≈3× 开销）；否则保持 1 |
| 动态分辨率 | 依 `Stats().Ms` 调 `eng.Resize`（demo 内实现可参考） |
| 并行 | `Renderer.Threads = 0`（默认 GOMAXPROCS）按屏幕行分带，无锁 |
| 三角形数 | 640×360 下 ≤ 3k 可稳定 50+ FPS（参考基准 `BenchmarkFrame`） |
| 光照 | Phong 逐像素最贵；大量远处物体改用 `ShadeFlat` 或 `ShadeUnlit` |
| 纹理 | 尺寸不变时不重建纹理；小图用 `Nearest` 更快 |
| 剔除 | 物体级视锥剔除自动生效；把不动的大块几何合成单个网格 |
| 观测 | `Renderer.Stats()`：`Ms` / `Triangles` / `Pixels` / `Culled` |

基准（开发机，640×360，约 1.5k 三角 + 2 光源）：`go test ./game/test -bench BenchmarkFrame -benchtime 3s`。

---

## 5. 已知限制

- 无 PBR、SSAO、实时反射，也不支持自定义着色器（后处理是固定链）；
- 阴影仅单个方向光 shadow map（点光不投影，无软阴影级联）；
- 半透明按三角形深度排序（近似 painter），大面积相互穿插时仍可能出错；
- 骨骼蒙皮是 CPU 每帧重算顶点，适合低模角色（1000 顶点 / 4 骨约 0.11 ms），高模靠 LOD；
- 音频实际出声：Windows（winmm）、Linux（OSS，需 `/dev/dsp`）、浏览器（WebAudio）；
  macOS / iOS / Android 仍返回静音播放器（`Status().OK == false`），见 11.2；
- 内置位图字体不含中文；
- 软件光栅化在超大分辨率 / 大场景下性能有限，不适合重度 3D；
- 联机只有 `guis/netplay` 的局域网直连（无中继 / 匹配 / 回滚网络），见 `64-netplay.md`；
- 硬件加速后端未提供（`internal/gpu` 的 GL 函数表在 win32/wasm/metal 后端尚未绑全）。

---

## 6. API 索引

- 数学：`Vec2` `Vec3` `Vec4` `Quat` `Mat4` `Identity` `Translate` `Scale` `Rotate` `RotateEuler` `Perspective` `Ortho` `LookAt` `TRS` `Clamp` `Lerp` `SmoothStep` `Radians` `Degrees`
- 颜色/纹理：`RGBA` `Color` `RGB` `Texture` `NewTexture` `CheckerTexture` `LoadTexture` `LoadTextureBytes` `TextureFromImage`
- 网格：`Mesh` `NewMesh` `NewBox` `NewSphere` `NewPlane` `NewCylinder` `NewTorus` `LoadOBJ` `LoadOBJBytes` `AABB`
- 场景：`Node` `NewNode` `NewMeshNode` `Scene` `NewScene` `Light` `SunLight` `PointLight` `Fog`
- 相机：`Camera` `NewCamera` `Orbit` `NewOrbit` `Fly` `NewFly` `Plane` `Frustum`
- 渲染：`Renderer` `NewRenderer` `FrameStats` `Sprite2D`
- 引擎：`Engine` `World` `NewEngine` `Game` `Loop` `NewLoop`
- 输入：`Input` `NewInput` `Touch` `TouchPhase`
- 2D：`Sprite` `SpriteBatch` `Atlas` `Tilemap` `Rect2` `DrawSprite`
- 碰撞：`RayAABB` `RaySphere` `RayTriangle` `RayMesh` `SphereAABB` `Sphere`
- 动画：`Animator` `NewAnimator` `EaseLinear` `EaseIn` `EaseOut` `EaseInOut`
- 音频后端与总线：`Mixer` `NewMixer` `OpenPlayer` `PlayOpt` `VoiceID` `PlayerStatus` `StatusReporter` `BusControllable` `MusicFader` `Mixer.SetBusVolume` `Mixer.BusVolume` `Mixer.FadeBus` `Mixer.FadeMusic` `Mixer.PlayMusicStream` `Mixer.StopMusicStream` `Mixer.ActiveMusic` `MusicStream` `OpenMusicStream` `MusicStreamFromBytes`
- 空间音频：`Listener` `SpatialGain` `Mixer.SetListener` `Mixer.ListenerInfo` `Mixer.SpatialParams` `Mixer.PlayAt`
- BVH / LOD：`BVH` `NewBVH` `BVH.Ray` `BVH.TriCount` `BVH.NodeCount` `Mesh.BVH` `Mesh.InvalidateBVH` `LODLevel` `Node.PickLOD` `FrameStats.LODSwitches`
- 骨骼蒙皮：`Skeleton` `Bone` `Bone.LocalMatrix` `Skeleton.Validate` `SkinVertex` `SkinnedMesh` `NewSkinnedMesh` `NewSkinnedMeshFromOBJ` `SingleBoneSkeleton` `SkinnedMesh.SkinnedVertices` `SkinnedMesh.RenderMesh` `SkinnedMesh.BindPose` `SkinnedMesh.BindSingleBone` `SkinClip` `BoneTrack` `KeyFrame` `BonePose` `SkinAnimator` `NewSkinAnimator` `LoadSkin` `LoadSkinBytes` `SaveSkin` `MarshalSkin`
- 游戏系统（第 10 章）：`NavGrid` `NewNavGrid` `NavGridFromTiled` `NavGridFromTileLayer` `CellsToWorld` `SmoothPath` `StateMachine` `NewStateMachine` `Transition` `OBB` `NewOBB` `Capsule` `AABBFromMesh` `AABBOBB` `Billboard` `BillboardSet` `NewBillboardSet` `SaveStore`
- 联机：`guis/netplay`（`Listen` `Dial` `Peer` `Server` `Client` `Transport` `Interpolator` `Lockstep`）详见 `64-netplay.md`
- 音效：`Audio` `NopAudio`
- ui：`Viewport3D` `Viewport3DOpts` `ViewportSource` `GameKeyName` `GameView` `GameViewOpts`
- 3D 效果：`Shadow` `DefaultShadow` `ShadowMap` `NewShadowMap` `PostFX` `NewPostFX` `Sky` `SkyMode` `GradientSky` `CubeSky` `Texture.BuildMips` `Texture.SampleTrilinear` `Texture.TexelLod` `Material.Mipmap` `Material.MipBias`
- 2D 增强：`Camera2D.Update` `Camera2D.Shake` `Camera2D.ZoomTo` `Camera2D.Screen` `Camera2D.World` `Camera2D.Apply` `Camera2D.ApplyBatch` `Camera2D.VisibleRect` `Camera2D.VisibleBounds`
- 2D 基础设施：`NineSlice` `NineSliceTexture` `BitmapFont` `BuiltinFont` `NewBitmapFont` `Parallax` `ParallaxLayer` `StarfieldTexture` `TileCollision` `NewTileCollision` `ResolveAABB` `FillRect`
- 易用性：`SceneManager` `NewSceneManager` `Level` `Pool` `NewPool`

---

## 7. 3D 效果包（阴影 / 后处理 / 天空 / Mipmap）

### 7.1 实时阴影（平行光 shadow map + PCF）

软件 shadow map：按方向光生成一张正交深度图，片元在光空间比较深度得到受光系数。

```go
sh := game.NewShadowMap()
sh.On = true
sh.Size = 512            // 阴影图分辨率（256~1024）
sh.PCF = 3               // 3×3 软阴影；1 = 硬阴影，5 = 更软
sh.Strength = 0.7         // 阴影浓度
sh.Bias = 0.0025          // 自阴影发花时调大
sh.Once = true            // 静态场景只构建一次
eng.Renderer.Shadow = sh
```

- 只对**方向光**（`Scene.AddSun`）生效；点光不投影阴影。
- 场景变化（物体动画）后调用 `sh.Reset()` 触发重建（`Once` 模式下必须）。
- 观测：`sh.Valid()` / `sh.Triangles()` / `sh.Depth()`。
- 成本：每帧一次全场景深度光栅（低分辨率）+ 受光片元 1~9 次采样；256~512 足够。

### 7.2 前处理与后处理链（PostFX）

```go
eng.Renderer.Post = &game.PostFX{On: true, Vignette: 0.35, Saturation: 1.1, Contrast: 1.05}
```

| 字段 | 作用 |
| --- | --- |
| `Brightness` / `Contrast` / `Gamma` | 亮度 / 对比 / 伽马（查表，最便宜） |
| `Saturation` / `Grayscale` | 饱和度 / 灰度混合 |
| `Tint` / `Flash` | 乘色染色 / 闪白闪红（受击反馈） |
| `Vignette` / `Scanlines` / `Noise` / `Posterize` | 暗角 / 扫描线 / 噪点 / 色阶量化（复古风） |
| `Bloom*` | 泛光（降采样亮部 + 两趟盒式模糊 + 加回） |

配合道具 / 关卡可直接调值做渐进效果：受击时 `post.Flash = game.Color(220, 40, 40, 140)`，之后每帧把 A 减下去即可。

### 7.3 天空盒 / 渐变天空

```go
// 三色渐变（最便宜，适合卡通 / 像素风）
sc.Sky = game.GradientSky(game.RGB(40, 80, 200), game.RGB(180, 210, 240), game.RGB(20, 20, 30))

// 立方体天空盒：4×3 交叉展开图（单面边长 = 图宽/4）
sc.Sky = game.CubeSky(tex, 0)
sc.Sky.Rotation = game.Radians(30) // 调整朝向
```

逐像素按视线方向着色，抬头 / 转头 / 正交相机都正确；只写背景像素，不遮挡场景。

### 7.4 纹理 mipmap 三线性过滤

```go
tex.BuildMips()                    // 构建 mipmap 链（可重复调用）
mat.Mipmap = true                  // 启用三线性采样
mat.MipBias = -0.5                 // 负值更锐利，正值更模糊
tex.FreeMips()                     // 改写像素后释放
```

mip 层级由光栅器按屏幕空间 UV 导数自动选择（`Texture.TexelLod`），远距离地面 / 大面积纹理不再闪烁，代价是采样成本略增。

---

## 8. 2D 基础设施

### 8.1 2D 摄像机（增强）

```go
cam := game.NewCamera2D(640, 360)
cam.Follow(playerX, playerY, 0.2)   // 平滑跟随（原有）
cam.Shake(6, 0.25)                  // 受击抖屏
cam.ZoomTo(1.35, 0.4)               // 平滑推近
cam.Rotation = game.Radians(3)      // 轻微旋转镜头
cam.Update(dt)                      // 每帧推进抖屏 / 缩放过渡

x, y := cam.Screen(wx, wy)          // 世界 → 屏幕（含缩放/旋转/抖屏）
wx, wy := cam.World(sx, sy)         // 屏幕 → 世界
for i := range actors {
	batch.Add(cam.Apply(actors[i]))  // 世界精灵 → 屏幕精灵
}
cam.ApplyBatch(batch, sprites)      // 批量转换 + 视口剔除
```

`VisibleRect` / `VisibleBounds` 用于对象剔除与刷怪范围；`SetBounds` 限制镜头范围。

### 8.2 九宫格（面板 / 按钮 / 对话框）

```go
tex := game.NineSliceTexture(24, game.RGB(40, 52, 78), game.RGB(150, 190, 240), 6)
game.NineSlice{Texture: tex, Dst: game.Rect2{X: 20, Y: 20, W: 320, H: 120}, Border: game.Rect2{8, 8, 8, 8}}.Draw(img)
```

四角不拉伸，四边单向拉伸，中区双向拉伸；目标小于边框总和时自动收缩，不破图。

### 8.3 位图字体（HUD / 分数 / 对话）

```go
f := game.BuiltinFont()             // 内置 5×7 像素字体，零外部资源
f.DrawText(img, "SCORE 1200", 8, 8, 2, game.RGB(255, 255, 255))
f.DrawTextShadow(img, "PAUSED", 100, 60, 3, game.RGB(255, 230, 80), game.RGB(0, 0, 0))
w, h := f.Measure("MULTI\nLINE", 2)

// 自制字体：图集网格（白字 + 透明底，颜色由 color 乘入）
myFont := game.NewBitmapFont(atlasTex, 8, 12, 16, 32, 127)
```

内置字体支持 `A-Z 0-9` 与常用标点，小写自动映射到大写，中文需自制图集。

### 8.4 视差背景

```go
bg := &game.Parallax{}
bg.Add(game.ParallaxLayer{Texture: game.StarfieldTexture(256, 120, game.RGB(5, 8, 20)), Factor: game.V2(0.1, 0)})
bg.Add(game.ParallaxLayer{Texture: mountainTex, Factor: game.V2(0.4, 0)})
bg.Draw(img, cam)                   // 在游戏精灵之前调用
```

`Factor` 为滚动系数（0 = 远景不动，1 = 与世界同速），自动平铺至铺满视口。

### 8.5 瓦片碰撞与单向平台

```go
col := game.NewTileCollision(32, 32)
col.MarkSolid(1, 2, 3).MarkOneWay(9)
solid, oneWay := col.Collect(tilemap, "collision")
body.Step(dt, solid, oneWay)        // PlatformerBody 已内置单向平台判定
```

实心瓦片默认按行合并成长条，盒体数量少；`ResolveAABB` 用于道具 / 非角色实体的简单推出解算。

---

### 8.6 完整 Tiled 地图（多图块集 / 对象层 / 动画瓦片）

`game.TiledMapFull` 支持 Tiled 1.x 导出的 `.tmj/.json` 主体特性：多图块集
（含外部 `.tsx`）、对象层（矩形/点/折线/多边形）、动画瓦片、每层 offset/opacity/parallax、
图像层、组层递归、图块与对象自定义属性、GID 翻转位。

```go
tm, err := game.LoadTiledMap("levels/stage1.tmj")
if err != nil {
	log.Fatal(err)
}
tm.Camera = game.V2(camX, camY) // 视差：parallax<1 的图层随相机少动
tm.Update(dt)                  // 推进动画瓦片

// 逻辑查询
for _, o := range tm.Objects("ents") {
	if o.Type == "spawn" {
		player.Pos = game.V3(o.X, 0, o.Y) // Tiled 坐标：像素、Y 向下
	}
}
gid := tm.Tile("ground", 3, 2)   // 取某格 GID
ts, id := tm.TilesetFor(gid)     // 自动处理翻转位与多图块集
```

`TiledMapFull` 实现 `Sprite2D`，直接 `Renderer.SetSprite2D(tm, ...)` 或由 `Engine` 叠加绘制。
旧的 `TiledMap` + `LoadTiledJSON` 行为不变（只解析图块层 + 首个图块集）。

### 8.7 图集打包（散图 → 图集）

```go
res, err := game.PackAtlas([]game.PackImage{
	{Name: "idle_0", Image: img0},
	{Name: "idle_1", Image: img1},
}, game.PackOptions{Padding: 2, MaxSize: 2048, PowerOfTwo: true})
// res.Image 是 *image.RGBA；res.Rects 是每张图的 {Name,X,Y,W,H}；res.Used 是利用率
```

命令行工具（打包 + 坐标表 + 可选 Go 常量）：

```bash
go run ./tools/atlaspack -in assets/sprites -out assets/atlas.png -json assets/atlas.json
go run ./tools/atlaspack -in assets/sprites -go assets/atlas_gen.go -pkg assets
```

运行时回读：

```go
am, err := game.LoadAtlasMapFile("assets/atlas.json") // 自动找同目录 atlas.png
if s, ok := am.Sprite("idle_1", x, y, 0, 0); ok {     // w/h 为 0 时用帧原始尺寸
	batch.Add(s)
}
```

与按固定格子切分的 `Atlas` 互补：`AtlasMap` 每帧尺寸任意，适合角色/UI/特效混合图集。

### 8.8 精灵锚点与 2D 层级

`Sprite.Origin`（0~1，相对 Size）+ `Sprite.UseOrigin` 控制旋转/定位锚点；
默认（`UseOrigin=false`）锚点为几何中心，与旧版逐像素一致。

```go
// 以精灵底部中心为锚点（重力平台游戏常用）
sp := game.Sprite{Pos: game.V2(x, y), Size: game.V2(24, 32), Texture: tex,
	UseOrigin: true, Origin: game.V2(0.5, 1), Rot: angle}
```

`Node2D` 把角色、手持武器、坐骑、血条组织成树，再一次展开成世界坐标精灵：

```go
body := game.NewNode2D("body")
body.Pos = game.V2(120, 200)
body.Sprite = &game.Sprite{Size: game.V2(32, 32), Texture: heroTex}

sword := game.NewNode2D("sword")
sword.Pos = game.V2(14, 0)
sword.Sprite = &game.Sprite{Size: game.V2(16, 8), Texture: swordTex}
sword.Alpha = 0.8 // 透明度沿子树累乘
body.Add(sword)

batch.AddNode2D(body) // 递归展开，父旋转/缩放/透明度自动继承
```

`Node2D` 也提供 `WorldPos / WorldRot / WorldScale / WorldAlpha / Traverse / Find` 查询。

### 8.9 Y 排序（前后遮挡）

```go
batch.SortY() // 按 Layer → Z → 脚底 Y 稳定排序（小者先画 = 更远）
```

- `Sprite.Layer`（整数层）、`Sprite.Z`（层内深度）优先于脚底 Y。
- 脚底 Y 由 `Sprite.FeetY()` 计算：`UseOrigin` 时按 `Origin.Y`，否则取矩形底边。
- 也可以用 `batch.SortFunc(less)` 自定义排序。

## 9. 开箱即用

### 9.1 ui.GameView（一体化控件）

```go
eng := game.NewEngine(640, 360)
eng.SetGame(myGame{})
ui.Column(
	ui.GameView(&ui.GameViewOpts{
		Engine:      eng,
		ShowStats:   true,        // 左上角 FPS / 三角形 / 像素
		IdleTimeout: 3,           // 空闲自动暂停（切窗口省 CPU）
		OnFrame:     func(e *game.Engine, dt float32) { hud.Update(dt) },
	}).Grow(1),
)
```

自动把键盘 / 鼠标 / 滚轮 / 触摸接到引擎，拥有焦点时自动取焦，2D / 3D 通用。

### 9.2 关卡 / 场景管理

```go
mgr := game.NewSceneManager()
mgr.Register("menu", buildMenu, nil, nil)
mgr.Register("level1", buildLevel1, nil, teardownLevel1)
mgr.FadeTo("level1", eng)          // 淡出 → 清场景 → 重建 → 淡入
// Update：mgr.Update(dt, eng)
// Draw 之后：mgr.Draw(img)
```

`Switch` 为立即切换；`FadeTime = 0` 时 `FadeTo` 退化为 `Switch`。

### 9.3 对象池

```go
bullets := game.NewPool(func() *Bullet { return &Bullet{} }, func(b *Bullet) { *b = Bullet{} })
bullets.Prewarm(64)
b := bullets.Get(); b.Pos = start; bullets.Put(b)
```

用于子弹 / 粒子 / 伤害数字，避免热路径频繁分配造成的 GC 抖动。


## 10. 游戏系统扩展（寻路 / 状态机 / 音频 / 存档 / 触屏 / 碰撞 / 广告牌）

> 本节 API 全部属于 `guis/game`（触屏控件除外），不依赖 GPU 与 cgo，六端行为一致。

### 10.1 A* 寻路

`game/nav.go` 提供网格 A*（四向 / 八向、可配阻止移动函数）：

```go
g := game.NewNavGrid(w, h)                     // w×h 格
g.SetBlocked(x, y, true)                       // 标记不可走
path, ok := g.FindPath(game.V2i(0, 0), game.V2i(9, 9))
```

### 10.2 有限状态机

`game/fsm.go` 是带进入 / 更新 / 退出钩子的极简 FSM，适合角色状态、UI 流程：

```go
fsm := game.NewFSM()
fsm.Add("idle", game.State{Enter: func() {}, Update: func(dt float32) {}, Exit: func() {}})
fsm.Set("idle")
fsm.Update(dt)
```

### 10.3 音频：多格式解码 + 3D 空间化 + 流式音乐

```go
snd, err := game.DecodeSoundFile("hit.ogg")    // 支持 .wav / .mp3 / .ogg（按文件头嗅探）
snd, err = game.DecodeSound(b)                 // 内存字节
snd, err = game.LoadWAV("hit.wav")             // 旧接口保持不变
```

3D 空间化（声像 + 距离衰减，监听者一般跟相机同步）：

```go
m := game.NewMixer(48000)
m.SetListener(game.Listener{
    Pos: cam.Pos, Forward: cam.Forward(), Up: game.Vec3{Y: 1},
    MinDist: 1, MaxDist: 30,
})
m.PlayAt(snd, enemy.Pos, game.PlayOpt{Volume: 1})  // 自动算 pan / 衰减；超出 MaxDist 返回 0
pan, gain := m.SpatialParams(pos)                   // 需要自己控制时单独取
```

长音乐用流式（分块解码，不整曲驻留内存）：

```go
ms, err := game.OpenMusicStream("bgm.mp3")   // mp3 / ogg 真流式，wav 整读
defer ms.Close()
n, err := ms.Read(buf)                        // buf []int16，交错样本
ms.SeekFrames(frame)
```

### 10.4 存档：版本迁移 / CRC 校验 / 备份槽 / 混淆

`game.SaveStore` 默认仍写纯 JSON（旧存档完全兼容）；打开开关后自动升级格式：

```go
s := game.SaveStore{
    Dir:        "saves",
    Version:    3,                                  // >0 时写版本头
    Checksum:   true,                               // 写 CRC32，坏了自动回退 .bak
    Migrate:    func(from int, d map[string]any) (map[string]any, error) {
        if from < 2 { d["gold"] = 0 }                // 逐版本升级
        return d, nil
    },
    Obfuscate:  true,                               // 异或混淆（防手改，不是加密）
    Key:        []byte("my-key"),
    KeepBackup: true,
}
s.Put("slot1", save)
err := s.Get("slot1", &out)                          // 自动迁移 + 校验 + 回退
names, _ := s.List()
s.RestoreBackup("slot1")
```

注意：`Obfuscate` 打开后不兼容之前的明文存档（需要自己迁移）。

### 10.5 虚拟摇杆与触屏按钮（ui 层）

```go
ui.Stack(
    ui.GameView(&ui.GameViewOpts{Engine: eng}),
    ui.VirtualStick(&ui.VirtualStickOpts{Key: "move", Size: 140, Deadzone: 0.15}),
    ui.VirtualButton(&ui.VirtualButtonOpts{Key: "jump", Size: 72, Label: "跳",
        OnPress: func(down bool) { eng.Input.Keys["space"] = down }}),
)
x, y := ui.VirtualStickAxis("move")   // -1~1，已做死区与圆形裁切
down := ui.VirtualButtonDown("jump")
```

- 走的是节点新增的 `Node.OnTouch(x, y, phase, id)`：命中即消费，不会把事件漏给下层视口；
- 多指隔离：摇杆只跟踪第一个按下的手指，其余手指的移动不影响轴值；
- 声明式视图每帧重建节点，控件状态按 `Key` 存在内部注册表里，跨帧保持。

### 10.6 指针锁定（FPS 转视角）

```go
ui.GameView(&ui.GameViewOpts{Engine: eng, MouseLock: true})
// 或引擎层手动控制：
eng.SetPointerLock(true)        // 绝对坐标不再参与差分，只认 HandleMouseMove
eng.HandleMouseMove(dx, dy)     // 原始位移（ui 层已自动转发 Event.DX/DY）
```

锁定后 `Input.MouseDX / MouseDY` 只来自相对位移，光标撞到屏幕边缘也不会卡住（建议全屏）。
`ui.Viewport3D` 的 `Viewport3DOpts.MouseLock` 是同一开关的底层入口。

### 10.7 碰撞与射线：OBB / 胶囊 / 三角形 / 网格 / 台阶

```go
t, ok := game.RayTriangle(origin, dir, a, b, c)        // Möller–Trumbore
t, tri, hit := game.RayMesh(origin, dir, mesh, model)  // 逐三角形，返回最近命中

box := game.NewOBB(center, half, [3]game.Vec3{ax, ay, az})
box.Contains(p); box.ClosestPoint(p); t, ok = box.Ray(origin, dir)
aabbBox := game.AABBOBB(bounds)                        // AABB → OBB

cap := game.Capsule{A: feet, B: head, Radius: 0.35}
cap.Contains(p); cap.ClosestPoint(p); t, ok = cap.Ray(origin, dir)

// 带台阶的角色移动（ground 返回该水平位置的支撑面高度）
moved := body.StepMove(game.Vec3{X: vx * dt, Z: vz * dt}, ground, 0.35)
```

### 10.8 世界空间广告牌（血条 / 伤害数字 / 图标）

```go
sc.Billboards = game.NewBillboardSet()          // 场景上挂一个集合
sc.Billboards.Add(game.Billboard{
    Pos: enemy.Pos, Offset: game.Vec3{Y: 2},
    Kind: game.BillboardBar, Width: 1.2, Height: 0.12,
    Fill: hp / maxHP, Color: game.Color(255, 60, 60, 255),
    BackColor: game.Color(0, 0, 0, 160), DepthTest: true,
})
sc.Billboards.Add(game.Billboard{Pos: p, Kind: game.BillboardText, Text: "-42",
    Color: game.Color(255, 230, 80, 255), Scale: 0})
```

- 在 3D 之后、Post 与 2D 精灵之前叠加；
- `DepthTest` 读光栅器的 invW 缓冲，所以血条会被墙挡住；
- `MaxDist > 0` 时超距不画；`FrameStats.Billboards` 给出本帧实际绘制数。

### 10.9 材质 AlphaMode（不透明 / 镂空 / 混合）

```go
mat.AlphaMode = game.AlphaBlend    // 默认：半透明，进透明通道按深度从远到近排
mat.AlphaMode = game.AlphaOpaque   // 忽略 alpha 强制不透明（写深度）
mat.AlphaMode = game.AlphaCutout   // 镂空：低于 AlphaCutoff 的像素丢弃，其余写深度
mat.AlphaCutoff = 0.5
```

镂空适合树叶、栅栏、草丛这类「要么全透要么全不透」的贴图，能正常写深度、不乱序。
透明三角形现在按「最远顶点」排序（painter 算法），大面积半透明面片互相穿插时更稳定。

## 11. 高级能力（音频后端 / BVH / LOD / 骨骼蒙皮 / 联机）

### 11.1 音量总线与流式音乐

```go
m := game.NewMixer(48000)
m.SetBusVolume("sfx", 0.8)     // 总线：sfx / music / voice，未登记按 1
m.Play(game.Tone(880, 0.1, 48000), game.PlayOpt{Bus: "sfx", Volume: 1})

ms, _ := game.OpenMusicStream("bgm.ogg")   // 文件 / 内存流均支持
m.PlayMusicStream(ms, game.PlayOpt{Loop: true, Bus: "music"})
m.FadeMusic(0, 1.5)            // 1.5 秒内把音乐总线淡到 0
m.StopMusicStream(0)           // 0 = 停全部
```

总线音量在混音阶段按帧加权（不是播放后调音量），渐变由混音帧推进，不需要额外协程；
流式音乐独立于一次性音效，不会挤占音效声部。

### 11.2 音频后端与状态自述

| 平台 | 后端 | 说明 |
|---|---|---|
| Windows | winmm（waveOut 双缓冲） | 默认可用 |
| Linux | OSS（`/dev/dsp`） | 现代发行版默认没有该设备 → 静音回退 |
| 浏览器（js/wasm） | WebAudio（AudioContext + ScriptProcessor） | 可用 |
| macOS / iOS / Android | 无（NopPlayer） | 需要 CoreAudio / AVAudioEngine / AudioTrack 接入 |

```go
p := game.OpenPlayer()
if sr, ok := p.(game.StatusReporter); ok {
	st := sr.Status()   // {Backend:"winmm", Rate:48000, OK:true}
	_ = st
}
if bc, ok := p.(game.BusControllable); ok { bc.SetBusVolume("music", 0.5) }
```

可选接口（`BusControllable` / `MusicFader` / `StatusReporter`）用类型断言探测，老代码只依赖 `Player` 也能编译。「为什么没声音」优先看 `Status().OK` 与 `Backend`。

### 11.3 BVH 加速射线拾取

```go
bvh := game.NewBVH(mesh)                    // 也可 mesh.BVH() 惰性缓存
t, tri, ok := bvh.Ray(origin, dir, model)   // 与 RayMesh 结果完全一致
mesh.InvalidateBVH()                        // 改了 Positions / Indices 后必须调
```

`Engine.PickMesh` 已自动走 BVH，业务代码不用改。实测 5000 三角平面网格：逐三角形 91 µs → BVH 2.55 µs（约 35×）。注意 BVH 走「进入距离」剪枝（起点在盒内返回 0），不能直接复用 `RayAABB`（它返回出口距离，会把眼前的子树误剪）。

### 11.4 LOD（距离层级）

```go
n := game.NewMeshNode("tree", hiMesh, mat)
n.LODs = []game.LODLevel{{Distance: 20, Mesh: hiMesh}, {Distance: 100, Mesh: loMesh}}
// 相机距离 ≤ 20 用 hiMesh，≤ 100 用 loMesh，再远沿用最后一级；n.Mesh 是兜底
```

`FrameStats.LODSwitches` 记录本帧使用非默认层级的次数，可用于验证 LOD 是否生效。

### 11.5 骨骼蒙皮动画（CPU）

```go
sk := &game.Skeleton{Bones: []game.Bone{
	{Name: "root", Parent: -1, Scl: game.Vec3{X: 1, Y: 1, Z: 1}, InverseBind: game.Identity()},
	{Name: "arm", Parent: 0, Pos: game.Vec3{Y: 1}, Scl: game.Vec3{X: 1, Y: 1, Z: 1}, InverseBind: game.Identity()},
}}
sm := game.NewSkinnedMesh(mesh, sk, nil)  // skin 为 nil 时自动全部绑到 0 号骨
anim := game.NewSkinAnimator(sm)
anim.Play(clip, true, 1)                  // clip / loop / speed
anim.CrossFade(walkClip, 0.3)             // 0.3 秒交叉淡入
anim.Update(dt)                           // 每帧调用
node.Skinned = sm                          // 接进场景即可参与渲染
node.SkinAnim = anim
```

约束与要点：

- **骨骼必须拓扑有序**（父骨索引 < 自身索引），`Skeleton.Validate()` 可校验；
- 每顶点最多 4 骨权重，权重之和不为 1 时自动归一化；
- 法线按「主导骨」（权重最大那根）变换，比逐骨加权便宜，低模视觉可接受；
- `RenderMesh` 复用同一块顶点缓冲，每帧只写不分配；1000 顶点 / 4 骨约 0.11 ms；
- 静态 OBJ 可以用 `NewSkinnedMeshFromOBJ` 套上单骨骨架，随便一根骨整体变换。

注意命名：2D 精灵动画用 `game.AnimClip` / `game.Animator`，骨骼用 `game.SkinClip` / `game.SkinAnimator`，两者互不影响。

### 11.6 `.skin` 文件格式（JSON）

一个文件同时装下网格（positions / indices）、骨架、每顶点权重与一段动画：

```go
sm, clip, err := game.LoadSkin("hero.skin")
if err := game.SaveSkin("hero.skin", sm, clip); err != nil { /* ... */ }
b, err := game.MarshalSkin(sm, clip)   // 内嵌资源用
```

加载时会校验骨骼拓扑与父索引，非法直接报错（不会带病运行）。

### 11.7 netplay 联机框架（`guis/netplay`）

```go
// 服务端
srv, _ := netplay.Listen("0.0.0.0:9000")
srv.OnJoin = func(p *netplay.Peer) { p.Send(netplay.MsgChat, []byte("welcome")) }
srv.Broadcast(netplay.MsgSnapshot, payload)

// 客户端
cli, _ := netplay.Dial("127.0.0.1:9000", "player1", 3*time.Second)
cli.Peer().On(netplay.MsgSnapshot, func(p *netplay.Peer, m netplay.Message) {
	var s netplay.Snapshot
	_ = netplay.DecodeJSON(m.Payload, &s)
})
```

- 传输：`Transport` 抽象，`NewTCPTransport`（4 字节长度前缀分帧，单帧上限 1 MiB）与 `NewUDPTransport`（一个包一条消息，丢了就丢，适合帧同步输入）。
- 协议：帧头 `magic(2) | version(1) | type(1) | seq(4) | len(4)`，版本不匹配直接拒绝。
- 握手：客户端发 `MsgHandshake`，服务端回 `MsgHandshakeAck`（分配玩家 ID）。
- 心跳与超时：`Peer.Heartbeat` 定时 Ping，`Peer.Timeout` 超时未收到任何消息即断开并回调。
- 状态同步：服务端按 tick 广播 `Snapshot`，客户端 `Interpolator.Push` 后用 `Sample(now)` 拿到相邻两帧与 alpha 自行插值。
- 帧同步：`Lockstep` 等所有玩家提交输入后 `Advance()` 统一放行，按玩家 ID 排序归并。

定位是**局域网直连**：不做中继、匹配、账号、反作弊与回滚网络。

### 11.8 性能基线

测试机 AMD Ryzen 5 3600，Go 1.26，单进程：

| 基准 | 结果 |
|---|---|
| `BenchmarkFrame`（640×360） | 4.1 ms/帧（≈240 FPS，多次跑 3.8～4.5 ms） |
| `BenchmarkFrameSS2` | 14.95 ms/帧（≈67 FPS） |
| `BenchmarkBVHRay`（5000 三角） | 2.55 µs |
| `BenchmarkRayMeshLinear`（同网格） | 91.2 µs |
| `BenchmarkSkin`（1000 顶点 / 4 骨） | 110 µs |

---

> 联机（握手 / 广播 / 快照插值 / 帧同步）的完整 API 与常见坑见 [`64-netplay.md`](64-netplay.md)。