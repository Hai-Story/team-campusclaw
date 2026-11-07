package retrieval

import (
	"context"
	"database/sql"
	"log"
	"sync"

	"campusclaw/backend/internal/chunking"
	"campusclaw/backend/internal/config"
)

// Indexer 把材料正文切分为切片、写入 MySQL，并逐切片嵌入后写入 Qdrant。
// 单切片嵌入失败不中断同批其余切片；失败切片标 failed 且 Qdrant 无残留（spec 嵌入失败不丢原文）。
type Indexer struct {
	materialLocks sync.Map
	DB            *sql.DB
	Cfg           *config.Config
	Embed         *EmbeddingClient
	Qdr           *QdrantClient
}

// IndexMaterial 按策略对材料重建索引：先删旧切片与旧向量，再重新切分、嵌入、写入（幂等）。
// 上传场景调用时 strategy 为 auto（custom/hierarchy 经 reindex 接口进入）。
// WithMaterial serializes source mutation and every indexing entry point for a material.
func (ix *Indexer) WithMaterial(ctx context.Context, id int64, fn func() error) error {
	candidate := make(chan struct{}, 1)
	candidate <- struct{}{}
	actual, _ := ix.materialLocks.LoadOrStore(id, candidate)
	gate := actual.(chan struct{})
	select {
	case <-gate:
		defer func() { gate <- struct{}{} }()
		return fn()
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (ix *Indexer) IndexMaterial(ctx context.Context, materialID, entryID, classID int64, bodyText, strategy string, opts chunking.Options) (int, error) {
	n := 0
	err := ix.WithMaterial(ctx, materialID, func() error {
		var err error
		n, err = ix.IndexCurrentLocked(ctx, materialID, strategy, opts)
		return err
	})
	return n, err
}

// IndexCurrentLocked is called only while holding WithMaterial; caller arguments cannot replay stale source text.
func (ix *Indexer) IndexCurrentLocked(ctx context.Context, materialID int64, strategy string, opts chunking.Options) (int, error) {
	var entryID, classID int64
	var bodyText string
	if err := ix.DB.QueryRowContext(ctx, `SELECT e.id,m.class_id,e.body_text FROM materials m JOIN knowledge_entries e ON e.material_id=m.id WHERE m.id=? ORDER BY e.id LIMIT 1`, materialID).Scan(&entryID, &classID, &bodyText); err != nil {
		return 0, err
	}
	return ix.indexText(ctx, materialID, entryID, classID, bodyText, strategy, opts)
}
func (ix *Indexer) indexText(ctx context.Context, materialID, entryID, classID int64, bodyText, strategy string, opts chunking.Options) (int, error) {
	chunks, err := chunking.Split(bodyText, strategy, opts, ix.Cfg.ChunkAutoMaxChars, ix.Cfg.ChunkAutoOverlap)
	if err != nil {
		return 0, err // 参数越界：调用方返回 400，既有切片与向量保持不变
	}

	// 删旧：MySQL 切片行 + Qdrant 向量点（按 material_id 过滤）。Qdrant 不可用时继续删 MySQL，
	// 后续 upsert 仍会先 EnsureCollection；旧向量残留风险在 Search 回表核对时兜底。
	if _, err := ix.DB.ExecContext(ctx, `DELETE FROM knowledge_chunks WHERE material_id = ?`, materialID); err != nil {
		return 0, err
	}
	if err := ix.Qdr.DeleteByMaterial(ctx, materialID); err != nil {
		log.Printf("删除旧向量失败（material %d）：%v", materialID, err)
	}

	// 先插 MySQL 切片行拿到 id（向量主键与之一致），再逐切片嵌入写入 Qdrant。
	for _, c := range chunks {
		res, err := ix.DB.ExecContext(ctx, `
			INSERT INTO knowledge_chunks (knowledge_entry_id, material_id, class_id, chunk_index, chunk_text, char_start, char_end, index_status, strategy)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?)`,
			entryID, materialID, classID, c.Index, c.Text, c.Start, c.End, strategy)
		if err != nil {
			return 0, err
		}
		chunkID, err := res.LastInsertId()
		if err != nil {
			return 0, err
		}
		status := "ready"
		vec, err := ix.Embed.Embed(ctx, c.Text)
		if err != nil {
			log.Printf("切片 %d 嵌入失败：%v", chunkID, err)
			status = "failed"
		} else if err := ix.Qdr.UpsertPoint(ctx, chunkID, classID, materialID, entryID, c.Index, vec); err != nil {
			log.Printf("切片 %d 写入 Qdrant 失败：%v", chunkID, err)
			status = "failed"
		}
		if _, err := ix.DB.ExecContext(ctx, `UPDATE knowledge_chunks SET index_status = ? WHERE id = ?`, status, chunkID); err != nil {
			log.Printf("更新切片状态失败 %d：%v", chunkID, err)
		}
	}
	return len(chunks), nil
}

// BackfillMissing 启动时为尚无切片的既有材料（含种子材料）按 auto 策略补齐索引。
// 嵌入失败标 failed，绝不阻断系统启动（spec 种子材料补齐索引）。
func (ix *Indexer) BackfillMissing(ctx context.Context) {
	rows, err := ix.DB.QueryContext(ctx, `
		SELECT m.id, e.id, e.class_id, e.body_text
		FROM materials m
		JOIN knowledge_entries e ON e.material_id = m.id
		WHERE NOT EXISTS (SELECT 1 FROM knowledge_chunks kc WHERE kc.material_id = m.id)`)
	if err != nil {
		log.Printf("补齐索引查询失败：%v", err)
		return
	}
	type item struct {
		materialID, entryID, classID int64
		body                         string
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.materialID, &it.entryID, &it.classID, &it.body); err != nil {
			log.Printf("补齐索引扫描失败：%v", err)
			rows.Close()
			return
		}
		items = append(items, it)
	}
	rows.Close()
	for _, it := range items {
		n, err := ix.IndexMaterial(ctx, it.materialID, it.entryID, it.classID, it.body, chunking.StrategyAuto, chunking.Options{})
		if err != nil {
			log.Printf("补齐索引失败（material %d）：%v", it.materialID, err)
			continue
		}
		log.Printf("已为材料 %d 补齐 %d 个切片", it.materialID, n)
	}
}
