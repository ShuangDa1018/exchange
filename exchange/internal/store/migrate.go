package store

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"path"
	"sort"
	"strings"
)

// RunMigrations [DB:MIGRATE] 按文件名顺序执行 migrations/*.sql（只跑未执行过的）
// 版本记录在 schema_migrations
func (db *DB) RunMigrations(ctx context.Context, migrationsFS fs.FS) error {
	// [DB:WRITE] 迁移版本表
	if _, err := db.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    text        PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
)`); err != nil {
		return fmt.Errorf("创建 schema_migrations: %w", err)
	}

	var files []string
	err := fs.WalkDir(migrationsFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".sql") {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return fmt.Errorf("读取 migrations: %w", err)
	}
	sort.Strings(files)

	for _, rel := range files {
		version := path.Base(rel) // 用文件名当版本号，如 001_init.sql

		// [DB:READ] 是否已执行过该版本
		var exists bool
		if err := db.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version,
		).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}

		sqlBytes, err := fs.ReadFile(migrationsFS, rel)
		if err != nil {
			return fmt.Errorf("读 %s: %w", rel, err)
		}

		// [DB:WRITE] 事务内执行 SQL + 记录版本
		tx, err := db.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("执行 %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations(version) VALUES($1)`, version,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("记录 %s: %w", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		log.Printf("migration applied: %s", version)
	}
	return nil
}
