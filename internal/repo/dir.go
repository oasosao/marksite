package repo

import (
	"context"
	"database/sql"
	"errors"

	"marksite/internal/model"
)

type rowScanner interface {
	Scan(dest ...any) error
}

const dirCols = `id, parent_id, name, slug, page_title, page_content, page_summary, cover, is_page, is_published, menu_visible, sort, created_at, updated_at`

func scanDir(s rowScanner) (model.Directory, error) {
	var d model.Directory
	err := s.Scan(&d.ID, &d.ParentID, &d.Name, &d.Slug,
		&d.PageTitle, &d.PageContent, &d.PageSummary, &d.Cover,
		&d.IsPage, &d.IsPublished, &d.MenuVisible,
		&d.Sort, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (r *Repo) ListDirs(ctx context.Context) ([]model.Directory, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT `+dirCols+` FROM directories ORDER BY parent_id, sort, id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []model.Directory{}
	for rows.Next() {
		d, err := scanDir(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repo) GetDir(ctx context.Context, id int64) (*model.Directory, error) {
	d, err := scanDir(r.q.QueryRowContext(ctx, `SELECT `+dirCols+` FROM directories WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// CollectDescendantIDs 返回 rootID 及其所有子目录 ID（邻接表 BFS）
// 返回的切片保证包含 rootID 自身
func (r *Repo) CollectDescendantIDs(ctx context.Context, rootID int64) ([]int64, error) {
	all, err := r.ListDirs(ctx)
	if err != nil {
		return nil, err
	}
	children := map[int64][]int64{}
	for _, d := range all {
		children[d.ParentID] = append(children[d.ParentID], d.ID)
	}
	result := []int64{rootID}
	visited := map[int64]bool{rootID: true}
	queue := []int64{rootID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, cid := range children[cur] {
			if !visited[cid] {
				visited[cid] = true
				result = append(result, cid)
				queue = append(queue, cid)
			}
		}
	}
	return result, nil
}

// DirMeta 目录元信息（名称 + 完整路径）
type DirMeta struct {
	Name string
	Path string // 如 "技术博客 / Go 语言"，顶级目录只有自身名
}

// BatchDirMetaMap 批量构建目录 id → DirMeta 映射（单次 ListDirs + O(N) 建索引）
func (r *Repo) BatchDirMetaMap(ctx context.Context) (map[int64]DirMeta, error) {
	all, err := r.ListDirs(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[int64]model.Directory{}
	for _, d := range all {
		byID[d.ID] = d
	}
	out := map[int64]DirMeta{}
	var buildPath func(id int64) string
	buildPath = func(id int64) string {
		if meta, ok := out[id]; ok {
			return meta.Path // 已计算
		}
		d, ok := byID[id]
		if !ok {
			return ""
		}
		if d.ParentID == 0 {
			out[id] = DirMeta{Name: d.Name, Path: d.Name}
			return d.Name
		}
		parentPath := buildPath(d.ParentID)
		path := parentPath + " / " + d.Name
		out[id] = DirMeta{Name: d.Name, Path: path}
		return path
	}
	for _, d := range all {
		buildPath(d.ID)
	}
	return out, nil
}

// CreateDir 新建分类（含全部新字段）
func (r *Repo) CreateDir(ctx context.Context, d model.DirectoryInput, now string) (int64, error) {
	res, err := r.q.ExecContext(ctx,
		`INSERT INTO directories (parent_id, name, slug, page_title, page_content, page_summary, cover, is_page, is_published, menu_visible, sort, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ParentID, d.Name, d.Slug, d.PageTitle, d.PageContent, d.PageSummary, d.Cover,
		d.IsPage, d.IsPublished, d.MenuVisible, d.Sort, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateDir 更新分类（全部字段可选）
func (r *Repo) UpdateDir(ctx context.Context, id int64, patch model.DirectoryPatch, now string) error {
	sets := []string{}
	args := []any{}
	if patch.ParentID != nil {
		sets = append(sets, "parent_id = ?")
		args = append(args, *patch.ParentID)
	}
	if patch.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *patch.Name)
	}
	if patch.Slug != nil {
		sets = append(sets, "slug = ?")
		args = append(args, *patch.Slug)
	}
	if patch.PageTitle != nil {
		sets = append(sets, "page_title = ?")
		args = append(args, *patch.PageTitle)
	}
	if patch.PageContent != nil {
		sets = append(sets, "page_content = ?")
		args = append(args, *patch.PageContent)
	}
	if patch.PageSummary != nil {
		sets = append(sets, "page_summary = ?")
		args = append(args, *patch.PageSummary)
	}
	if patch.Cover != nil {
		sets = append(sets, "cover = ?")
		args = append(args, *patch.Cover)
	}
	if patch.IsPage != nil {
		sets = append(sets, "is_page = ?")
		args = append(args, *patch.IsPage)
	}
	if patch.IsPublished != nil {
		sets = append(sets, "is_published = ?")
		args = append(args, *patch.IsPublished)
	}
	if patch.MenuVisible != nil {
		sets = append(sets, "menu_visible = ?")
		args = append(args, *patch.MenuVisible)
	}
	if patch.Sort != nil {
		sets = append(sets, "sort = ?")
		args = append(args, *patch.Sort)
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, now, id)
	sqlStr := `UPDATE directories SET ` + joinComma(sets) + `, updated_at = ? WHERE id = ?`
	_, err := r.q.ExecContext(ctx, sqlStr, args...)
	return err
}

// DeleteDir 删除分类
func (r *Repo) DeleteDir(ctx context.Context, id int64) error {
	_, err := r.q.ExecContext(ctx, `DELETE FROM directories WHERE id = ?`, id)
	return err
}

// MoveDirChildren 将被删分类的直接子分类与内容上移到新父级
func (r *Repo) MoveDirChildren(ctx context.Context, from, to int64, now string) error {
	if _, err := r.q.ExecContext(ctx, `UPDATE directories SET parent_id = ?, updated_at = ? WHERE parent_id = ?`, to, now, from); err != nil {
		return err
	}
	if _, err := r.q.ExecContext(ctx, `UPDATE posts SET dir_id = ?, updated_at = ? WHERE dir_id = ?`, to, now, from); err != nil {
		return err
	}
	_, err := r.q.ExecContext(ctx, `UPDATE pages SET dir_id = ?, updated_at = ? WHERE dir_id = ?`, to, now, from)
	return err
}

func (r *Repo) MaxDirSort(ctx context.Context, parentID int64) int {
	var v sql.NullInt64
	if err := r.q.QueryRowContext(ctx, `SELECT MAX(sort) FROM directories WHERE parent_id = ?`, parentID).Scan(&v); err != nil {
		return 0
	}
	if !v.Valid {
		return -1
	}
	return int(v.Int64)
}

func (r *Repo) DirNameExists(ctx context.Context, parentID int64, name string, excludeID int64) (bool, error) {
	var count int
	err := r.q.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM directories WHERE parent_id = ? AND name = ? AND id <> ?`,
		parentID, name, excludeID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// DirSlugExists 分类 slug 是否已存在
func (r *Repo) DirSlugExists(ctx context.Context, slug string, excludeID int64) (bool, error) {
	var count int
	err := r.q.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM directories WHERE slug = ? AND id <> ?`,
		slug, excludeID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repo) CountDirs(ctx context.Context) (int64, error) {
	var count int64
	err := r.q.QueryRowContext(ctx, `SELECT COUNT(1) FROM directories`).Scan(&count)
	return count, err
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ","
		}
		out += v
	}
	return out
}
