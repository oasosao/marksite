package db

import (
	"database/sql"
	"fmt"
	"strings"

	"marksite/internal/pkg"
)

// SeedDemo 清空现有内容并灌入 demo 数据（9 目录 / 18 文章 / 6 页面 / 8 标签）
// 注意：会删除所有目录、文章、页面、标签、快照！
func SeedDemo(conn *sql.DB) error {
	tables := []string{"posts_tags", "pages_tags", "snapshots", "pages", "posts", "directories", "tags", "generate_tasks"}
	for _, t := range tables {
		if _, err := conn.Exec(`DELETE FROM ` + t); err != nil {
			return fmt.Errorf("清空 %s 失败: %w", t, err)
		}
	}

	now := pkg.NowStr()

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// 覆盖站点配置
	_, _ = tx.Exec(`UPDATE site_config SET site_name=?, site_desc=?, site_url=?, copyright=?, updated_at=?`,
		"MarkSite 演示站", "一个用 Go 开发的静态网站生成管理系统", "http://localhost:8080", "© 2026 MarkSite", now)

	type dirRow struct {
		id, parent, isPage, isPub, menuVisible, sort int
		name, slug, title, content, summary          string
	}
	dirs := []dirRow{
		{1, 0, 0, 1, 1, 1, "技术博客", "tech", "技术博客",
			md("# 技术博客\n\n编程、架构与工具链相关的思考与实践。"), "编程、架构与工具链相关的文章合集"},
		{2, 0, 0, 1, 1, 2, "生活随笔", "life", "生活随笔",
			md("# 生活随笔\n\n日常旅行、美食、阅读随想。"), "日常旅行、美食、阅读随想"},
		{3, 1, 0, 1, 1, 1, "Go 语言", "go", "Go 语言",
			md("## Go 语言专题\n\nGo 并发模型、标准库与工程实践。"), "Go 并发模型、标准库与工程实践"},
		{4, 1, 0, 1, 1, 2, "前端开发", "frontend", "前端开发",
			md("## 前端开发\n\nVue3、TypeScript、Vite 等现代前端技术栈。"), "Vue3、TypeScript、Vite 前端技术栈"},
		{5, 1, 0, 1, 1, 3, "DevOps", "devops", "DevOps",
			md("## DevOps\n\nCI/CD、容器化、云原生运维实践。"), "CI/CD、容器化、云原生实践"},
		{6, 2, 0, 1, 1, 1, "旅行游记", "travel", "旅行游记",
			md("## 旅行游记\n\n用文字和照片记录走过的每一个地方。"), "旅行见闻与游记"},
		{7, 2, 0, 1, 1, 2, "读书笔记", "books", "读书笔记",
			md("## 读书笔记\n\n阅读是最好的自我投资。"), "阅读笔记与好书推荐"},
		{99, 0, 1, 1, 1, 10, "关于我们", "about", "关于我们",
			md("# 关于我们\n\nMarkSite 是一款轻量级静态网站生成管理系统。\n\n- Go 开发，单二进制\n- 内置 Bluge 全文检索\n- 支持自定义主题与 Markdown 编辑器"), "关于本站的介绍"},
		{100, 0, 1, 1, 1, 11, "联系方式", "contact", "联系方式",
			md("# 联系方式\n\n- 邮箱：hello@example.com\n- GitHub：github.com/example"), "联系信息"},
	}
	for _, d := range dirs {
		if _, err := tx.Exec(`INSERT INTO directories
			(id,parent_id,name,slug,page_title,page_content,page_summary,is_page,is_published,menu_visible,sort,created_at,updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			d.id, d.parent, d.name, d.slug, d.title, d.content, d.summary,
			d.isPage, d.isPub, d.menuVisible, d.sort, now, now); err != nil {
			return fmt.Errorf("插入目录 %s: %w", d.name, err)
		}
	}

	type tagRow struct {
		id, sort   int
		name, slug string
	}
	tags := []tagRow{
		{1, 1, "Go", "go"}, {2, 2, "并发", "concurrency"},
		{3, 3, "Vue3", "vue3"}, {4, 4, "TypeScript", "typescript"},
		{5, 5, "Docker", "docker"}, {6, 6, "Kubernetes", "kubernetes"},
		{7, 7, "旅行", "travel"}, {8, 8, "读书笔记", "book-notes"},
	}
	for _, t := range tags {
		if _, err := tx.Exec(`INSERT INTO tags(id,name,slug,sort,created_at) VALUES(?,?,?,?,?)`,
			t.id, t.name, t.slug, t.sort, now); err != nil {
			return fmt.Errorf("插入标签 %s: %w", t.name, err)
		}
	}

	type postRow struct {
		id, dirID, sort, isPub        int
		title, slug, content, summary string
		tagIDs                        []int64
	}
	posts := []postRow{
		{1, 3, 1, 1, "Go 1.22 路由新特性详解", "go-1-22-routing",
			md("## 引言\n\nGo 1.22 的 ServeMux 现在支持方法匹配和路径通配符，告别第三方路由库。\n\n### 方法匹配\n\nGET / POST 分别注册处理器，一目了然。\n\n### 路径参数\n\n通过 PathValue 获取动态参数，零额外依赖。"),
			"Go 1.22 ServeMux 新增方法匹配和路径参数", []int64{1, 2}},
		{2, 3, 2, 1, "深入理解 Go 的 context 包", "go-context-deep-dive",
			md("## Context 本质\n\nContext 是跨 API 边界传递取消信号、超时时间和请求作用域值的标准机制。\n\n### 三种用法\n\n1. 取消传播 WithCancel\n2. 超时控制 WithTimeout / WithDeadline\n3. 值传递 WithValue"),
			"Context 三种用法：取消、超时、值传递", []int64{1, 2}},
		{3, 3, 3, 1, "用 channel 构建优雅的并发管道", "go-channel-patterns",
			md("## 生产者-消费者\n\nGo 的 channel 是实现并发管道的理想工具。\n\n### fan-out / fan-in\n\n多个 worker 并行处理，汇总结果。\n\n### 带取消的管道\n\n所有 goroutine 监听 ctx.Done() 优雅退出。"),
			"channel + goroutine 构建优雅并发管道", []int64{1, 2}},
		{4, 3, 4, 1, "Go 项目从零搭建：目录结构与工程化", "go-project-layout",
			md("## 推荐目录结构\n\ncmd 入口 / internal 私有 / pkg 可复用 / api 定义 / configs / scripts。\n\n### 工具链\n\ngo mod 依赖管理 + golangci-lint 代码检查 + make 任务编排。"),
			"Go 项目目录结构、依赖管理、工具链配置", []int64{1}},
		{5, 3, 5, 1, "SQLite 在生产环境的正确使用姿势", "sqlite-production-tips",
			md("## 为什么是 SQLite\n\n它不是玩具，是世界上部署量最大的数据库引擎之一。\n\n### 写并发\n\n单写者模型，WAL 模式下读不阻塞写。\n\n### 索引策略\n\nEXPLAIN QUERY PLAN 是你的好朋友。"),
			"SQLite 生产环境调优：WAL、索引、连接池", []int64{1}},

		{6, 4, 1, 1, "Vue3 Composition API 从入门到精通", "vue3-composition-api",
			md("## 为什么需要 Composition API\n\nOptions API 按类型组织代码，相关逻辑分散。Composition API 让同一逻辑聚在一起。\n\n### setup 函数\n\nsetup 是 Composition API 的入口，替代 data/methods/computed。\n\n### 自定义 Hook\n\n逻辑复用的终极方案：useXXX()。"),
			"Vue3 Composition API 让逻辑组织更清晰、复用更简单", []int64{3, 4}},
		{7, 4, 2, 1, "TypeScript 类型体操实战", "typescript-type-gymnastics",
			md("## 条件类型\n\nT extends string ? true : false，类型系统的 if/else。\n\n## 映射类型\n\nreadonly / Partial / Required 都是映射类型。\n\n## 模板字面量类型\n\n类型级别的字符串拼接，强大但要克制。"),
			"条件类型、映射类型、模板字面量类型实战", []int64{4}},
		{8, 4, 3, 1, "Vite 插件开发指南", "vite-plugin-guide",
			md("## Vite 插件 API\n\n基于 Rollup 插件接口，额外提供 Vite 专用钩子。\n\n### 常用钩子\n\nconfig 修改配置 / transform 转译源码 / configureServer 访问开发服务器 / buildEnd 构建结束。"),
			"从零开发 Vite Markdown 插件", []int64{3, 4}},
		{9, 4, 4, 1, "前端状态管理的演进之路", "frontend-state-evolution",
			md("## Props Drilling 到 Zustand\n\n状态管理方案的选择取决于应用规模。\n\n- 小型：reducer + Context\n- 中型：Zustand / Jotai\n- 大型：Redux Toolkit / MobX\n\n没有银弹，合适的才是最好的。"),
			"前端状态管理方案演进全梳理", []int64{3, 4}},

		{10, 5, 1, 1, "Docker Compose 多服务编排实战", "docker-compose-orchestration",
			md("## 基础示例\n\n声明 services、volumes、networks，一条命令启动整套环境。\n\n### 依赖与健康检查\n\ndepends_on + healthcheck 组合，确保启动顺序正确。\n\n### 生产部署\n\ncompose 配合 .env 和 profiles 用于生产也很成熟。"),
			"Docker Compose 从开发到生产完整指南", []int64{5}},
		{11, 5, 2, 1, "Kubernetes 核心概念速查", "k8s-core-concepts",
			md("## Pod\n\n最小部署单元，包含一个或多个容器。\n\n## Service\n\n为 Pod 提供稳定的访问入口。\n\n## Deployment\n\n声明式更新和回滚 Pod。\n\n## ConfigMap & Secret\n\n配置管理，敏感数据用 Secret。"),
			"Pod、Service、Deployment：K8s 核心组件快速上手", []int64{5, 6}},
		{12, 5, 3, 1, "GitLab CI 从简单到复杂", "gitlab-ci-pipeline",
			md("## 基础 Pipeline\n\nstages 定义阶段顺序，每个 job 声明 script。\n\n### 缓存与制品\n\ncache 加速重复构建，artifacts 传递产物。\n\n### 并行与矩阵\n\nparallel 并行执行，matrix 多维度参数化。"),
			"GitLab CI 配置、缓存、制品、并行构建完整教程", []int64{5}},

		{13, 6, 1, 1, "京都四日漫游：古寺与樱花", "kyoto-4days",
			md("## Day 1 东山\n\n清水寺 → 二年坂 → 八坂神社 → 祗园花见小路\n\n## Day 2 岚山\n\n渡月桥 → 天龙寺 → 竹林小径 → 常寂光寺\n\n## Day 3 金阁寺线\n\n金阁寺 → 龙安寺 → 岚山小火车 → 保津川漂流\n\n## Day 4 伏见稻荷\n\n千本鸟居从山脚爬到山顶。"),
			"京都四日行程规划，古寺与樱花", []int64{7}},
		{14, 6, 2, 1, "东京美食地图：从筑地到涩谷", "tokyo-food-map",
			md("## 寿司\n\n筑地场外市场的 Sushi Dai 必须早上 5 点去排队。\n\n## 拉面\n\n一蘭自动售票机点餐，次郎米其林三星提前数月预约。\n\n## 居酒屋\n\n新宿黄金街的小巷子里藏着无数宝藏小店。"),
			"东京寿司、拉面、居酒屋吃货朝圣之旅", []int64{7}},
		{15, 6, 3, 1, "大理洱海骑行两日", "dali-erhai-cycling",
			md("## 路线\n\nD1 古城 → 喜洲 → 海舌公园 → 双廊（住）\nD2 双廊 → 小普陀 → 挖色 → 返回古城\n\n## 补给\n\n喜洲吃破酥粑粑，双廊喝洱海鱼火锅。\n\n## 注意\n\n防晒！防晒！防晒！"),
			"大理洱海环湖两日骑行，路线与美食推荐", []int64{7}},

		{16, 7, 1, 1, "设计数据密集型应用 读书笔记", "ddia-notes",
			md("## 第一章 可靠性、可扩展性与可维护性\n\n可靠性是底线，可扩展性应对增长，可维护性降低长期成本。\n\n## 第二章 数据模型\n\n关系模型适合结构化数据，文档模型适合灵活 schema，图数据库适合多对多关系。"),
			"DDIA 每章核心要点，数据系统必读书", []int64{8}},
		{17, 7, 2, 1, "Go 程序设计语言 精读笔记", "go-pl-notes",
			md("## 第三章 基础\n\n变量、常量、类型转换，零值是 Go 的核心设计。\n\n## 第四章 复合类型\n\nslice 是对数组的包装，理解 len/cap/底层数组是关键。\n\n## 第六章 方法\n\n指针接收者 vs 值接收者，选择取决于是否需要修改。"),
			"Go 官方教程精读，每章一个核心概念", []int64{1, 8}},
		{18, 7, 3, 1, "重构 改善既有代码的设计 笔记", "refactoring-notes",
			md("## 代码坏味道\n\n过长函数 → 提取子函数。过大的类 → 拆分职责。重复代码 → DRY。\n\n## 重构手法\n\nExtract Method / Move Method / Replace Conditional with Polymorphism\n\n每一步小改，跑测试确认安全。"),
			"Martin Fowler 重构笔记：坏味道与手法总结", []int64{8}},
	}
	for _, p := range posts {
		cover := coverByDir(p.dirID)
		if _, err := tx.Exec(`INSERT INTO posts
			(id,dir_id,title,slug,content,summary,cover,sort,is_published,published_at,created_at,updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
			p.id, p.dirID, p.title, p.slug, p.content, p.summary, cover, p.sort, p.isPub, now, now, now); err != nil {
			return fmt.Errorf("插入文章 %s: %w", p.title, err)
		}
		for _, tid := range p.tagIDs {
			if _, err := tx.Exec(`INSERT INTO posts_tags(post_id,tag_id) VALUES(?,?)`, p.id, tid); err != nil {
				return err
			}
		}
	}

	type pageRow struct {
		id, dirID, sort, isPub        int
		title, slug, content, summary string
	}
	pages := []pageRow{
		{1, 99, 1, 1, "团队介绍", "team",
			md("## 核心团队\n\n### 张三 - 创始人\n全栈工程师，10 年 Web 开发经验。\n\n### 李四 - 前端负责人\nVue3 / React 双栈，热衷性能优化。\n\n### 王五 - 后端负责人\nGo / Rust 爱好者，分布式系统实践者。"),
			"MarkSite 核心团队成员介绍"},
		{2, 99, 2, 1, "使用文档", "docs",
			md("## 快速开始\n\n### 安装\n下载对应平台的二进制文件。\n\n### 启动\n./marksite -data ./data\n\n## 功能特性\n- Markdown 编辑器\n- 全文检索\n- 自定义主题\n- 一键生成静态站"),
			"MarkSite 快速上手指南"},
		{3, 99, 3, 1, "更新日志", "changelog",
			md("## v1.0.0 (2026-10-01)\n\n首个正式版本：Markdown 编辑 / 自定义主题 / Bluge 全文检索。\n\n## v0.9.0 (2026-09-15)\n\n目录树形管理 / 标签系统 / 自动快照。\n\n## v0.8.0 (2026-08-20)\n\n首个 beta 版本。"),
			"MarkSite 版本更新日志"},
		{4, 100, 1, 1, "常见问题", "faq",
			md("## 如何修改管理员密码？\n登录后台 → 设置 → 修改密码。\n\n## 数据存储在哪里？\n所有数据存储在 data 目录下的 marksite.db 文件中。\n\n## 如何备份？\n后台 → 备份 → 创建备份。生成 zip 包可直接恢复。"),
			"MarkSite 常见问题解答"},
		{5, 100, 2, 1, "隐私政策", "privacy",
			md("## 数据收集\nMarkSite 不收集任何用户数据，所有数据存储在您自己的服务器上。\n\n## Cookie\n本站仅使用必要的 session cookie 用于管理员登录鉴权。\n\n## 第三方服务\n不集成任何第三方统计或分析服务。"),
			"本站隐私政策声明"},
		{6, 0, 3, 1, "首页展示", "homepage",
			md("## 欢迎使用 MarkSite\n\n### 特色\n- 轻量：单二进制，部署即运行\n- 高效：Bluge 全文检索，毫秒级响应\n- 美观：内置主题 + 自定义主题支持\n\n点击右上角登录按钮进入管理后台。"),
			"网站首页示例页面"},
	}
	for _, p := range pages {
		cover := "/media/2026/10/default-cover.svg"
		if _, err := tx.Exec(`INSERT INTO pages
			(id,dir_id,title,slug,content,summary,cover,sort,is_published,published_at,created_at,updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
			p.id, p.dirID, p.title, p.slug, p.content, p.summary, cover, p.sort, p.isPub, now, now, now); err != nil {
			return fmt.Errorf("插入页面 %s: %w", p.title, err)
		}
	}

	return tx.Commit()
}

// coverByDir 根据目录 ID 返回对应的封面图路径（seed 内置 SVG 配图）
func coverByDir(dirID int) string {
	switch dirID {
	case 3:
		return "/media/2026/10/go-cover.svg"
	case 4:
		return "/media/2026/10/frontend-cover.svg"
	case 5:
		return "/media/2026/10/devops-cover.svg"
	case 6:
		return "/media/2026/10/travel-cover.svg"
	case 7:
		return "/media/2026/10/books-cover.svg"
	default:
		return "/media/2026/10/default-cover.svg"
	}
}

func md(s string) string { return strings.TrimSpace(s) }
