# MarkSite 主题制作指南

本文档介绍如何为 MarkSite 制作自定义主题。MarkSite 主题基于 Go 标准库 `html/template` 模板引擎，无需构建工具，纯 HTML + CSS + JS 即可。

## 一、主题目录结构

一个主题就是一个目录，结构如下：

```
mytheme/                  # 主题目录名 = theme.json 中的 name
├── theme.json            # 主题元信息（必需）
├── templates/            # 模板目录（必需）
│   ├── index.html        # 首页 / 分页页（必需）
│   ├── post.html         # 文章详情页（必需）
│   ├── page.html         # 独立页面详情（必需）
│   ├── archive.html      # 归档页（必需）
│   ├── category.html     # 分类页（可选，缺省则不生成分类页）
│   └── tag.html          # 标签页（可选，缺省则不生成标签页）
└── static/               # 静态资源（可选）
    ├── style.css
    └── ...
```

### 文件说明

| 文件 | 必需 | 说明 |
|---|---|---|
| `theme.json` | ✓ | 主题元信息 |
| `templates/index.html` | ✓ | 首页、分页页 |
| `templates/post.html` | ✓ | 文章详情页 |
| `templates/page.html` | ✓ | 独立页面详情页 |
| `templates/archive.html` | ✓ | 按年份归档页 |
| `templates/category.html` | ✗ | 分类页，不提供则不生成分类页 |
| `templates/tag.html` | ✗ | 标签页，不提供则不生成标签页 |
| `static/` | ✗ | CSS/JS/图片等静态资源，生成时复制到输出目录 `/static/` |

## 二、theme.json 格式

```json
{
  "name": "mytheme",
  "version": "1.0.0",
  "author": "你的名字",
  "description": "主题描述"
}
```

### 字段说明

| 字段 | 必需 | 规则 |
|---|---|---|
| `name` | ✓ | 主题名，小写字母/数字开头，2-32 字符，仅含 `a-z0-9-_`；不可与内置主题（`default`/`dark`/`ink`/`paper`/`vibrant`）重名 |
| `version` | ✗ | 版本号，缺省 `1.0.0` |
| `author` | ✗ | 作者名 |
| `description` | ✗ | 主题描述 |

## 三、模板语法

MarkSite 使用 Go `html/template`，常用语法：

```gotemplate
{{.字段名}}              输出变量（自动 HTML 转义）
{{.字段名 | safe}}        不转义输出（用于 HTML 内容）
{{if .字段}}...{{end}}    条件判断
{{if .字段}}...{{else}}...{{end}}
{{range .列表}}...{{end}}  遍历列表
{{with .字段}}...{{end}}   缩小作用域
{{.字段 | printf "%s"}}    管道函数
```

> 注意：`ContentHTML`、`CustomHead`、`CustomFooter` 字段已标记为 `template.HTML`，输出时不会转义，可直接写 `{{.Post.ContentHTML}}`。

## 四、模板数据结构

### 4.1 站点信息 `.Site`（所有页面都有）

| 字段 | 类型 | 说明 |
|---|---|---|
| `.Site.Name` | string | 站点名称 |
| `.Site.Desc` | string | 站点描述 |
| `.Site.URL` | string | 站点 URL |
| `.Site.Logo` | string | Logo 图片路径（相对，需拼 `/assets/`） |
| `.Site.Favicon` | string | Favicon 路径（相对，需拼 `/assets/`） |
| `.Site.Copyright` | string | 版权信息 |
| `.Site.CustomHead` | HTML | 自定义头部 HTML（不转义） |
| `.Site.CustomFooter` | HTML | 自定义底部 HTML（不转义） |
| `.Site.HomeURL` | string | 首页 URL |
| `.Site.ArchiveURL` | string | 归档页 URL |
| `.Site.RSSURL` | string | RSS 订阅 URL |
| `.Site.AssetBase` | string | 资源基础路径 |
| `.Site.GeneratedAt` | string | 生成时间 |
| `.Site.PageSize` | int | 每页文章数 |
| `.Site.NavMenu` | []NavItem | 导航菜单 |

**NavItem 结构**：

| 字段 | 类型 | 说明 |
|---|---|---|
| `.Label` | string | 菜单文字 |
| `.URL` | string | 链接地址 |
| `.Slug` | string | 标识 |
| `.IsPage` | bool | 是否为页面 |
| `.Children` | []NavItem | 子菜单 |

### 4.2 文章卡片 `.Posts` / `.Post`（列表项）

| 字段 | 类型 | 说明 |
|---|---|---|
| `.ID` | int64 | 文章 ID |
| `.Title` | string | 标题 |
| `.Slug` | string | URL 别名 |
| `.Summary` | string | 摘要 |
| `.Cover` | string | 封面图 URL（已处理为 `/assets/xxx`） |
| `.Tags` | []Tag | 标签列表 |
| `.URL` | string | 文章链接 |
| `.PublishDate` | string | 发布日期 |
| `.DirID` | int64 | 所属目录 ID |

### 4.3 文章详情 `.Post`（post.html）

在 PostCard 基础上增加：

| 字段 | 类型 | 说明 |
|---|---|---|
| `.ContentHTML` | HTML | 正文 HTML（不转义） |
| `.Gallery` | []string | 图集图片 URL 列表（已处理为 `/assets/xxx`） |

### 4.4 页面详情 `.Page`（page.html）

| 字段 | 类型 | 说明 |
|---|---|---|
| `.ID` | int64 | 页面 ID |
| `.Title` | string | 标题 |
| `.Slug` | string | URL 别名 |
| `.Summary` | string | 摘要 |
| `.Cover` | string | 封面图 URL |
| `.Gallery` | []string | 图集图片 URL 列表 |
| `.URL` | string | 页面链接 |
| `.ContentHTML` | HTML | 正文 HTML |

### 4.5 标签 `.Tags`

| 字段 | 类型 | 说明 |
|---|---|---|
| `.Name` | string | 标签名 |
| `.Slug` | string | URL 别名（链接为 `/tags/{Slug}.html`） |

### 4.6 分页 `.Pagination`（index.html）

| 字段 | 类型 | 说明 |
|---|---|---|
| `.Page` | int | 当前页码 |
| `.TotalPages` | int | 总页数 |
| `.Total` | int | 文章总数 |
| `.PrevURL` | string | 上一页 URL（空表示无） |
| `.NextURL` | string | 下一页 URL（空表示无） |

### 4.7 分组 `.Groups`（index.html 分类聚合页）

当分类页跨多个子目录时，文章按子目录分组：

| 字段 | 类型 | 说明 |
|---|---|---|
| `.DirID` | int64 | 目录 ID |
| `.DirName` | string | 目录名 |
| `.DirSlug` | string | 目录别名 |
| `.URL` | string | 子目录独立页链接 |
| `.Posts` | []PostCard | 该目录下的文章 |

### 4.8 归档 `.Years`（archive.html）

| 字段 | 类型 | 说明 |
|---|---|---|
| `.Year` | int | 年份 |
| `.Posts` | []PostCard | 该年份的文章 |

## 五、各页面数据模型

### index.html（首页/分页/分类/标签）

数据：`IndexData`

```gotemplate
{{.Site}}        站点信息
{{.Posts}}       文章列表 []PostCard
{{.Pagination}}  分页信息
{{.Heading}}     页面标题（分类名/标签名，首页为空）
{{.Groups}}      子目录分组（仅分类聚合页有值）
```

### post.html（文章详情）

数据：`PostData`

```gotemplate
{{.Site}}   站点信息
{{.Post}}   文章详情 PostView
```

### page.html（页面详情）

数据：`PageData`

```gotemplate
{{.Site}}   站点信息
{{.Page}}   页面详情 PageView
```

### archive.html（归档）

数据：`ArchiveData`

```gotemplate
{{.Site}}   站点信息
{{.Years}}  年份分组 []YearGroup
```

## 六、静态资源

### 主题自带资源

主题 `static/` 目录下的所有文件会被复制到生成站点的 `/static/` 目录下，保持目录结构。

```
主题 static/style.css  →  输出 /static/style.css
主题 static/js/app.js  →  输出 /static/js/app.js
```

模板中引用：

```html
<link rel="stylesheet" href="/static/style.css">
<script src="/static/js/app.js"></script>
```

### 资源库资源

用户在后台上传的资源（封面、图集、正文插图）位于 `/assets/` 目录下。模板中通过 `.Cover`、`.Gallery` 等字段获取的路径已自动处理为 `/assets/xxx` 格式，可直接使用。

Logo 和 Favicon 需要手动拼接：

```html
{{if .Site.Logo}}<img src="/assets/{{.Site.Logo}}" alt="logo">{{end}}
{{if .Site.Favicon}}<link rel="icon" href="/assets/{{.Site.Favicon}}">{{end}}
```

## 七、最小可用主题示例

### theme.json

```json
{
  "name": "minimal",
  "version": "1.0.0",
  "author": "You",
  "description": "极简主题"
}
```

### templates/index.html

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>{{if .Heading}}{{.Heading}} - {{end}}{{.Site.Name}}</title>
  <link rel="stylesheet" href="/static/style.css">
</head>
<body>
  <header>
    <h1><a href="/">{{.Site.Name}}</a></h1>
    <nav>
      <a href="/">首页</a>
      <a href="/archive.html">归档</a>
    </nav>
  </header>
  <main>
    {{range .Posts}}
    <article>
      <h2><a href="{{.URL}}">{{.Title}}</a></h2>
      <time>{{.PublishDate}}</time>
      {{if .Summary}}<p>{{.Summary}}</p>{{end}}
    </article>
    {{end}}
    {{if gt .Pagination.TotalPages 1}}
    <nav>
      {{if .Pagination.PrevURL}}<a href="{{.Pagination.PrevURL}}">上一页</a>{{end}}
      <span>{{.Pagination.Page}} / {{.Pagination.TotalPages}}</span>
      {{if .Pagination.NextURL}}<a href="{{.Pagination.NextURL}}">下一页</a>{{end}}
    </nav>
    {{end}}
  </main>
  <footer>{{.Site.Copyright}}</footer>
</body>
</html>
```

### templates/post.html

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>{{.Post.Title}} - {{.Site.Name}}</title>
  <link rel="stylesheet" href="/static/style.css">
</head>
<body>
  <article>
    <h1>{{.Post.Title}}</h1>
    <time>{{.Post.PublishDate}}</time>
    <div class="content">{{.Post.ContentHTML}}</div>
    {{if .Post.Gallery}}
    <div class="gallery">
      {{range .Post.Gallery}}<img src="{{.}}" alt="">{{end}}
    </div>
    {{end}}
  </article>
</body>
</html>
```

### templates/page.html

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>{{.Page.Title}} - {{.Site.Name}}</title>
  <link rel="stylesheet" href="/static/style.css">
</head>
<body>
  <article>
    <h1>{{.Page.Title}}</h1>
    <div class="content">{{.Page.ContentHTML}}</div>
  </article>
</body>
</html>
```

### templates/archive.html

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>归档 - {{.Site.Name}}</title>
  <link rel="stylesheet" href="/static/style.css">
</head>
<body>
  <h1>归档</h1>
  {{range .Years}}
  <section>
    <h2>{{.Year}}</h2>
    <ul>
      {{range .Posts}}
      <li>{{.PublishDate}} <a href="{{.URL}}">{{.Title}}</a></li>
      {{end}}
    </ul>
  </section>
  {{end}}
</body>
</html>
```

### static/style.css

```css
body { font-family: system-ui, sans-serif; max-width: 720px; margin: 0 auto; padding: 20px; }
article { margin: 24px 0; }
time { color: #888; font-size: 14px; }
.content { line-height: 1.8; }
.gallery { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 10px; }
.gallery img { width: 100%; border-radius: 6px; }
```

## 八、打包与上传

### 打包

将主题目录打包为 zip，**zip 内可以是主题目录本身，也可以直接包含 theme.json 和 templates**：

```bash
# 方式一：包含主题目录（推荐）
cd /path/to/themes
zip -r mytheme.zip mytheme/

# 方式二：直接打包内容
cd /path/to/themes/mytheme
zip -r ../mytheme.zip .
```

两种方式都支持，系统会自动识别。

### 上传

1. 进入后台 → 主题管理
2. 点击「上传主题 zip」
3. 选择 zip 文件上传
4. 上传成功后在列表中点击「激活」
5. 重新生成站点使主题生效

### 命名限制

- 主题名必须匹配正则 `^[a-z0-9][a-z0-9-_]{1,31}$`
- 不可与内置主题重名：`default`、`dark`、`ink`、`paper`、`vibrant`

## 九、调试技巧

1. **模板语法错误**：生成站点时会报错，提示具体模板和行号
2. **字段为空**：用 `{{if .字段}}...{{end}}` 包裹，避免渲染空内容
3. **图片不显示**：检查封面/图集路径是否为 `/assets/xxx`，Logo/Favicon 是否拼了 `/assets/`
4. **样式不生效**：确认 `static/` 目录下的文件已被复制到输出目录的 `/static/`
5. **本地预览**：生成后用后台「预览」功能查看效果

## 十、注意事项

1. **模板文件必须存在**：`index.html`、`post.html`、`page.html`、`archive.html` 缺一不可，否则安装失败
2. **主题名即目录名**：`theme.json` 中的 `name` 必须与上传后的目录名一致（系统会以 name 作为目录名）
3. **路径使用正斜杠**：模板中所有 URL 路径使用 `/`，不要用 `\`
4. **图集渲染**：`.Gallery` 是字符串数组，每个元素是完整的图片 URL，直接 `<img src="{{.}}">` 即可
5. **正文 HTML**：`.ContentHTML` 已渲染为 HTML，直接输出，不要用 `printf` 或转义
6. **覆盖已有主题**：上传同名主题会被拒绝，请先在后台删除旧主题

## 十一、参考内置主题

MarkSite 内置 5 套主题可作为参考，源码位于 `internal/theme/` 目录：

- `default` — 简洁浅色博客
- `dark` — 深色主题
- `ink` — 墨水风
- `paper` — 纸张风
- `vibrant` — 鲜艳配色

建议从复制 `default` 主题开始，修改 CSS 和模板结构来打造自己的主题。
