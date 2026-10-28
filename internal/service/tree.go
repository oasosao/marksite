package service

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"marksite/internal/model"
	"marksite/internal/pkg"
	"marksite/internal/repo"
)

// DeleteDirResult 删除分类结果统计
type DeleteDirResult struct {
	MovedDirs    int `json:"moved_dirs"`
	MovedPosts   int `json:"moved_posts"`
	MovedPages   int `json:"moved_pages"`
	DeletedDirs  int `json:"deleted_dirs"`
	DeletedPosts int `json:"deleted_posts"`
	DeletedPages int `json:"deleted_pages"`
}

// Tree 组装混合内容树（directories + posts + pages）
func (a *App) Tree(ctx context.Context) ([]*model.TreeNode, error) {
	dirs, err := a.Repo.ListDirs(ctx)
	if err != nil {
		return nil, err
	}
	posts, err := a.Repo.ListAllPosts(ctx)
	if err != nil {
		return nil, err
	}
	pages, err := a.Repo.ListAllPages(ctx)
	if err != nil {
		return nil, err
	}

	dirByID := map[int64]model.Directory{}
	for _, d := range dirs {
		dirByID[d.ID] = d
	}
	// 防御循环分类：无法回溯到根的分类挂到根
	effectiveParent := map[int64]int64{}
	for _, d := range dirs {
		effective := d.ParentID
		cur := effective
		seen := map[int64]bool{d.ID: true}
		for cur != 0 {
			if _, ok := dirByID[cur]; !ok {
				effective = 0
				break
			}
			if seen[cur] {
				effective = 0
				break
			}
			seen[cur] = true
			cur = dirByID[cur].ParentID
		}
		effectiveParent[d.ID] = effective
	}

	children := map[int64][]*model.TreeNode{}
	for _, d := range dirs {
		node := &model.TreeNode{
			Key: "dir-" + itoa(d.ID), Type: "dir", ID: d.ID, ParentID: effectiveParent[d.ID],
			Label: d.Name, Slug: d.Slug, Sort: d.Sort, IsPublished: d.IsPublished,
			IsPage: d.IsPage, MenuVisible: d.MenuVisible, Children: []*model.TreeNode{},
		}
		children[effectiveParent[d.ID]] = append(children[effectiveParent[d.ID]], node)
	}
	for _, p := range posts {
		parent := p.DirID
		if _, ok := dirByID[parent]; !ok {
			parent = 0
		}
		node := &model.TreeNode{
			Key: "post-" + itoa(p.ID), Type: "post", ID: p.ID, ParentID: parent,
			Label: p.Title, Slug: p.Slug, Sort: p.Sort, IsPublished: p.IsPublished,
			Children: []*model.TreeNode{},
		}
		children[parent] = append(children[parent], node)
	}
	for _, p := range pages {
		parent := p.DirID
		if _, ok := dirByID[parent]; !ok {
			parent = 0
		}
		node := &model.TreeNode{
			Key: "page-" + itoa(p.ID), Type: "page", ID: p.ID, ParentID: parent,
			Label: p.Title, Slug: p.Slug, Sort: p.Sort, IsPublished: p.IsPublished,
			Children: []*model.TreeNode{},
		}
		children[parent] = append(children[parent], node)
	}
	for parentID, list := range children {
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].Sort != list[j].Sort {
				return list[i].Sort < list[j].Sort
			}
			return list[i].ID < list[j].ID
		})
		children[parentID] = list
	}
	// 挂载子节点
	for _, d := range dirs {
		node := findDirNode(children[effectiveParent[d.ID]], d.ID)
		if node != nil {
			node.Children = children[d.ID]
			if node.Children == nil {
				node.Children = []*model.TreeNode{}
			}
		}
	}
	roots := children[0]
	if roots == nil {
		roots = []*model.TreeNode{}
	}
	return roots, nil
}

func findDirNode(list []*model.TreeNode, id int64) *model.TreeNode {
	for _, n := range list {
		if n.Type == "dir" && n.ID == id {
			return n
		}
	}
	return nil
}

// CreateDir 新建分类（含 slug 自动派生）
func (a *App) CreateDir(ctx context.Context, input model.DirInput) (*model.Directory, error) {
	input.Name = strings.TrimSpace(input.Name)
	if !pkg.ValidDirName(input.Name) {
		return nil, pkg.BadRequest("分类名称非法")
	}
	if input.ParentID != 0 {
		if _, err := a.Repo.GetDir(ctx, input.ParentID); err != nil {
			return nil, pkg.BadRequest("父分类不存在")
		}
	}
	exists, err := a.Repo.DirNameExists(ctx, input.ParentID, input.Name, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, pkg.Conflict("同层级已存在同名分类")
	}

	// slug 处理
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = deriveDirSlug(input.Name)
	}
	if !pkg.ValidSlug(slug) {
		return nil, pkg.BadRequest("slug 非法（仅小写字母、数字、中划线）")
	}
	if err := checkSlugGlobally(ctx, a, slug, 0); err != nil {
		return nil, err
	}

	// 布尔字段默认值：未显式设置（值为 0）时默认已发布、分类可见
	sortVal := a.Repo.MaxDirSort(ctx, input.ParentID) + 1
	inp := model.DirectoryInput{
		ParentID:    input.ParentID,
		Name:        input.Name,
		Slug:        slug,
		PageTitle:   input.PageTitle,
		PageContent: input.PageContent,
		PageSummary: input.PageSummary,
		Cover:       input.Cover,
		IsPage:      input.IsPage,
		IsPublished: input.IsPublished,
		MenuVisible: input.MenuVisible,
		Sort:        sortVal,
	}
	if input.IsPublished == 0 {
		inp.IsPublished = 1
	}
	if input.MenuVisible == 0 && input.IsPage == 0 {
		inp.MenuVisible = 1
	}
	id, err := a.Repo.CreateDir(ctx, inp, pkg.NowStr())
	if err != nil {
		return nil, err
	}
	return a.Repo.GetDir(ctx, id)
}

// deriveDirSlug 从 name 派生 slug（中文自动转拼音）
func deriveDirSlug(name string) string {
	return pkg.Slugify(name, "untitled")
}

// reservedSlugs 根级 {slug}.html 禁止的保留字
var reservedSlugs = map[string]bool{
	"index": true, "archive": true, "feed": true, "sitemap": true, "robots": true,
	"tags": true, "tag": true, "article": true, "page": true, "static": true,
	"assets": true, "media": true,
}

// checkSlugGlobally 检查 slug 是否被占用（分类/文章/页面）或为保留字
func checkSlugGlobally(ctx context.Context, a *App, slug string, excludeID int64) error {
	if reservedSlugs[slug] {
		return pkg.BadRequest("slug 为系统保留字，不可使用")
	}
	if used, err := a.Repo.DirSlugExists(ctx, slug, excludeID); err != nil {
		return err
	} else if used {
		return pkg.Conflict("slug 已存在")
	}
	if used, err := a.Repo.PostSlugExists(ctx, slug, 0); err != nil {
		return err
	} else if used {
		return pkg.Conflict("slug 已被文章占用")
	}
	if used, err := a.Repo.PageSlugExists(ctx, slug, 0); err != nil {
		return err
	} else if used {
		return pkg.Conflict("slug 已被页面占用")
	}
	return nil
}

// UpdateDir 重命名 / 移动 / 排序 / 编辑分类
func (a *App) UpdateDir(ctx context.Context, id int64, input model.DirUpdateInput) (*model.Directory, error) {
	dir, err := a.Repo.GetDir(ctx, id)
	if err != nil {
		return nil, pkg.NotFound("分类不存在")
	}
	patch := model.DirectoryPatch{}

	// 名称校验
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if !pkg.ValidDirName(name) {
			return nil, pkg.BadRequest("分类名称非法")
		}
		patch.Name = &name
	}
	// slug 校验
	if input.Slug != nil {
		slug := strings.TrimSpace(*input.Slug)
		if slug == "" {
			return nil, pkg.BadRequest("slug 不能为空")
		}
		if !pkg.ValidSlug(slug) {
			return nil, pkg.BadRequest("slug 非法")
		}
		if err := checkSlugGlobally(ctx, a, slug, id); err != nil {
			return nil, err
		}
		patch.Slug = &slug
	}
	// 父分类校验
	if input.ParentID != nil {
		parentID := *input.ParentID
		if parentID != 0 {
			if _, err := a.Repo.GetDir(ctx, parentID); err != nil {
				return nil, pkg.BadRequest("目标父分类不存在")
			}
		}
		if parentID == id {
			return nil, pkg.BadRequest("分类不能作为自身的父级")
		}
		if parentID != 0 {
			dirs, err := a.Repo.ListDirs(ctx)
			if err != nil {
				return nil, err
			}
			parents := map[int64]int64{}
			for _, d := range dirs {
				parents[d.ID] = d.ParentID
			}
			cur := parentID
			for cur != 0 {
				if cur == id {
					return nil, pkg.BadRequest("不能将分类移动到其子分类下")
				}
				next, ok := parents[cur]
				if !ok {
					break
				}
				cur = next
			}
		}
		patch.ParentID = input.ParentID
	}
	if input.Sort != nil {
		s := *input.Sort
		if s < 0 {
			s = 0
		}
		patch.Sort = &s
	}
	// 同层级重名校验（若 name 或 parentID 有改）
	if patch.Name != nil || patch.ParentID != nil {
		targetParent := dir.ParentID
		targetName := dir.Name
		if patch.ParentID != nil {
			targetParent = *patch.ParentID
		}
		if patch.Name != nil {
			targetName = *patch.Name
		}
		if exists, err := a.Repo.DirNameExists(ctx, targetParent, targetName, id); err != nil {
			return nil, err
		} else if exists {
			return nil, pkg.Conflict("同层级已存在同名分类")
		}
	}
	// 其余字段直接透传
	patch.PageTitle = input.PageTitle
	patch.PageContent = input.PageContent
	patch.PageSummary = input.PageSummary
	patch.Cover = input.Cover
	patch.IsPage = input.IsPage
	patch.IsPublished = input.IsPublished
	patch.MenuVisible = input.MenuVisible

	if err := a.Repo.UpdateDir(ctx, id, patch, pkg.NowStr()); err != nil {
		return nil, err
	}
	return a.Repo.GetDir(ctx, id)
}

// DeleteDir 删除分类（mode: move / cascade）
func (a *App) DeleteDir(ctx context.Context, id int64, mode string) (*DeleteDirResult, error) {
	dir, err := a.Repo.GetDir(ctx, id)
	if err != nil {
		return nil, pkg.NotFound("分类不存在")
	}
	if mode != "move" && mode != "cascade" {
		return nil, pkg.BadRequest("mode 必须为 move 或 cascade")
	}
	dirs, err := a.Repo.ListDirs(ctx)
	if err != nil {
		return nil, err
	}
	result := &DeleteDirResult{}
	now := pkg.NowStr()
	if mode == "move" {
		for _, d := range dirs {
			if d.ParentID == id {
				result.MovedDirs++
			}
		}
		posts, _ := a.Repo.ListPostsByDirIDs(ctx, []int64{id})
		pages, _ := a.Repo.ListPagesByDirIDs(ctx, []int64{id})
		result.MovedPosts = len(posts)
		result.MovedPages = len(pages)
		err = a.Repo.WithTx(ctx, func(tx *repo.Repo) error {
			if err := tx.MoveDirChildren(ctx, id, dir.ParentID, now); err != nil {
				return err
			}
			return tx.DeleteDir(ctx, id)
		})
		if err != nil {
			return nil, err
		}
		// 移动后的条目需要更新索引里的 dir_id
		for _, p := range posts {
			p.DirID = dir.ParentID
			if err := a.indexPost(p); err != nil {
				slog.Warn("分类移动后文章索引更新失败", "post", p.ID, "err", err)
			}
		}
		for _, p := range pages {
			p.DirID = dir.ParentID
			if err := a.indexPage(p); err != nil {
				slog.Warn("分类移动后页面索引更新失败", "page", p.ID, "err", err)
			}
		}
		return result, nil
	}

	// cascade：收集全部后代分类
	descendants := map[int64]bool{id: true}
	changed := true
	for changed {
		changed = false
		for _, d := range dirs {
			if descendants[d.ParentID] && !descendants[d.ID] {
				descendants[d.ID] = true
				changed = true
			}
		}
	}
	ids := make([]int64, 0, len(descendants))
	for did := range descendants {
		ids = append(ids, did)
	}
	posts, err := a.Repo.ListPostsByDirIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	pages, err := a.Repo.ListPagesByDirIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	postIDs := make([]int64, 0, len(posts))
	for _, p := range posts {
		postIDs = append(postIDs, p.ID)
	}
	pageIDs := make([]int64, 0, len(pages))
	for _, p := range pages {
		pageIDs = append(pageIDs, p.ID)
	}
	err = a.Repo.WithTx(ctx, func(tx *repo.Repo) error {
		if err := tx.DeleteSnapshotsByTargets(ctx, "post", postIDs); err != nil {
			return err
		}
		if err := tx.DeleteSnapshotsByTargets(ctx, "page", pageIDs); err != nil {
			return err
		}
		if err := tx.DeletePostsByDirIDs(ctx, ids); err != nil {
			return err
		}
		if err := tx.DeletePagesByDirIDs(ctx, ids); err != nil {
			return err
		}
		for _, did := range ids {
			if err := tx.DeleteDir(ctx, did); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result.DeletedDirs = len(ids)
	result.DeletedPosts = len(posts)
	result.DeletedPages = len(pages)
	for _, p := range posts {
		if err := a.FTS.Delete("post", p.ID); err != nil {
			slog.Warn("分类级联删除后文章索引清理失败", "post", p.ID, "err", err)
		}
	}
	for _, p := range pages {
		if err := a.FTS.Delete("page", p.ID); err != nil {
			slog.Warn("分类级联删除后页面索引清理失败", "page", p.ID, "err", err)
		}
	}
	return result, nil
}

// MovePost 移动文章
func (a *App) MovePost(ctx context.Context, id, dirID int64) (*model.Post, error) {
	if dirID != 0 {
		if _, err := a.Repo.GetDir(ctx, dirID); err != nil {
			return nil, pkg.BadRequest("目标分类不存在")
		}
	}
	if _, err := a.Repo.GetPost(ctx, id); err != nil {
		return nil, pkg.NotFound("文章不存在")
	}
	if err := a.Repo.MovePost(ctx, id, dirID, pkg.NowStr()); err != nil {
		return nil, err
	}
	post, err := a.Repo.GetPost(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := a.indexPost(*post); err != nil {
		slog.Warn("文章移动后索引更新失败", "post", post.ID, "err", err)
	}
	return post, nil
}

// MovePage 移动页面
func (a *App) MovePage(ctx context.Context, id, dirID int64) (*model.Page, error) {
	if dirID != 0 {
		if _, err := a.Repo.GetDir(ctx, dirID); err != nil {
			return nil, pkg.BadRequest("目标分类不存在")
		}
	}
	if _, err := a.Repo.GetPage(ctx, id); err != nil {
		return nil, pkg.NotFound("页面不存在")
	}
	if err := a.Repo.MovePage(ctx, id, dirID, pkg.NowStr()); err != nil {
		return nil, err
	}
	page, err := a.Repo.GetPage(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := a.indexPage(*page); err != nil {
		slog.Warn("页面移动后索引更新失败", "page", page.ID, "err", err)
	}
	return page, nil
}

// SortTree 同层排序
func (a *App) SortTree(ctx context.Context, input model.TreeSortInput) error {
	if input.ParentID != 0 {
		if _, err := a.Repo.GetDir(ctx, input.ParentID); err != nil {
			return pkg.BadRequest("父分类不存在")
		}
	}
	for _, item := range input.Items {
		switch item.Type {
		case "dir":
			d, err := a.Repo.GetDir(ctx, item.ID)
			if err != nil {
				return pkg.NotFound("分类不存在")
			}
			if d.ParentID != input.ParentID {
				return pkg.BadRequest("节点父级不匹配，请先执行移动")
			}
		case "post":
			p, err := a.Repo.GetPost(ctx, item.ID)
			if err != nil {
				return pkg.NotFound("文章不存在")
			}
			if p.DirID != input.ParentID {
				return pkg.BadRequest("节点父级不匹配，请先执行移动")
			}
		case "page":
			p, err := a.Repo.GetPage(ctx, item.ID)
			if err != nil {
				return pkg.NotFound("页面不存在")
			}
			if p.DirID != input.ParentID {
				return pkg.BadRequest("节点父级不匹配，请先执行移动")
			}
		default:
			return pkg.BadRequest("非法节点类型")
		}
	}
	now := pkg.NowStr()
	return a.Repo.WithTx(ctx, func(tx *repo.Repo) error {
		for _, item := range input.Items {
			var err error
			switch item.Type {
			case "dir":
				err = tx.UpdateDirSort(ctx, item.ID, item.Sort, now)
			case "post":
				err = tx.UpdatePostSort(ctx, item.ID, item.Sort, now)
			case "page":
				err = tx.UpdatePageSort(ctx, item.ID, item.Sort, now)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
