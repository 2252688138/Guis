// 统计内置工具定义的 JSON 体积（用于定位每轮「未命中 token」是否来自 tools schema）。
package main

import (
	"encoding/json"
	"fmt"
	"sort"

	"guis/cmd/deepseek/agent"
)

func main() {
	r := agent.BuildRegistry(nil)
	specs := r.Specs()
	type row struct {
		name string
		size int
	}
	rows := make([]row, 0, len(specs))
	total := 0
	for _, s := range specs {
		b, _ := json.Marshal(s)
		rows = append(rows, row{s.Name, len(b)})
		total += len(b)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].size > rows[j].size })
	fmt.Printf("tools=%d total=%dbytes approxTokens=%d\n", len(specs), total, total/2)
	for _, x := range rows {
		fmt.Printf("%s|%d\n", x.name, x.size)
	}
}
