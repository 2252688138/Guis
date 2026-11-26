// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LoadOBJ 从 Wavefront OBJ 文件加载网格（支持 v / vt / vn / f，多边形自动扇形三角化）。
func LoadOBJ(path string) (*Mesh, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := LoadOBJBytes(b)
	if err != nil {
		return nil, fmt.Errorf("game: 解析 OBJ %s 失败: %w", path, err)
	}
	return m, nil
}

// objKey OBJ 顶点属性组合去重键。
type objKey struct{ v, t, n int }

// LoadOBJBytes 从内存字节加载 OBJ 网格。
func LoadOBJBytes(b []byte) (*Mesh, error) {
	var (
		pos []Vec3
		nrm []Vec3
		uv  []Vec2
	)
	m := &Mesh{}
	index := map[objKey]uint32{}

	// resolve 把 OBJ 的 1 基（或负数相对）下标转成 0 基。
	resolve := func(s string, count int) (int, bool) {
		if s == "" {
			return -1, true
		}
		i, err := strconv.Atoi(s)
		if err != nil {
			return 0, false
		}
		if i > 0 {
			return i - 1, true
		}
		if i < 0 {
			return count + i, true
		}
		return 0, false
	}

	sc := bufio.NewScanner(strings.NewReader(string(b)))
	sc.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		switch fields[0] {
		case "v":
			if len(fields) < 4 {
				continue
			}
			x, _ := strconv.ParseFloat(fields[1], 32)
			y, _ := strconv.ParseFloat(fields[2], 32)
			z, _ := strconv.ParseFloat(fields[3], 32)
			pos = append(pos, V3(float32(x), float32(y), float32(z)))
		case "vn":
			if len(fields) < 4 {
				continue
			}
			x, _ := strconv.ParseFloat(fields[1], 32)
			y, _ := strconv.ParseFloat(fields[2], 32)
			z, _ := strconv.ParseFloat(fields[3], 32)
			nrm = append(nrm, V3(float32(x), float32(y), float32(z)))
		case "vt":
			if len(fields) < 3 {
				continue
			}
			x, _ := strconv.ParseFloat(fields[1], 32)
			y, _ := strconv.ParseFloat(fields[2], 32)
			uv = append(uv, V2(float32(x), 1-float32(y)))
		case "f":
			if len(fields) < 4 {
				continue
			}
			ring := make([]uint32, 0, len(fields)-1)
			for _, f := range fields[1:] {
				parts := strings.Split(f, "/")
				vi, ok := resolve(parts[0], len(pos))
				if !ok || vi < 0 || vi >= len(pos) {
					continue
				}
				ti, ni := -1, -1
				if len(parts) > 1 {
					ti, _ = resolve(parts[1], len(uv))
				}
				if len(parts) > 2 {
					ni, _ = resolve(parts[2], len(nrm))
				}
				k := objKey{vi, ti, ni}
				id, ok := index[k]
				if !ok {
					id = uint32(len(m.Positions))
					index[k] = id
					m.Positions = append(m.Positions, pos[vi])
					if ti >= 0 && ti < len(uv) {
						m.UVs = append(m.UVs, uv[ti])
					} else {
						m.UVs = append(m.UVs, Vec2{})
					}
					if ni >= 0 && ni < len(nrm) {
						m.Normals = append(m.Normals, nrm[ni].Normalize())
					} else {
						m.Normals = append(m.Normals, Vec3{})
					}
				}
				ring = append(ring, id)
			}
			if len(ring) < 3 {
				continue
			}
			for k := 1; k+1 < len(ring); k++ {
				m.Indices = append(m.Indices, ring[0], ring[k], ring[k+1])
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(m.Positions) == 0 || len(m.Indices) == 0 {
		return nil, fmt.Errorf("game: OBJ 中没有有效的顶点/面")
	}
	m.RecomputeBounds()

	// 有顶点法线全为 0 时补平滑法线
	missing := true
	for _, n := range m.Normals {
		if n.Len() > 0 {
			missing = false
			break
		}
	}
	if missing {
		m.RecomputeNormals()
	}
	return m, nil
}
