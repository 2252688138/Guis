// 桌宠素材工具：从 dsh-pet 仓库拉取动画 GIF 并生成 assets/pet/index.json。
//
// 用法（仓库根目录）：
//
//	go run ./tools/pet-assets -out assets/pet            # 下载 + 生成索引
//	go run ./tools/pet-assets -out assets/pet -index     # 只生成索引
//	go run ./tools/pet-assets -list                      # 列出内置清单
//
// 素材来源：https://github.com/PC2005-cloud/dsh-pet（CC BY-NC-SA 4.0，禁止商用）。
// 下载走 HTTPS_PROXY / HTTP_PROXY，未设置时回落到 -proxy 默认值（本机常用 127.0.0.1:8081）。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/gif"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// animEntry index.json 的一项。
type animEntry struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	File   string `json:"file"`
	Group  string `json:"group"`
	Weight int    `json:"weight"`
	Loop   bool   `json:"loop"`
}

// indexDoc index.json 整体结构。
type indexDoc struct {
	Version int         `json:"version"`
	Source  string      `json:"source"`
	Anims   []animEntry `json:"anims"`
}

// animRow 内置清单的一行（生成 index.json 的源数据）。
type animRow struct {
	id     string
	name   string
	group  string
	weight int
	loop   bool
}

const repoRaw = "https://raw.githubusercontent.com/PC2005-cloud/dsh-pet/main/dsh-pet/assets/preview/"

// catalog 全部 109 段动画。
// group 取值：idle/move/action/play/eat/festival/click/drag/sleep/sleepIn/sleepOut/
// workThink/workBusy/workTidy/workWait/workDone/workError/balance/talk。
// weight 为同组随机挑选的相对权重。
var catalog = []animRow{
	{"daiji-huxi-xiuxian", "待机呼吸", "idle", 40, true},
	{"dongzhangxiwang", "东张西望", "idle", 20, true},
	{"youxian-hengga", "悠闲哼歌", "idle", 10, false},
	{"chaoda-shenlanyao", "超大伸懒腰", "action", 6, false},
	{"haqian-liantian", "哈欠连天", "action", 6, false},
	{"yuandi-xiaoqi-chenmian", "原地小憩", "action", 6, false},
	{"zhao-jingzi", "照镜子", "action", 4, false},
	{"zhengti-huanzhuang-shise", "换装试色", "action", 3, false},
	{"nvpu-quxi-liyi", "屈膝礼仪", "action", 4, false},
	{"beixiayitiao-zhamao", "被吓一跳", "action", 4, false},
	{"xiaofudu-yuandi-360du-xuanzhuan-zhanshi", "360度展示", "action", 3, false},
	{"touchi-lingshi-bei-zhuazhu", "偷吃被抓", "action", 4, false},
	{"yong-jingyu-weiba-paidadi", "尾巴拍地", "action", 3, false},
	{"da-keshui-bei-jingxing", "瞌睡惊醒", "action", 4, false},
	{"yuandi-qiaoji-zhuomian-hudong", "敲击桌面", "action", 3, false},
	{"yuandi-zhongli-xiadun-yasuo", "重力下蹲", "action", 3, false},
	{"chenjian-shuaya", "晨间刷牙", "action", 3, false},
	{"yaoshan-naliang", "摇扇纳凉", "action", 3, false},
	{"qingkuai-jilu", "轻快记录", "action", 3, false},

	{"pangxie-zoulu", "螃蟹走路", "move", 12, true},
	{"yuandi-piaofu-tabu", "漂浮踏步", "move", 10, true},
	{"yuandi-zuozhuan-benpao", "原地奔跑", "move", 8, true},

	{"dianji-huiying-kaixin-yuedong", "点击-开心跃动", "click", 10, false},
	{"dianji-huiying-haixiu-jingya", "点击-害羞惊讶", "click", 10, false},
	{"dianji-huiying-aojiao-shengqi-ceshen-zhanshi", "点击-傲娇生气", "click", 10, false},
	{"dianji-huiying-naoyang-gegexiao", "点击-挠痒咯咯笑", "click", 10, false},
	{"dianji-huiying-yuanqi-huishou", "点击-元气挥手", "click", 10, false},

	{"beishubiao-tuozhuai-xuankong-fankui", "被拖拽悬空", "drag", 10, true},

	{"shuijiao-kaishi", "睡觉-开始", "sleepIn", 10, false},
	{"shuijiao-xunhuan", "睡觉-循环", "sleep", 10, true},
	{"shuijiao-qilai", "睡觉-起来", "sleepOut", 10, false},

	{"gongzuozhuangtai-sikao-maopao", "工作-思考", "workThink", 10, true},
	{"gongzuozhuangtai-manglu-dianan", "工作-忙碌", "workBusy", 10, true},
	{"gongzuozhuangtai-qingdian-guidang", "工作-整理归档", "workTidy", 10, true},
	{"gongzuozhuangtai-yuandi-duobu-zhangwang", "工作-等待张望", "workWait", 10, true},
	{"gongzuozhuangtai-queyue-qingzhu", "工作-成功", "workDone", 10, false},
	{"gongzuozhuangtai-chuitou-tanqi-maohan", "工作-出错", "workError", 10, false},
	{"xie-daima", "写代码", "workBusy", 4, true},

	{"qian-dai-man-yi", "余额-钱袋满溢", "balance", 1, false},
	{"jin-dai-ding-dang", "余额-金袋叮当", "balance", 1, false},
	{"qian-dai-ru-chang", "余额-钱袋如常", "balance", 1, false},
	{"shu-jin-zhou-mei", "余额-数金皱眉", "balance", 1, false},
	{"dai-kong-ru-xi", "余额-袋空如洗", "balance", 1, false},
	{"fen-wen-bu-sheng", "余额-分文不剩", "balance", 1, false},

	{"suisuinian-cazhuo-suisuinian", "擦桌碎碎念", "talk", 10, false},
	{"suisuinian-duiping-suisuinian", "对屏碎碎念", "talk", 10, false},
	{"suisuinian-fadai-suisuinian", "发呆碎碎念", "talk", 10, false},
	{"shendu-sikao-suisuinian", "深度思考碎碎念", "talk", 6, false},
	{"shia-chishenme", "是啊吃什么", "talk", 6, false},

	{"yuandi-zhuanxin-wan-mofang", "玩魔方", "play", 3, false},
	{"yuandi-dunxia-wan-wanju-qiche", "玩玩具汽车", "play", 3, false},
	{"jingyu-tu-paopao-texiao", "鲸鱼吐泡泡", "play", 3, false},
	{"yuandi-tiaoyue-zhuasui-touding-wupin", "跳跃抓碎", "play", 3, false},
	{"wan-youxi-qijibaituai", "玩游戏气急", "play", 3, false},
	{"wan-shuiqiang", "玩水枪", "play", 3, false},
	{"xiaotiqin-yanzou", "小提琴演奏", "play", 3, false},
	{"lanjing-xianshi", "蓝鲸现世", "play", 2, false},
	{"youya-nvpuwu", "优雅女仆舞", "play", 3, false},
	{"qingkuai-yaobaiwu", "轻快摇摆舞", "play", 3, false},
	{"keai-zhaiwu", "可爱宅舞", "play", 3, false},
	{"chui-qiqiu", "吹气球", "play", 3, false},
	{"dongwu-huanrao", "动物环绕", "play", 2, false},
	{"fang-fengzheng", "放风筝", "play", 2, false},
	{"chai-liwu", "拆礼物", "play", 3, false},
	{"bian-gezi", "变鸽子", "play", 2, false},
	{"puke-moshu", "扑克魔术", "play", 3, false},
	{"chou-tuoluo", "抽陀螺", "play", 3, false},
	{"chui-dizi", "吹笛子", "play", 2, false},
	{"hudie-mifeng-huanrao-touding-kaihua", "蝶蜂开花", "play", 2, false},
	{"lu-mao", "撸猫", "play", 3, false},
	{"pingkong-shenghua", "凭空生花", "play", 2, false},
	{"qi-muma", "骑木马", "play", 3, false},
	{"sanqiu-paojie", "三球抛接", "play", 2, false},
	{"ti-jianzi", "踢毽子", "play", 3, false},
	{"xiawuziqi", "下五子棋", "play", 2, false},
	{"dangqiuqian", "荡秋千", "play", 2, false},

	{"chi-baifan", "吃白饭", "eat", 3, false},
	{"dakou-chi-lingshi", "大口吃零食", "eat", 3, false},
	{"chi-token", "吃Token", "eat", 3, false},
	{"chi-zaocan", "吃早餐", "eat", 2, false},
	{"chi-wucan", "吃午餐", "eat", 2, false},
	{"chi-wancan", "吃晚餐", "eat", 2, false},
	{"chi-bingqilin-ronghua", "冰淇淋融化", "eat", 2, false},
	{"chi-dazhaxie", "吃大闸蟹", "eat", 2, false},
	{"chi-tanghulu", "吃糖葫芦", "eat", 2, false},
	{"chi-changshoumian", "吃长寿面", "eat", 2, false},
	{"chi-xigua", "吃西瓜", "eat", 2, false},
	{"shuan-huoguo", "涮火锅", "eat", 2, false},
	{"chijiaozi", "吃饺子", "eat", 2, false},
	{"chitangyuan", "吃汤圆", "eat", 2, false},

	{"beiluoye-yanmo", "被落叶淹没", "festival", 1, false},
	{"zhongqiu-shangyue-chi-yuebing", "中秋赏月", "festival", 1, false},
	{"duixueren", "堆雪人", "festival", 1, false},
	{"fang-yanhua", "放烟花", "festival", 1, false},
	{"chi-zongzi", "吃粽子", "festival", 1, false},
	{"chi-niangao", "吃年糕", "festival", 1, false},
	{"chi-qingtuan", "吃青团", "festival", 1, false},
	{"chi-labazhou", "吃腊八粥", "festival", 1, false},
	{"chi-chongyanggao", "吃重阳糕", "festival", 1, false},
	{"shou-hongbao", "收红包", "festival", 1, false},
	{"xie-fuzi", "写福字", "festival", 1, false},
	{"chuanzhenqiqiao", "穿针乞巧", "festival", 1, false},
	{"wu-shitou", "舞狮头", "festival", 1, false},
	{"taotang-nanguadeng", "讨糖南瓜灯", "festival", 1, false},
	{"cha-zhuyu-shangju", "插茱萸赏菊", "festival", 1, false},
	{"fanghedeng", "放河灯", "festival", 1, false},
	{"menghua-xiaoyouling", "萌化小幽灵", "festival", 1, false},
	{"zhuangdian-shengdanshu", "装点圣诞树", "festival", 1, false},
	{"fang-kongmingdeng", "放孔明灯", "festival", 1, false},
}

func main() {
	out := flag.String("out", "assets/pet", "素材输出目录")
	indexOnly := flag.Bool("index", false, "只生成 index.json，不下载")
	list := flag.Bool("list", false, "打印内置清单")
	verify := flag.Bool("verify", false, "校验素材目录的 GIF，删除损坏文件")
	jobs := flag.Int("jobs", 6, "并发下载数")
	proxy := flag.String("proxy", "http://127.0.0.1:8081", "下载代理（环境变量 HTTPS_PROXY 优先）")
	flag.Parse()

	if *list {
		for _, c := range catalog {
			fmt.Printf("%-45s %-10s %s\n", c.id, c.group, c.name)
		}
		return
	}

	if *verify {
		bad, missing := verifyAssets(*out)
		for _, id := range bad {
			fmt.Println("已删除损坏文件:", id)
		}
		for _, id := range missing {
			fmt.Println("缺失:", id)
		}
		fmt.Printf("校验完成：损坏 %d，缺失 %d，共 %d 段\n", len(bad), len(missing), len(catalog))
		return
	}

	if !*indexOnly {
		if err := download(*out, *jobs, *proxy); err != nil {
			fmt.Fprintln(os.Stderr, "下载失败:", err)
			os.Exit(1)
		}
	}
	if err := writeIndex(*out); err != nil {
		fmt.Fprintln(os.Stderr, "写索引失败:", err)
		os.Exit(1)
	}
	fmt.Printf("完成：%d 段动画 → %s\n", len(catalog), *out)
}

// download 并发拉取全部 GIF（已存在且非空则跳过）。
func download(dir string, jobs int, proxy string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	client := &http.Client{Timeout: 120 * time.Second}
	if p := proxyOf(proxy); p != nil {
		client.Transport = &http.Transport{Proxy: http.ProxyURL(p)}
	}

	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failed []string
	for _, c := range catalog {
		path := filepath.Join(dir, c.id+".gif")
		if st, err := os.Stat(path); err == nil && st.Size() > 1024 {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(id, path string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := fetch(client, repoRaw+url.PathEscape(id)+".gif", path); err != nil {
				mu.Lock()
				failed = append(failed, fmt.Sprintf("%s: %v", id, err))
				mu.Unlock()
			}
		}(c.id, path)
	}
	wg.Wait()
	if len(failed) > 0 {
		sort.Strings(failed)
		return fmt.Errorf("%d 个文件失败：\n%s", len(failed), strings.Join(failed, "\n"))
	}
	return nil
}

// proxyOf 优先用环境变量代理，其次用参数默认值。
func proxyOf(fallback string) *url.URL {
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			if u, err := url.Parse(v); err == nil && u.Host != "" {
				return u
			}
		}
	}
	if fallback == "" {
		return nil
	}
	u, err := url.Parse(fallback)
	if err != nil || u.Host == "" {
		return nil
	}
	return u
}

// fetch 下载单个文件到 path（先写 .part 再改名，避免中断留半包）。
func fetch(client *http.Client, src, path string) error {
	resp, err := client.Get(src)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// writeIndex 生成 index.json（按 ID 排序，便于 diff）。
func writeIndex(dir string) error {
	entries := make([]animEntry, 0, len(catalog))
	for _, c := range catalog {
		entries = append(entries, animEntry{
			ID: c.id, Name: c.name, File: c.id + ".gif",
			Group: c.group, Weight: c.weight, Loop: c.loop,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	doc := indexDoc{Version: 1, Source: "https://github.com/PC2005-cloud/dsh-pet", Anims: entries}
	buf, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	return os.WriteFile(filepath.Join(dir, "index.json"), buf, 0o644)
}

// verifyAssets 逐段解码校验：损坏的直接删掉（下载脚本下次会补），返回损坏与缺失清单。
func verifyAssets(dir string) (bad, missing []string) {
	for _, c := range catalog {
		path := filepath.Join(dir, c.id+".gif")
		f, err := os.Open(path)
		if err != nil {
			missing = append(missing, c.id)
			continue
		}
		_, err = gif.DecodeAll(f)
		f.Close()
		if err != nil {
			bad = append(bad, c.id)
			os.Remove(path)
		}
	}
	return bad, missing
}
