package service

import (
	"context"
	"os"

	"marksite/internal/model"
)

// Stats 仪表盘统计
func (a *App) Stats(ctx context.Context) (*model.Stats, error) {
	out := &model.Stats{}
	postTotal, postPublished, err := a.Repo.CountPosts(ctx)
	if err != nil {
		return nil, err
	}
	pageTotal, pagePublished, err := a.Repo.CountPages(ctx)
	if err != nil {
		return nil, err
	}
	out.Posts = model.CountPair{Total: postTotal, Published: postPublished}
	out.Pages = model.CountPair{Total: pageTotal, Published: pagePublished}
	if out.Dirs, err = a.Repo.CountDirs(ctx); err != nil {
		return nil, err
	}
	if out.Tags, err = a.Repo.Tag().Count(ctx); err != nil {
		return nil, err
	}
	count, size, err := a.Repo.AssetTotals(ctx)
	if err != nil {
		return nil, err
	}
	out.Assets = model.AssetStats{Count: count, Size: size}
	out.FTS = model.FTSStats{Docs: a.FTS.Count(), Exists: a.FTS.Exists()}
	if list, err := a.ListBackups(); err == nil {
		out.Backups = model.BackupStats{Count: len(list)}
	}
	if st, err := os.Stat(a.Paths.DBPath); err == nil {
		out.DBSize = st.Size()
	}
	if last, err := a.Repo.LastSuccessfulTask(ctx); err == nil {
		out.LastGenerate = last
	}
	recent, err := a.Repo.ListTasks(ctx, 5)
	if err != nil {
		return nil, err
	}
	for i := range recent {
		recent[i].Log = ""
	}
	out.RecentTasks = recent
	return out, nil
}
