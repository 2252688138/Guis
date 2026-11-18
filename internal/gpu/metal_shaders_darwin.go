//go:build darwin && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package gpu

// Metal 版着色器（MSL）：与 shaders.go 中的 GLSL 330 一一对应。
//
// 之所以“重写”而不是翻译 GLSL：MSL 的分量语义（纹理 v 轴、采样器绑定、
// 顶点属性必须由 MTLVertexDescriptor 描述）与 GL 差异较大，手写一份更容易
// 保证与 GL 路径的同像素级一致。
//
// 坐标约定：CPU 侧顶点逻辑与 GL 路径完全相同（屏幕 y 向下，
// gl_Position.y 取反 → NDC y 向上），因此顶点变换公式与 GLSL 保持一致。
// 纹理侧：本后端把「离屏目标」统一按 GL 语义（第 0 行 = 屏幕底部）暴露给
// 渲染器，所有采样离屏目标的着色器都做 v = 1 - v 的翻转（见 f_blur / f_resolve /
// f_copy），普通图集/图片纹理不翻转。
// metalKindFromSource 按 GLSL 源码特征识别程序种类（渲染器的 6 个着色器固定）。
func metalKindFromSource(src string) (int, bool) {
	switch {
	case containsASCII(src, "uSpan"):
		return pipeResolve, true
	case containsASCII(src, "uRadius") && containsASCII(src, "uTexel"):
		return pipeBlur, true
	case containsASCII(src, "vColor2"):
		return pipeShape, true
	case containsASCII(src, "uAtlas"):
		return pipeText, true
	case containsASCII(src, "vRadius") && containsASCII(src, "uTex"):
		return pipeImage, true
	}
	return 0, false
}

const metalShaderSource = `
#include <metal_stdlib>
using namespace metal;

// 通用 uniform：vp = 逻辑视口尺寸；u = 各着色器共用的小参数区
// （text: u.x=gamma；blur: u.xy=texel、u.z=radius；resolve: u.xy=texel、
//  u.z=span、u.w=sharpen；copy/solid: 不使用）。
struct Uniforms {
    float2 vp;
    float4 u;
};

static inline float sdRoundBox(float2 p, float2 b, float4 r) {
    r.xy = (p.x > 0.0) ? r.xy : r.zw;
    r.x  = (p.y > 0.0) ? r.x  : r.y;
    float2 q = abs(p) - b + r.x;
    return min(max(q.x, q.y), 0.0) + length(max(q, 0.0)) - r.x;
}

// ---- 圆角矩形 / 阴影 / 描边 ----
struct ShapeIn {
    float2 pos    [[attribute(0)]];
    float2 local  [[attribute(1)]];
    float4 color  [[attribute(2)]];
    float4 color2 [[attribute(3)]];
    float2 halfSize [[attribute(4)]];
    float4 radius [[attribute(5)]];
    float4 params [[attribute(6)]];
};

struct ShapeOut {
    float4 position [[position]];
    float2 local;
    float4 color;
    float4 color2;
    float2 halfSize;
    float4 radius;
    float4 params;
    float2 t;
};

vertex ShapeOut v_shape(ShapeIn in [[stage_in]],
                        constant Uniforms &U [[buffer(1)]]) {
    ShapeOut o;
    float2 p = (in.pos / U.vp) * 2.0 - 1.0;
    o.position = float4(p.x, -p.y, 0.0, 1.0);
    o.local = in.local;
    o.color = in.color;
    o.color2 = in.color2;
    o.halfSize = in.halfSize;
    o.radius = in.radius;
    o.params = in.params;
    o.t = clamp((in.local + in.halfSize) / max(in.halfSize * 2.0, float2(1.0)), 0.0, 1.0);
    return o;
}

fragment float4 f_shape(ShapeOut in [[stage_in]]) {
    float mode    = in.params.y;
    float feather = max(in.params.x, 0.25);
    float strokeW = in.params.z;
    float glow    = in.params.w;

    float d = sdRoundBox(in.local, in.halfSize, in.radius);
    if (glow > 0.0) {
        float k = clamp(-d / max(glow, 0.001), 0.0, 1.0);
        d = d + glow * (1.0 - k);
    }

    float alpha;
    if (mode < 0.5) {
        alpha = 1.0 - smoothstep(-feather, feather, d);
    } else if (mode < 1.5) {
        alpha = 1.0 - smoothstep(-feather, feather, abs(d + strokeW * 0.5) - strokeW * 0.5);
    } else {
        alpha = (1.0 - smoothstep(-glow, glow, d)) * 0.9;
    }
    if (alpha <= 0.001) { discard_fragment(); }

    float4 col = in.color;
    if (in.color2.a >= 0.0) {
        col = mix(in.color, in.color2, in.t.x);
    }
    return float4(col.rgb, col.a * alpha);
}

// ---- 文字（R8 覆盖率图集 / RGBA 彩色 Emoji）----
struct TextIn {
    float2 pos   [[attribute(0)]];
    float2 uv    [[attribute(1)]];
    float4 color [[attribute(2)]];
    float mode   [[attribute(3)]];
};

struct TextOut {
    float4 position [[position]];
    float2 uv;
    float4 color;
    float mode;
};

vertex TextOut v_text(TextIn in [[stage_in]],
                      constant Uniforms &U [[buffer(1)]]) {
    TextOut o;
    float2 p = (in.pos / U.vp) * 2.0 - 1.0;
    o.position = float4(p.x, -p.y, 0.0, 1.0);
    o.uv = in.uv;
    o.color = in.color;
    o.mode = in.mode;
    return o;
}

fragment float4 f_text(TextOut in [[stage_in]],
                       texture2d<float> atlas [[texture(0)]],
                       sampler smp [[sampler(0)]],
                       constant Uniforms &U [[buffer(1)]]) {
    float4 c = atlas.sample(smp, in.uv);
    if (in.mode > 0.5) {
        if (c.a <= 0.004) { discard_fragment(); }
        return float4(c.rgb, c.a * in.color.a);
    }
    float a = c.r;
    if (a <= 0.004) { discard_fragment(); }
    float lum = dot(in.color.rgb, float3(0.2126, 0.7152, 0.0722));
    if (U.u.x != 1.0 && lum > 0.5) {
        a = pow(a, U.u.x);
    }
    return float4(in.color.rgb, in.color.a * a);
}

// ---- 图片（圆角裁剪 + 着色）----
struct ImgIn {
    float2 pos    [[attribute(0)]];
    float2 uv     [[attribute(1)]];
    float4 color  [[attribute(2)]];
    float2 local  [[attribute(3)]];
    float2 halfSize [[attribute(4)]];
    float4 radius [[attribute(5)]];
};

struct ImgOut {
    float4 position [[position]];
    float2 uv;
    float4 color;
    float2 local;
    float2 halfSize;
    float4 radius;
};

vertex ImgOut v_image(ImgIn in [[stage_in]],
                      constant Uniforms &U [[buffer(1)]]) {
    ImgOut o;
    float2 p = (in.pos / U.vp) * 2.0 - 1.0;
    o.position = float4(p.x, -p.y, 0.0, 1.0);
    o.uv = in.uv;
    o.color = in.color;
    o.local = in.local;
    o.halfSize = in.halfSize;
    o.radius = in.radius;
    return o;
}

fragment float4 f_image(ImgOut in [[stage_in]],
                        texture2d<float> tex [[texture(0)]],
                        sampler smp [[sampler(0)]],
                        constant Uniforms &U [[buffer(1)]]) {
    float d = sdRoundBox(in.local, in.halfSize, in.radius);
    float a = 1.0 - smoothstep(-0.7, 0.7, d);
    if (a <= 0.002) { discard_fragment(); }
    float2 uv = in.uv;
    // U.u.x = 1 表示纹理是离屏目标（第 0 行在屏幕顶部，GL 语义需翻 v）
    if (U.u.x > 0.5) { uv.y = 1.0 - uv.y; }
    float4 c = tex.sample(smp, uv);
    return float4(c.rgb * in.color.rgb, c.a * in.color.a * a);
}

// ---- 全屏四边形（毛玻璃 / 降采样 / 拷贝 / 清屏）----
struct QuadIn {
    float2 pos [[attribute(0)]];
    float2 uv  [[attribute(1)]];
};

struct QuadOut {
    float4 position [[position]];
    float2 uv;
};

vertex QuadOut v_quad(QuadIn in [[stage_in]]) {
    QuadOut o;
    o.position = float4(in.pos, 0.0, 1.0);
    o.uv = in.uv;
    return o;
}

// 直接按 buffer 取顶点（拷贝/清屏用，随后按需 setVertexBytes，不依赖 VBO）。
struct CopyVert {
    float2 pos;
    float2 uv;
};

struct CopyOut {
    float4 position [[position]];
    float2 uv;
};

vertex CopyOut v_copy(device const CopyVert *verts [[buffer(0)]],
                      uint vid [[vertex_id]]) {
    CopyOut o;
    o.position = float4(verts[vid].pos, 0.0, 1.0);
    o.uv = verts[vid].uv;
    return o;
}

// 离屏目标第 0 行语义是「屏幕底部」，采样时翻 v。
fragment float4 f_blur(QuadOut in [[stage_in]],
                       texture2d<float> tex [[texture(0)]],
                       sampler smp [[sampler(0)]],
                       constant Uniforms &U [[buffer(1)]]) {
    float2 vuv = float2(in.uv.x, 1.0 - in.uv.y);
    float2 o = U.u.xy * U.u.z;
    float4 c = tex.sample(smp, vuv) * 4.0;
    c += tex.sample(smp, vuv + float2(-o.x, 0.0)) * 2.0;
    c += tex.sample(smp, vuv + float2( o.x, 0.0)) * 2.0;
    c += tex.sample(smp, vuv + float2(0.0, -o.y)) * 2.0;
    c += tex.sample(smp, vuv + float2(0.0,  o.y)) * 2.0;
    c += tex.sample(smp, vuv + float2(-o.x, -o.y));
    c += tex.sample(smp, vuv + float2( o.x, -o.y));
    c += tex.sample(smp, vuv + float2(-o.x,  o.y));
    c += tex.sample(smp, vuv + float2( o.x,  o.y));
    return c / 16.0;
}

fragment float4 f_resolve(QuadOut in [[stage_in]],
                          texture2d<float> tex [[texture(0)]],
                          sampler smp [[sampler(0)]],
                          constant Uniforms &U [[buffer(1)]]) {
    float2 vuv = float2(in.uv.x, 1.0 - in.uv.y);
    float2 texel = U.u.xy;
    float span = U.u.z;
    float sharpen = U.u.w;
    float2 base = texel * (span * 0.125);
    float4 c = float4(0.0);
    for (int y = 0; y < 4; y++) {
        for (int x = 0; x < 4; x++) {
            float2 o = float2(float(x) * 2.0 - 3.0, float(y) * 2.0 - 3.0) * base;
            c += tex.sample(smp, vuv + o);
        }
    }
    c *= 0.0625;
    if (sharpen > 0.0) {
        float2 o = texel * span;
        float4 blur = (tex.sample(smp, vuv + float2(o.x, 0.0)) +
                       tex.sample(smp, vuv - float2(o.x, 0.0)) +
                       tex.sample(smp, vuv + float2(0.0, o.y)) +
                       tex.sample(smp, vuv - float2(0.0, o.y))) * 0.25;
        c = clamp(c + (c - blur) * sharpen, 0.0, 1.0);
    }
    return c;
}

// 拷贝：u = 源矩形（GL 语义 uv：x0, y0, w, h，y 从底部量）。
// 离屏目标第 0 行在屏幕顶部：先算视觉坐标，再整体翻转。
fragment float4 f_copy(CopyOut in [[stage_in]],
                       texture2d<float> tex [[texture(0)]],
                       sampler smp [[sampler(0)]],
                       constant Uniforms &U [[buffer(1)]]) {
    float2 suv = float2(U.u.x + in.uv.x * U.u.z, U.u.y + in.uv.y * U.u.w);
    return tex.sample(smp, float2(suv.x, 1.0 - suv.y));
}

// 清屏：u = 背景色。
fragment float4 f_solid(CopyOut in [[stage_in]],
                        constant Uniforms &U [[buffer(1)]]) {
    return U.u;
}
`
